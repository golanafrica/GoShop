package reportingdto

import "time"

// PlatformRevenueRequest représente les paramètres de requête pour les rapports de revenus plateforme
type PlatformRevenueRequest struct {
	StartDate time.Time `json:"start_date" form:"start_date"`
	EndDate   time.Time `json:"end_date" form:"end_date"`
	Limit     int       `json:"limit" form:"limit"`
	Offset    int       `json:"offset" form:"offset"`
}

// PlatformRevenueBalanceResponse représente le solde actuel des revenus de la plateforme
type PlatformRevenueBalanceResponse struct {
	BalanceCents        int64     `json:"balance_cents"`
	TotalCollectedCents int64     `json:"total_collected_cents"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// PlatformRevenueTransactionItem représente une transaction de revenu plateforme
type PlatformRevenueTransactionItem struct {
	ID              string    `json:"id"`
	TransactionType string    `json:"transaction_type"`
	AmountCents     int64     `json:"amount_cents"`
	ReferenceType   string    `json:"reference_type"`
	ReferenceID     string    `json:"reference_id"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

// PlatformRevenueListResponse représente la réponse paginée des transactions de revenus
type PlatformRevenueListResponse struct {
	Transactions []PlatformRevenueTransactionItem `json:"transactions"`
	Total        int                              `json:"total"`
	Limit        int                              `json:"limit"`
	Offset       int                              `json:"offset"`
}

// MerchantStatementRequest représente les paramètres pour un relevé marchand
type MerchantStatementRequest struct {
	ShopID    string    `json:"shop_id" form:"shop_id"`
	StartDate time.Time `json:"start_date" form:"start_date"`
	EndDate   time.Time `json:"end_date" form:"end_date"`
	Limit     int       `json:"limit" form:"limit"`
	Offset    int       `json:"offset" form:"offset"`
}

// WalletTransactionItem représente une transaction de portefeuille marchand
type WalletTransactionItem struct {
	ID                string    `json:"id"`
	ShopID            string    `json:"shop_id"`
	TransactionType   string    `json:"transaction_type"`
	AmountCents       int64     `json:"amount_cents"`
	BalanceAfterCents int64     `json:"balance_after_cents"`
	ReferenceType     *string   `json:"reference_type,omitempty"`
	ReferenceID       *string   `json:"reference_id,omitempty"`
	Description       *string   `json:"description,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

// MerchantStatementResponse représente le relevé complet d'un marchand
type MerchantStatementResponse struct {
	ShopID           string                  `json:"shop_id"`
	PeriodStart      time.Time               `json:"period_start"`
	PeriodEnd        time.Time               `json:"period_end"`
	OpeningBalance   int64                   `json:"opening_balance_cents"`
	ClosingBalance   int64                   `json:"closing_balance_cents"`
	TotalCredits     int64                   `json:"total_credits_cents"`
	TotalDebits      int64                   `json:"total_debits_cents"`
	Transactions     []WalletTransactionItem `json:"transactions"`
	TransactionCount int                     `json:"transaction_count"`
}

// ExportFormat définit les formats d'export supportés
type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)
