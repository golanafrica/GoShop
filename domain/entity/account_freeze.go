package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ACCOUNT FREEZE (Historique des gels de compte)
// ============================================================

// AccountFreeze représente un gel de compte marchand
// Créé automatiquement quand le wallet devient négatif
// ou quand une commission COD ne peut pas être prélevée
type AccountFreeze struct {
	ID     string `json:"id" db:"id"`
	ShopID string `json:"shop_id" db:"shop_id"`

	// Raison du gel
	FreezeReason  FreezeReason `json:"freeze_reason" db:"freeze_reason"`
	FreezeDetails *string      `json:"freeze_details,omitempty" db:"freeze_details"`

	// Montant dû au système
	AmountDueCents int64 `json:"amount_due_cents" db:"amount_due_cents"`

	// Timeline
	FrozenAt          time.Time `json:"frozen_at" db:"frozen_at"`
	GracePeriodDays   int       `json:"grace_period_days" db:"grace_period_days"`
	GracePeriodEndsAt time.Time `json:"grace_period_ends_at" db:"grace_period_ends_at"`

	// Résolution
	ResolvedAt *time.Time        `json:"resolved_at,omitempty" db:"resolved_at"`
	Resolution *FreezeResolution `json:"resolution,omitempty" db:"resolution"`
	ResolvedBy *string           `json:"resolved_by,omitempty" db:"resolved_by"`

	// Rappels envoyés
	Reminder1SentAt *time.Time `json:"reminder_1_sent_at,omitempty" db:"reminder_1_sent_at"`
	Reminder2SentAt *time.Time `json:"reminder_2_sent_at,omitempty" db:"reminder_2_sent_at"`
	Reminder3SentAt *time.Time `json:"reminder_3_sent_at,omitempty" db:"reminder_3_sent_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ============================================================
// CONSTRUCTEUR
// ============================================================

// NewAccountFreeze crée un nouveau gel de compte
func NewAccountFreeze(
	shopID string,
	reason FreezeReason,
	amountDueCents int64,
	details *string,
) (*AccountFreeze, error) {
	if shopID == "" {
		return nil, errors.New("shop_id is required")
	}
	if !reason.IsValid() {
		return nil, fmt.Errorf("invalid freeze reason: %s", reason)
	}
	if amountDueCents <= 0 {
		return nil, errors.New("amount_due_cents must be positive")
	}

	now := time.Now().UTC()
	gracePeriodEnds := now.AddDate(0, 0, DefaultGracePeriodDays)

	return &AccountFreeze{
		ShopID:            shopID,
		FreezeReason:      reason,
		FreezeDetails:     details,
		AmountDueCents:    amountDueCents,
		FrozenAt:          now,
		GracePeriodDays:   DefaultGracePeriodDays,
		GracePeriodEndsAt: gracePeriodEnds,
		CreatedAt:         now,
	}, nil
}

// ============================================================
// MÉTHODES : RAPPELS
// ============================================================

// MarkReminder1Sent marque le rappel J+1 comme envoyé
func (f *AccountFreeze) MarkReminder1Sent() error {
	if f.ResolvedAt != nil {
		return errors.New("cannot send reminder for resolved freeze")
	}
	if f.Reminder1SentAt != nil {
		return errors.New("reminder 1 already sent")
	}

	now := time.Now().UTC()
	f.Reminder1SentAt = &now
	return nil
}

// MarkReminder2Sent marque le rappel J+3 comme envoyé
func (f *AccountFreeze) MarkReminder2Sent() error {
	if f.ResolvedAt != nil {
		return errors.New("cannot send reminder for resolved freeze")
	}
	if f.Reminder2SentAt != nil {
		return errors.New("reminder 2 already sent")
	}
	if f.Reminder1SentAt == nil {
		return errors.New("reminder 1 must be sent first")
	}

	now := time.Now().UTC()
	f.Reminder2SentAt = &now
	return nil
}

// MarkReminder3Sent marque le rappel J+6 comme envoyé
func (f *AccountFreeze) MarkReminder3Sent() error {
	if f.ResolvedAt != nil {
		return errors.New("cannot send reminder for resolved freeze")
	}
	if f.Reminder3SentAt != nil {
		return errors.New("reminder 3 already sent")
	}
	if f.Reminder2SentAt == nil {
		return errors.New("reminder 2 must be sent first")
	}

	now := time.Now().UTC()
	f.Reminder3SentAt = &now
	return nil
}

// ShouldSendReminder1 vérifie si le rappel J+1 doit être envoyé
func (f *AccountFreeze) ShouldSendReminder1() bool {
	if f.IsResolved() {
		return false
	}
	if f.Reminder1SentAt != nil {
		return false
	}
	elapsed := time.Since(f.FrozenAt)
	return elapsed.Hours()/24 >= Reminder1Days
}

// ShouldSendReminder2 vérifie si le rappel J+3 doit être envoyé
func (f *AccountFreeze) ShouldSendReminder2() bool {
	if f.IsResolved() {
		return false
	}
	if f.Reminder2SentAt != nil {
		return false
	}
	if f.Reminder1SentAt == nil {
		return false
	}
	elapsed := time.Since(f.FrozenAt)
	return elapsed.Hours()/24 >= Reminder2Days
}

// ShouldSendReminder3 vérifie si le rappel J+6 doit être envoyé
func (f *AccountFreeze) ShouldSendReminder3() bool {
	if f.IsResolved() {
		return false
	}
	if f.Reminder3SentAt != nil {
		return false
	}
	if f.Reminder2SentAt == nil {
		return false
	}
	elapsed := time.Since(f.FrozenAt)
	return elapsed.Hours()/24 >= Reminder3Days
}

// ============================================================
// MÉTHODES : RÉSOLUTION
// ============================================================

// ResolvePaid marque le gel comme résolu (marchand a payé)
func (f *AccountFreeze) ResolvePaid(resolvedBy string) error {
	if f.IsResolved() {
		return errors.New("freeze already resolved")
	}
	if resolvedBy == "" {
		return errors.New("resolved_by is required")
	}

	now := time.Now().UTC()
	resolution := FreezeResolutionPaid

	f.ResolvedAt = &now
	f.Resolution = &resolution
	f.ResolvedBy = &resolvedBy
	return nil
}

// ResolveSuspended marque le gel comme résolu (suspension définitive)
func (f *AccountFreeze) ResolveSuspended(resolvedBy string) error {
	if f.IsResolved() {
		return errors.New("freeze already resolved")
	}
	if resolvedBy == "" {
		return errors.New("resolved_by is required")
	}

	now := time.Now().UTC()
	resolution := FreezeResolutionSuspended

	f.ResolvedAt = &now
	f.Resolution = &resolution
	f.ResolvedBy = &resolvedBy
	return nil
}

// ResolveWaived marque le gel comme résolu (dette annulée)
func (f *AccountFreeze) ResolveWaived(resolvedBy string) error {
	if f.IsResolved() {
		return errors.New("freeze already resolved")
	}
	if resolvedBy == "" {
		return errors.New("resolved_by is required")
	}

	now := time.Now().UTC()
	resolution := FreezeResolutionWaived

	f.ResolvedAt = &now
	f.Resolution = &resolution
	f.ResolvedBy = &resolvedBy
	return nil
}

// ResolveEscalated marque le gel comme escaladé (vers juridique)
func (f *AccountFreeze) ResolveEscalated(resolvedBy string) error {
	if f.IsResolved() {
		return errors.New("freeze already resolved")
	}
	if resolvedBy == "" {
		return errors.New("resolved_by is required")
	}

	now := time.Now().UTC()
	resolution := FreezeResolutionEscalated

	f.ResolvedAt = &now
	f.Resolution = &resolution
	f.ResolvedBy = &resolvedBy
	return nil
}

// ============================================================
// MÉTHODES DE REQUÊTE
// ============================================================

// IsResolved vérifie si le gel est résolu
func (f *AccountFreeze) IsResolved() bool {
	return f.ResolvedAt != nil && f.Resolution != nil
}

// IsExpired vérifie si la période de grâce est expirée
func (f *AccountFreeze) IsExpired() bool {
	if f.IsResolved() {
		return false
	}
	return time.Now().UTC().After(f.GracePeriodEndsAt)
}

// DaysUntilExpiration retourne le nombre de jours restants avant expiration
func (f *AccountFreeze) DaysUntilExpiration() int {
	if f.IsResolved() {
		return 0
	}
	if f.IsExpired() {
		return 0
	}
	remaining := time.Until(f.GracePeriodEndsAt)
	return int(remaining.Hours() / 24)
}

// DaysSinceFrozen retourne le nombre de jours depuis le gel
func (f *AccountFreeze) DaysSinceFrozen() int {
	return int(time.Since(f.FrozenAt).Hours() / 24)
}

// IsPaid vérifie si résolu par paiement
func (f *AccountFreeze) IsPaid() bool {
	return f.Resolution != nil && *f.Resolution == FreezeResolutionPaid
}

// IsSuspended vérifie si résolu par suspension
func (f *AccountFreeze) IsSuspended() bool {
	return f.Resolution != nil && *f.Resolution == FreezeResolutionSuspended
}

// IsWaived vérifie si résolu par annulation
func (f *AccountFreeze) IsWaived() bool {
	return f.Resolution != nil && *f.Resolution == FreezeResolutionWaived
}

// IsEscalated vérifie si escaladé
func (f *AccountFreeze) IsEscalated() bool {
	return f.Resolution != nil && *f.Resolution == FreezeResolutionEscalated
}

// NeedsImmediateAction vérifie si une action immédiate est requise
func (f *AccountFreeze) NeedsImmediateAction() bool {
	if f.IsResolved() {
		return false
	}
	// Action requise si J+6 ou plus
	return f.DaysSinceFrozen() >= Reminder3Days
}

// ShouldSuspend vérifie si le compte doit être suspendu définitivement
func (f *AccountFreeze) ShouldSuspend() bool {
	if f.IsResolved() {
		return false
	}
	// Suspension si période de grâce expirée
	return f.IsExpired()
}

// GetResolutionString retourne la résolution sous forme de string
func (f *AccountFreeze) GetResolutionString() string {
	if f.Resolution == nil {
		return "unresolved"
	}
	return string(*f.Resolution)
}

// ============================================================
// VALIDATION
// ============================================================

// Validate valide le gel de compte
func (f *AccountFreeze) Validate() error {
	if f.ShopID == "" {
		return errors.New("shop_id is required")
	}
	if !f.FreezeReason.IsValid() {
		return fmt.Errorf("invalid freeze reason: %s", f.FreezeReason)
	}
	if f.AmountDueCents <= 0 {
		return errors.New("amount_due_cents must be positive")
	}
	if f.GracePeriodDays <= 0 {
		return errors.New("grace_period_days must be positive")
	}
	if f.GracePeriodEndsAt.Before(f.FrozenAt) {
		return errors.New("grace_period_ends_at must be after frozen_at")
	}

	// Si résolu, vérifier les champs requis
	if f.ResolvedAt != nil {
		if f.Resolution == nil {
			return errors.New("resolution is required when resolved_at is set")
		}
		if !f.Resolution.IsValid() {
			return fmt.Errorf("invalid resolution: %s", *f.Resolution)
		}
		if f.ResolvedBy == nil || *f.ResolvedBy == "" {
			return errors.New("resolved_by is required when resolved_at is set")
		}
	}

	return nil
}

// ============================================================
// HELPERS : CRÉATION RAPIDE
// ============================================================

// NewNegativeBalanceFreeze crée un gel pour wallet négatif
func NewNegativeBalanceFreeze(shopID string, amountDueCents int64) (*AccountFreeze, error) {
	details := fmt.Sprintf("Wallet balance is negative: -%d FCFA", amountDueCents/100)
	return NewAccountFreeze(shopID, FreezeReasonNegativeBalance, amountDueCents, &details)
}

// NewUnpaidCommissionFreeze crée un gel pour commission COD impayée
func NewUnpaidCommissionFreeze(shopID string, amountDueCents int64, orderID string) (*AccountFreeze, error) {
	details := fmt.Sprintf("Commission for order %s could not be collected", orderID)
	return NewAccountFreeze(shopID, FreezeReasonUnpaidCommission, amountDueCents, &details)
}

// NewFraudFreeze crée un gel pour fraude suspectée
func NewFraudFreeze(shopID string, amountDueCents int64, details string) (*AccountFreeze, error) {
	return NewAccountFreeze(shopID, FreezeReasonFraudSuspected, amountDueCents, &details)
}

// NewAdminFreeze crée un gel par décision admin
func NewAdminFreeze(shopID string, amountDueCents int64, details string) (*AccountFreeze, error) {
	return NewAccountFreeze(shopID, FreezeReasonAdminDecision, amountDueCents, &details)
}
