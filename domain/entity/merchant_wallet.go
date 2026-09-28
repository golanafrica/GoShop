package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ENUMS WALLET
// ============================================================

type WalletTransactionType string

const (
	// Crédits
	WalletTxSaleCredit      WalletTransactionType = "sale_credit"
	WalletTxCOD             WalletTransactionType = "sale_cod"
	WalletTxSaleTontine     WalletTransactionType = "sale_tontine"
	WalletTxSaleCreditPlan  WalletTransactionType = "sale_credit_plan"
	WalletTxDeposit         WalletTransactionType = "deposit"
	WalletTxUnfreezeDeposit WalletTransactionType = "unfreeze_deposit"

	// Débits
	WalletTxCommissionDebit WalletTransactionType = "commission_debit"
	WalletTxPayout          WalletTransactionType = "payout"
	WalletTxRefund          WalletTransactionType = "refund"
	WalletTxClawback        WalletTransactionType = "clawback"
	WalletTxFreezePenalty   WalletTransactionType = "freeze_penalty"
	WalletTxDebtAdd         WalletTransactionType = "debt_add"   // hausse dette (audit)
	WalletTxDebtSweep       WalletTransactionType = "debt_sweep" // prélèvement sur crédit
)

func (t WalletTransactionType) IsValid() bool {
	switch t {
	case WalletTxSaleCredit, WalletTxCOD, WalletTxSaleTontine,
		WalletTxSaleCreditPlan, WalletTxDeposit, WalletTxUnfreezeDeposit,
		WalletTxCommissionDebit, WalletTxPayout, WalletTxRefund,
		WalletTxClawback, WalletTxFreezePenalty,
		WalletTxDebtAdd, WalletTxDebtSweep:
		return true
	}
	return false
}

func (t WalletTransactionType) IsCredit() bool {
	switch t {
	case WalletTxSaleCredit, WalletTxCOD, WalletTxSaleTontine,
		WalletTxSaleCreditPlan, WalletTxDeposit, WalletTxUnfreezeDeposit:
		return true
	}
	return false
}

func (t WalletTransactionType) IsDebit() bool {
	return !t.IsCredit()
}

type WalletTransactionStatus string

const (
	WalletTxPending   WalletTransactionStatus = "pending"
	WalletTxCompleted WalletTransactionStatus = "completed"
	WalletTxFailed    WalletTransactionStatus = "failed"
	WalletTxCancelled WalletTransactionStatus = "cancelled"
)

func (s WalletTransactionStatus) IsValid() bool {
	switch s {
	case WalletTxPending, WalletTxCompleted, WalletTxFailed, WalletTxCancelled:
		return true
	}
	return false
}

type FreezeReason string

const (
	FreezeReasonNegativeBalance  FreezeReason = "negative_balance"
	FreezeReasonUnpaidCommission FreezeReason = "unpaid_commission"
	FreezeReasonFraudSuspected   FreezeReason = "fraud_suspected"
	FreezeReasonAdminDecision    FreezeReason = "admin_decision"
)

func (r FreezeReason) IsValid() bool {
	switch r {
	case FreezeReasonNegativeBalance, FreezeReasonUnpaidCommission,
		FreezeReasonFraudSuspected, FreezeReasonAdminDecision:
		return true
	}
	return false
}

type FreezeResolution string

const (
	FreezeResolutionPaid      FreezeResolution = "paid"
	FreezeResolutionSuspended FreezeResolution = "suspended"
	FreezeResolutionWaived    FreezeResolution = "waived"
	FreezeResolutionEscalated FreezeResolution = "escalated"
)

func (r FreezeResolution) IsValid() bool {
	switch r {
	case FreezeResolutionPaid, FreezeResolutionSuspended,
		FreezeResolutionWaived, FreezeResolutionEscalated:
		return true
	}
	return false
}

const (
	DefaultMaxNegativeBalanceCents = -5_000_000
	DefaultGracePeriodDays         = 7
	Reminder1Days                  = 1
	Reminder2Days                  = 3
	Reminder3Days                  = 6
)

// ============================================================
// MERCHANT WALLET
// ============================================================
//
// balance_cents = ledger disponible (peut être 0 après clawback)
// held_cents    = gelé (tontine, etc.)
// debt_cents    = dû plateforme (clawback partiel) — toujours >= 0
// available     = max(0, balance - held - debt)

