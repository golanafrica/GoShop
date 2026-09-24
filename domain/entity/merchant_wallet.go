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
	WalletTxPayout          WalletTransactionType = "payout"           // Virement / cash-out
	WalletTxRefund          WalletTransactionType = "refund"           // Remboursement client (ledger)
	WalletTxClawback        WalletTransactionType = "clawback"         // Reprise post-release (litige)
	WalletTxFreezePenalty   WalletTransactionType = "freeze_penalty"   // Pénalité gel
)

// IsValid vérifie si le type de transaction est valide
func (t WalletTransactionType) IsValid() bool {
	switch t {
	case WalletTxSaleCredit, WalletTxCOD, WalletTxSaleTontine,
		WalletTxSaleCreditPlan, WalletTxDeposit, WalletTxUnfreezeDeposit,
		WalletTxCommissionDebit, WalletTxPayout, WalletTxRefund,
		WalletTxClawback, WalletTxFreezePenalty:
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
	// -50 000 XOF max de dette ledger (refund ~975 XOF = 97_500 cents)
	DefaultMaxNegativeBalanceCents = -5_000_000

	DefaultGracePeriodDays = 7

	Reminder1Days = 1
	Reminder2Days = 3
	Reminder3Days = 6
)

// ============================================================
// MERCHANT WALLET (Portefeuille marchand)
// ============================================================

// MerchantWallet représente le portefeuille virtuel d'un marchand
//
// balance_cents = solde ledger total (peut être négatif = dette)
// held_cents    = montant gelé (tontine, etc.) — migration 045
// available     = max(0, balance_cents - held_cents)
type MerchantWallet struct {
	ShopID string `json:"shop_id" db:"shop_id"`

	BalanceCents int64 `json:"balance_cents" db:"balance_cents"`
	HeldCents    int64 `json:"held_cents" db:"held_cents"`

	IsFrozen     bool       `json:"is_frozen" db:"is_frozen"`
	FrozenAt     *time.Time `json:"frozen_at,omitempty" db:"frozen_at"`
	FrozenReason *string    `json:"frozen_reason,omitempty" db:"frozen_reason"`
	FrozenUntil  *time.Time `json:"frozen_until,omitempty" db:"frozen_until"`

	MaxNegativeBalanceCents int64 `json:"max_negative_balance_cents" db:"max_negative_balance_cents"`

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
		HeldCents:               0,
		MaxNegativeBalanceCents: DefaultMaxNegativeBalanceCents,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

// ============================================================
// AVAILABLE / HELD
// ============================================================

func (w *MerchantWallet) AvailableCents() int64 {
	avail := w.BalanceCents - w.HeldCents
	if avail < 0 {
		return 0
	}
	return avail
}

func (w *MerchantWallet) Hold(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New("hold amount must be positive")
	}
	if w.AvailableCents() < amountCents {
		return fmt.Errorf("insufficient available balance to hold: available=%d requested=%d",
			w.AvailableCents(), amountCents)
	}
	w.HeldCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *MerchantWallet) ReleaseHeld(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New("release amount must be positive")
	}
	if amountCents > w.HeldCents {
		return fmt.Errorf("cannot release more than held: held=%d requested=%d",
			w.HeldCents, amountCents)
	}
	w.HeldCents -= amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// CreditAndHold : tontine — crédit + hold. Bloqué si frozen (pas de recovery hold).
func (w *MerchantWallet) CreditAndHold(amountCents int64) error {
	if w.IsFrozen {
		return errors.New("wallet is frozen, cannot credit")
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	w.BalanceCents += amountCents
	w.HeldCents += amountCents
	w.TotalSalesCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// ============================================================
// MÉTHODES DE TRANSACTION
// ============================================================

// Credit crédite le wallet. Autorisé même si frozen : réduit une dette (recovery).
func (w *MerchantWallet) Credit(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	w.BalanceCents += amountCents
	w.TotalSalesCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Debit débite le wallet — bloqué si frozen.
func (w *MerchantWallet) Debit(amountCents int64) error {
	if w.IsFrozen {
		return errors.New("wallet is frozen, cannot debit")
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	newBalance := w.BalanceCents - amountCents
	if newBalance < w.MaxNegativeBalanceCents {
		return fmt.Errorf("debit would exceed max negative balance: %d < %d",
			newBalance, w.MaxNegativeBalanceCents)
	}
	w.BalanceCents = newBalance
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// ApplyClawback reprend un montant post-release (litige). Ne gèle PAS. Peut aller négatif.
func (w *MerchantWallet) ApplyClawback(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New("clawback amount must be positive")
	}
	newBalance := w.BalanceCents - amountCents
	if newBalance < w.MaxNegativeBalanceCents {
		return fmt.Errorf(
			"clawback would exceed max negative balance: new=%d min=%d",
			newBalance, w.MaxNegativeBalanceCents,
		)
	}
	w.BalanceCents = newBalance
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// DebtAfterCredit : dette restante après un crédit hypothétique (notifs).
func (w *MerchantWallet) DebtAfterCredit(creditCents int64) int64 {
	if creditCents < 0 {
		creditCents = 0
	}
	after := w.BalanceCents + creditCents
	if after >= 0 {
		return 0
	}
	return -after
}

func (w *MerchantWallet) DebitCommission(commissionCents int64) (bool, error) {
	if commissionCents <= 0 {
		return false, errors.New("commission must be positive")
	}
	newBalance := w.BalanceCents - commissionCents
	if newBalance >= w.MaxNegativeBalanceCents {
		w.BalanceCents = newBalance
		w.TotalCommissionsCents += commissionCents
		w.UpdatedAt = time.Now().UTC()
		return true, nil
	}
	return false, nil
}

func (w *MerchantWallet) RequestPayout(amountCents int64) error {
	if w.IsFrozen {
		return errors.New("wallet is frozen, cannot request payout")
	}
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	if amountCents > w.AvailableCents() {
		return fmt.Errorf("insufficient available balance: requested=%d available=%d (held=%d)",
			amountCents, w.AvailableCents(), w.HeldCents)
	}
	w.BalanceCents -= amountCents
	w.TotalPayoutsCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// ============================================================
// GEL / DÉGEL
// ============================================================

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
	_ = details
	return nil
}

func (w *MerchantWallet) Unfreeze(depositAmountCents int64) error {
	if !w.IsFrozen {
		return errors.New("wallet is not frozen")
	}
	if w.BalanceCents < 0 && depositAmountCents < -w.BalanceCents {
		return fmt.Errorf("deposit must cover debt: %d < %d",
			depositAmountCents, -w.BalanceCents)
	}
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
// REQUÊTES
// ============================================================

func (w *MerchantWallet) IsPositive() bool { return w.BalanceCents > 0 }
func (w *MerchantWallet) IsZero() bool     { return w.BalanceCents == 0 }
func (w *MerchantWallet) IsNegative() bool { return w.BalanceCents < 0 }

func (w *MerchantWallet) GetDebt() int64 {
	if w.BalanceCents >= 0 {
		return 0
	}
	return -w.BalanceCents
}

func (w *MerchantWallet) CanCoverCommission(commissionCents int64) bool {
	return (w.BalanceCents - commissionCents) >= w.MaxNegativeBalanceCents
}

func (w *MerchantWallet) GracePeriodExpired() bool {
	if !w.IsFrozen || w.FrozenUntil == nil {
		return false
	}
	return time.Now().UTC().After(*w.FrozenUntil)
}

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

func (w *MerchantWallet) ShouldSendReminder1() bool {
	if !w.IsFrozen || w.FrozenAt == nil {
		return false
	}
	elapsed := time.Since(*w.FrozenAt)
	return elapsed.Hours()/24 >= Reminder1Days && elapsed.Hours()/24 < Reminder1Days+1
}

func (w *MerchantWallet) ShouldSendReminder2() bool {
	if !w.IsFrozen || w.FrozenAt == nil {
		return false
	}
	elapsed := time.Since(*w.FrozenAt)
	return elapsed.Hours()/24 >= Reminder2Days && elapsed.Hours()/24 < Reminder2Days+1
}

func (w *MerchantWallet) ShouldSendReminder3() bool {
	if !w.IsFrozen || w.FrozenAt == nil {
		return false
	}
	elapsed := time.Since(*w.FrozenAt)
	return elapsed.Hours()/24 >= Reminder3Days && elapsed.Hours()/24 < Reminder3Days+1
}

func (w *MerchantWallet) Validate() error {
	if w.ShopID == "" {
		return errors.New("shop_id is required")
	}
	if w.HeldCents < 0 {
		return errors.New("held_cents cannot be negative")
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
// WALLET TRANSACTION
// ============================================================

type WalletTransaction struct {
	ID                string                  `json:"id" db:"id"`
	ShopID            string                  `json:"shop_id" db:"shop_id"`
	TransactionType   WalletTransactionType   `json:"transaction_type" db:"transaction_type"`
	AmountCents       int64                   `json:"amount_cents" db:"amount_cents"`
	BalanceAfterCents int64                   `json:"balance_after_cents" db:"balance_after_cents"`
	ReferenceType     *string                 `json:"reference_type,omitempty" db:"reference_type"`
	ReferenceID       *string                 `json:"reference_id,omitempty" db:"reference_id"`
	Description       *string                 `json:"description,omitempty" db:"description"`
	Status            WalletTransactionStatus `json:"status" db:"status"`
	CreatedAt         time.Time               `json:"created_at" db:"created_at"`
}

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

func (t *WalletTransaction) IsCredit() bool {
	return t.TransactionType.IsCredit()
}

func (t *WalletTransaction) IsDebit() bool {
	return t.TransactionType.IsDebit()
}

func (t *WalletTransaction) GetAbsoluteAmount() int64 {
	if t.AmountCents < 0 {
		return -t.AmountCents
	}
	return t.AmountCents
}
