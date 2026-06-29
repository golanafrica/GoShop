package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ENUMS WALLET
// ============================================================

// WalletTransactionType représente le type de transaction wallet
type WalletTransactionType string

const (
	// Crédits (argent qui rentre)
	WalletTxSaleCredit      WalletTransactionType = "sale_credit"      // Vente Mobile Money
	WalletTxCOD             WalletTransactionType = "sale_cod"         // Vente COD (après commission)
	WalletTxSaleTontine     WalletTransactionType = "sale_tontine"     // Vente tontine
	WalletTxSaleCreditPlan  WalletTransactionType = "sale_credit_plan" // Vente crédit tempérament
	WalletTxDeposit         WalletTransactionType = "deposit"          // Dépôt manuel
	WalletTxUnfreezeDeposit WalletTransactionType = "unfreeze_deposit" // Dépôt pour dégeler

	// Débits (argent qui sort)
	WalletTxCommissionDebit WalletTransactionType = "commission_debit" // Commission GoShop
	WalletTxPayout          WalletTransactionType = "payout"           // Virement vers banque
	WalletTxRefund          WalletTransactionType = "refund"           // Remboursement client
	WalletTxFreezePenalty   WalletTransactionType = "freeze_penalty"   // Pénalité gel
)

// IsValid vérifie si le type de transaction est valide
func (t WalletTransactionType) IsValid() bool {
	switch t {
	case WalletTxSaleCredit, WalletTxCOD, WalletTxSaleTontine,
		WalletTxSaleCreditPlan, WalletTxDeposit, WalletTxUnfreezeDeposit,
		WalletTxCommissionDebit, WalletTxPayout, WalletTxRefund,
		WalletTxFreezePenalty:
		return true
	}
	return false
}

// IsCredit vérifie si c'est un crédit (argent qui rentre)
func (t WalletTransactionType) IsCredit() bool {
	switch t {
	case WalletTxSaleCredit, WalletTxCOD, WalletTxSaleTontine,
		WalletTxSaleCreditPlan, WalletTxDeposit, WalletTxUnfreezeDeposit:
		return true
	}
	return false
}

// IsDebit vérifie si c'est un débit (argent qui sort)
func (t WalletTransactionType) IsDebit() bool {
	return !t.IsCredit()
}

// WalletTransactionStatus représente le statut d'une transaction
type WalletTransactionStatus string

const (
	WalletTxPending   WalletTransactionStatus = "pending"
	WalletTxCompleted WalletTransactionStatus = "completed"
	WalletTxFailed    WalletTransactionStatus = "failed"
	WalletTxCancelled WalletTransactionStatus = "cancelled"
)

// IsValid vérifie si le statut est valide
func (s WalletTransactionStatus) IsValid() bool {
	switch s {
	case WalletTxPending, WalletTxCompleted, WalletTxFailed, WalletTxCancelled:
		return true
	}
	return false
}

// FreezeReason représente la raison d'un gel de compte
type FreezeReason string

const (
	FreezeReasonNegativeBalance  FreezeReason = "negative_balance"  // Wallet passé en négatif
	FreezeReasonUnpaidCommission FreezeReason = "unpaid_commission" // Commission COD impayée
	FreezeReasonFraudSuspected   FreezeReason = "fraud_suspected"   // Fraude suspectée
	FreezeReasonAdminDecision    FreezeReason = "admin_decision"    // Décision admin
)

// IsValid vérifie si la raison est valide
func (r FreezeReason) IsValid() bool {
	switch r {
	case FreezeReasonNegativeBalance, FreezeReasonUnpaidCommission,
		FreezeReasonFraudSuspected, FreezeReasonAdminDecision:
		return true
	}
	return false
}

// FreezeResolution représente la résolution d'un gel
type FreezeResolution string

const (
	FreezeResolutionPaid      FreezeResolution = "paid"      // Marchand a payé
	FreezeResolutionSuspended FreezeResolution = "suspended" // Suspension définitive
	FreezeResolutionWaived    FreezeResolution = "waived"    // Dette annulée
	FreezeResolutionEscalated FreezeResolution = "escalated" // Escaladé
)