type MerchantWallet struct {
	ShopID string `json:"shop_id" db:"shop_id"`

	BalanceCents int64 `json:"balance_cents" db:"balance_cents"`
	HeldCents    int64 `json:"held_cents" db:"held_cents"`
	DebtCents    int64 `json:"debt_cents" db:"debt_cents"`

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

func NewMerchantWallet(shopID string) *MerchantWallet {
	now := time.Now().UTC()
	return &MerchantWallet{
		ShopID:                  shopID,
		BalanceCents:            0,
		HeldCents:               0,
		DebtCents:               0,
		MaxNegativeBalanceCents: DefaultMaxNegativeBalanceCents,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

// ============================================================
// AVAILABLE / HELD / DEBT
// ============================================================

func (w *MerchantWallet) AvailableCents() int64 {
	if w == nil {
		return 0
	}
	avail := w.BalanceCents - w.HeldCents - w.DebtCents
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
// TRANSACTIONS
// ============================================================

// Credit crédite sans sweep (legacy). Préférer CreditWithDebtSweep pour les ventes.
func (w *MerchantWallet) Credit(amountCents int64) error {
	if amountCents <= 0 {
		return errors.New("amount must be positive")
	}
	w.BalanceCents += amountCents
	w.TotalSalesCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// CreditWithDebtSweep crédite puis prélève sur debt_cents.
// Retourne (net ajouté à balance, montant sweepé sur dette).
func (w *MerchantWallet) CreditWithDebtSweep(creditCents int64) (netToBalance int64, swept int64, err error) {
	if w == nil {
		return 0, 0, errors.New("wallet is nil")
	}
	if creditCents <= 0 {
		return 0, 0, errors.New("credit must be positive")
	}
	// Autorisé même frozen : on réduit la dette / on récupère
	swept = creditCents
	if swept > w.DebtCents {
		swept = w.DebtCents
	}
	netToBalance = creditCents - swept
	w.BalanceCents += netToBalance
	w.DebtCents -= swept
	if w.DebtCents < 0 {
		w.DebtCents = 0
	}
	w.TotalSalesCents += creditCents
	w.UpdatedAt = time.Now().UTC()
	return netToBalance, swept, nil
}

// AddDebt augmente debt_cents (sans toucher balance).
func (w *MerchantWallet) AddDebt(amountCents int64) error {
	if w == nil {
		return errors.New("wallet is nil")
	}
	if amountCents <= 0 {
		return errors.New("debt amount must be positive")
	}
	w.DebtCents += amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

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

// ApplyClawback : legacy — débite balance uniquement (peut aller négatif via MaxNegative).
// Préférer ApplyClawbackToDebt pour le modèle dette explicite.
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

// ApplyClawbackToDebt : prend sur balance (sans descendre sous 0), reste → debt_cents.
// Ne gèle pas. Ne rend pas balance négative.
func (w *MerchantWallet) ApplyClawbackToDebt(amountCents int64) (fromBalance int64, toDebt int64, err error) {
	if w == nil {
		return 0, 0, errors.New("wallet is nil")
	}
	if amountCents <= 0 {
		return 0, 0, errors.New("clawback amount must be positive")
	}
	available := w.BalanceCents
	if available < 0 {
		available = 0
	}
	fromBalance = amountCents
	if fromBalance > available {
		fromBalance = available
	}
	toDebt = amountCents - fromBalance
	w.BalanceCents -= fromBalance
	if toDebt > 0 {
		w.DebtCents += toDebt
	}
	w.UpdatedAt = time.Now().UTC()
	return fromBalance, toDebt, nil
}

func (w *MerchantWallet) DebtAfterCredit(creditCents int64) int64 {
	if creditCents < 0 {
		creditCents = 0
	}
	// Après un crédit avec sweep : dette restante
	debt := w.DebtCents - creditCents
	if debt < 0 {
		return 0
	}
	return debt
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
	if w.DebtCents > 0 {
		return fmt.Errorf("cannot withdraw while debt outstanding: debt_cents=%d", w.DebtCents)
	}
	if amountCents > w.AvailableCents() {
		return fmt.Errorf("insufficient available balance: requested=%d available=%d (held=%d debt=%d)",
			amountCents, w.AvailableCents(), w.HeldCents, w.DebtCents)
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
	// Couvrir dette explicite ou balance négative legacy
	need := w.DebtCents
	if w.BalanceCents < 0 {
		need += -w.BalanceCents
	}
	if need > 0 && depositAmountCents < need {
		return fmt.Errorf("deposit must cover debt: %d < %d", depositAmountCents, need)
	}
	if depositAmountCents > 0 {
		net, _, err := w.CreditWithDebtSweep(depositAmountCents)
		if err != nil {
			return err
		}
		_ = net
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

// GetDebt : dette explicite + éventuel solde négatif legacy.
func (w *MerchantWallet) GetDebt() int64 {
	d := w.DebtCents
	if w.BalanceCents < 0 {
		d += -w.BalanceCents
	}
	return d
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
	if w.DebtCents < 0 {
		return errors.New("debt_cents cannot be negative")
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
