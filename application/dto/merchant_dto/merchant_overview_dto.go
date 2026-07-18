package merchantdto

import "time"

// MerchantOverviewResponse représente la vue d'ensemble d'un marchand
type MerchantOverviewResponse struct {
	// Ventes
	TotalSalesCents  int64 `json:"total_sales_cents"`
	TotalOrdersCount int   `json:"total_orders_count"`
	OnlineSalesCents int64 `json:"online_sales_cents"`
	CashSalesCents   int64 `json:"cash_sales_cents"`

	// Crédit
	ActiveContractsCount  int   `json:"active_contracts_count"`
	TotalFinancedCents    int64 `json:"total_financed_cents"`
	TotalOutstandingCents int64 `json:"total_outstanding_cents"` // Montant encore dû

	// Recouvrement
	RecoveryRatePercent float64 `json:"recovery_rate_percent"`
	OverdueAmountCents  int64   `json:"overdue_amount_cents"`
	OverdueCount        int     `json:"overdue_count"`

	// Wallet
	WalletBalanceCents int64      `json:"wallet_balance_cents"`
	IsFrozen           bool       `json:"is_frozen"`
	FreezeReason       *string    `json:"freeze_reason,omitempty"`
	AmountDueCents     int64      `json:"amount_due_cents,omitempty"`
	GracePeriodEndsAt  *time.Time `json:"grace_period_ends_at,omitempty"`

	// Commissions
	MonthlyCommissionCents int64 `json:"monthly_commission_cents"`

	// Timestamp
	GeneratedAt time.Time `json:"generated_at"`
}