// IsValid vérifie si la résolution est valide
func (r FreezeResolution) IsValid() bool {
	switch r {
	case FreezeResolutionPaid, FreezeResolutionSuspended,
		FreezeResolutionWaived, FreezeResolutionEscalated:
		return true
	}
	return false
}

// ============================================================
// CONSTANTES WALLET
// ============================================================

const (
	// Limites
	DefaultMaxNegativeBalanceCents = -100000 // -1000 FCFA max

	// Délais gel
	DefaultGracePeriodDays = 7 // 7 jours pour payer avant suspension

	// Seuils de rappel
	Reminder1Days = 1 // J+1
	Reminder2Days = 3 // J+3
	Reminder3Days = 6 // J+6
)

// ============================================================
// MERCHANT WALLET (Portefeuille marchand)
// ============================================================

// MerchantWallet représente le portefeuille virtuel d'un marchand
type MerchantWallet struct {
	ShopID string `json:"shop_id" db:"shop_id"`

	// Solde (peut être NÉGATIF = dette)
	BalanceCents int64 `json:"balance_cents" db:"balance_cents"`

	// Gel du compte
	IsFrozen     bool       `json:"is_frozen" db:"is_frozen"`
	FrozenAt     *time.Time `json:"frozen_at,omitempty" db:"frozen_at"`
	FrozenReason *string    `json:"frozen_reason,omitempty" db:"frozen_reason"`
	FrozenUntil  *time.Time `json:"frozen_until,omitempty" db:"frozen_until"`

	// Limites
	MaxNegativeBalanceCents int64 `json:"max_negative_balance_cents" db:"max_negative_balance_cents"`

	// Stats
	TotalSalesCents       int64 `json:"total_sales_cents" db:"total_sales_cents"`
	TotalCommissionsCents int64 `json:"total_commissions_cents" db:"total_commissions_cents"`
	TotalPayoutsCents     int64 `json:"total_payouts_cents" db:"total_payouts_cents"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewMerchantWallet crée un nouveau wallet pour un shop
func NewMerchantWallet(shopID string) *MerchantWallet {
	now := time.Now().UTC()
	return &MerchantWallet{
		ShopID:                  shopID,
		BalanceCents:            0,
		MaxNegativeBalanceCents: DefaultMaxNegativeBalanceCents,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

// ============================================================
// MÉTHODES DE TRANSACTION
// ============================================================

// Credit crédite le wallet (argent qui rentre)
func (w *MerchantWallet) Credit(amountCents int64) error {
	if w.IsFrozen {
		return errors.New("wallet is frozen, cannot credit")
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}

	w.BalanceCents += amountCents
	w.TotalSalesCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Debit débite le wallet (argent qui sort)
func (w *MerchantWallet) Debit(amountCents int64) error {
	if w.IsFrozen {
		return errors.New("wallet is frozen, cannot debit")
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}

	newBalance := w.BalanceCents - amountCents

	// Vérifier qu'on ne dépasse pas la limite négative
	if newBalance < w.MaxNegativeBalanceCents {
		return fmt.Errorf("debit would exceed max negative balance: %d < %d",
			newBalance, w.MaxNegativeBalanceCents)
	}

	w.BalanceCents = newBalance
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// DebitCommission prélève la commission GoShop
// Retourne une erreur si le wallet ne peut pas couvrir la commission
func (w *MerchantWallet) DebitCommission(commissionCents int64) (bool, error) {
	if commissionCents <= 0 {
		return false, errors.New("commission must be positive")
	}

	newBalance := w.BalanceCents - commissionCents

	// Si le wallet peut couvrir la commission
	if newBalance >= w.MaxNegativeBalanceCents {
		w.BalanceCents = newBalance
		w.TotalCommissionsCents += commissionCents
		w.UpdatedAt = time.Now().UTC()
		return true, nil // Commission prélevée
	}

	// Sinon → GEL du compte
	return false, nil // Commission NON prélevée, doit geler
}

// RequestPayout demande un virement vers le compte bancaire
func (w *MerchantWallet) RequestPayout(amountCents int64) error {
	if w.IsFrozen {
		return errors.New("wallet is frozen, cannot request payout")
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	if amountCents > w.BalanceCents {
		return fmt.Errorf("insufficient balance: %d > %d", amountCents, w.BalanceCents)
	}

	w.BalanceCents -= amountCents
	w.TotalPayoutsCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// ============================================================
// MÉTHODES DE GEL / DÉGEL
// ============================================================

// Freeze gèle le compte
func (w *MerchantWallet) Freeze(reason FreezeReason, details string) error {
	if w.IsFrozen {
		return errors.New("wallet is already frozen")
	}
	if !reason.IsValid() {
		return fmt.Errorf("invalid freeze reason: %s", reason)
	}

	now := time.Now().UTC()
	gracePeriodEnds := now.AddDate(0, 0, DefaultGracePeriodDays)
	reasonStr := string(reason)

	w.IsFrozen = true
	w.FrozenAt = &now
	w.FrozenReason = &reasonStr
	w.FrozenUntil = &gracePeriodEnds
	w.UpdatedAt = now
	_ = details // Pour logging éventuel
	return nil
}

// Unfreeze dégèle le compte (après paiement de la dette)
func (w *MerchantWallet) Unfreeze(depositAmountCents int64) error {
	if !w.IsFrozen {
		return errors.New("wallet is not frozen")
	}

	// Vérifier que le dépôt couvre la dette
	if w.BalanceCents < 0 && depositAmountCents < -w.BalanceCents {
		return fmt.Errorf("deposit must cover debt: %d < %d",
			depositAmountCents, -w.BalanceCents)
	}

	// Créditer le dépôt
	if depositAmountCents > 0 {
		w.BalanceCents += depositAmountCents
	}

	w.IsFrozen = false
	w.FrozenAt = nil
	w.FrozenReason = nil
	w.FrozenUntil = nil
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// ForceUnfreeze dégèle le compte sans dépôt (admin)
func (w *MerchantWallet) ForceUnfreeze() error {
	if !w.IsFrozen {
		return errors.New("wallet is not frozen")
	}

	w.IsFrozen = false
	w.FrozenAt = nil
	w.FrozenReason = nil
	w.FrozenUntil = nil
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// ============================================================
// MÉTHODES DE REQUÊTE
// ============================================================

// IsPositive vérifie si le solde est positif
func (w *MerchantWallet) IsPositive() bool {
	return w.BalanceCents > 0
}

// IsZero vérifie si le solde est zéro
func (w *MerchantWallet) IsZero() bool {
	return w.BalanceCents == 0
}

// IsNegative vérifie si le solde est négatif
func (w *MerchantWallet) IsNegative() bool {
	return w.BalanceCents < 0
}

// GetDebt retourne le montant de la dette (0 si pas de dette)
func (w *MerchantWallet) GetDebt() int64 {
	if w.BalanceCents >= 0 {
		return 0
	}
	return -w.BalanceCents
}

// CanCoverCommission vérifie si le wallet peut couvrir une commission
func (w *MerchantWallet) CanCoverCommission(commissionCents int64) bool {
	return (w.BalanceCents - commissionCents) >= w.MaxNegativeBalanceCents
}

// GracePeriodExpired vérifie si la période de grâce est expirée
func (w *MerchantWallet) GracePeriodExpired() bool {
	if !w.IsFrozen || w.FrozenUntil == nil {
		return false
	}
	return time.Now().UTC().After(*w.FrozenUntil)
}

// DaysUntilSuspension retourne le nombre de jours restants avant suspension
func (w *MerchantWallet) DaysUntilSuspension() int {
	if !w.IsFrozen || w.FrozenUntil == nil {
		return 0
	}
	remaining := time.Until(*w.FrozenUntil)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

// ShouldSendReminder1 vérifie si le rappel J+1 doit être envoyé
func (w *MerchantWallet) ShouldSendReminder1() bool {
	if !w.IsFrozen || w.FrozenAt == nil {
		return false
	}
	elapsed := time.Since(*w.FrozenAt)
	return elapsed.Hours()/24 >= Reminder1Days && elapsed.Hours()/24 < Reminder1Days+1
}

// ShouldSendReminder2 vérifie si le rappel J+3 doit être envoyé
func (w *MerchantWallet) ShouldSendReminder2() bool {
	if !w.IsFrozen || w.FrozenAt == nil {
		return false
	}
	elapsed := time.Since(*w.FrozenAt)
	return elapsed.Hours()/24 >= Reminder2Days && elapsed.Hours()/24 < Reminder2Days+1
}

// ShouldSendReminder3 vérifie si le rappel J+6 doit être envoyé
func (w *MerchantWallet) ShouldSendReminder3() bool {
	if !w.IsFrozen || w.FrozenAt == nil {
		return false
	}
	elapsed := time.Since(*w.FrozenAt)
	return elapsed.Hours()/24 >= Reminder3Days && elapsed.Hours()/24 < Reminder3Days+1
}

// ============================================================
// VALIDATION
// ============================================================

// Validate valide le wallet
func (w *MerchantWallet) Validate() error {
	if w.ShopID == "" {
		return errors.New("shop_id is required")
	}
	if w.MaxNegativeBalanceCents > 0 {
		return errors.New("max_negative_balance_cents must be negative or zero")
	}
	if w.TotalSalesCents < 0 {
		return errors.New("total_sales_cents cannot be negative")
	}
	if w.TotalCommissionsCents < 0 {
		return errors.New("total_commissions_cents cannot be negative")
	}
	if w.TotalPayoutsCents < 0 {
		return errors.New("total_payouts_cents cannot be negative")
	}
	return nil
}

// ============================================================
// WALLET TRANSACTION (Historique des transactions)
// ============================================================

// WalletTransaction représente une transaction dans le wallet
type WalletTransaction struct {
	ID     string `json:"id" db:"id"`
	ShopID string `json:"shop_id" db:"shop_id"`

	// Type et montant
	TransactionType   WalletTransactionType `json:"transaction_type" db:"transaction_type"`
	AmountCents       int64                 `json:"amount_cents" db:"amount_cents"`
	BalanceAfterCents int64                 `json:"balance_after_cents" db:"balance_after_cents"`

	// Référence (order, credit, tontine, etc.)
	ReferenceType *string `json:"reference_type,omitempty" db:"reference_type"`
	ReferenceID   *string `json:"reference_id,omitempty" db:"reference_id"`

	// Description
	Description *string `json:"description,omitempty" db:"description"`

	// Statut
	Status WalletTransactionStatus `json:"status" db:"status"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewWalletTransaction crée une nouvelle transaction
func NewWalletTransaction(
	shopID string,
	transactionType WalletTransactionType,
	amountCents int64,
	balanceAfterCents int64,
	referenceType, referenceID, description *string,
) (*WalletTransaction, error) {
	if shopID == "" {
		return nil, errors.New("shop_id is required")
	}
	if !transactionType.IsValid() {
		return nil, fmt.Errorf("invalid transaction type: %s", transactionType)
	}
	if amountCents == 0 {
		return nil, errors.New("amount cannot be zero")
	}

	// Vérifier cohérence signe montant / type
	if transactionType.IsCredit() && amountCents < 0 {
		return nil, errors.New("credit transaction must have positive amount")
	}
	if transactionType.IsDebit() && amountCents > 0 {
		return nil, errors.New("debit transaction must have negative amount")
	}

	return &WalletTransaction{
		ShopID:            shopID,
		TransactionType:   transactionType,
		AmountCents:       amountCents,
		BalanceAfterCents: balanceAfterCents,
		ReferenceType:     referenceType,
		ReferenceID:       referenceID,
		Description:       description,
		Status:            WalletTxCompleted,
		CreatedAt:         time.Now().UTC(),
	}, nil
}

// IsCredit vérifie si c'est un crédit
func (t *WalletTransaction) IsCredit() bool {
	return t.TransactionType.IsCredit()
}

// IsDebit vérifie si c'est un débit
func (t *WalletTransaction) IsDebit() bool {
	return t.TransactionType.IsDebit()
}

// GetAbsoluteAmount retourne le montant absolu
func (t *WalletTransaction) GetAbsoluteAmount() int64 {
	if t.AmountCents < 0 {
		return -t.AmountCents
	}
	return t.AmountCents
}
