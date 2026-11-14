package reconciliation_dto

// ShopReconciliationResponse représente l'état de cohérence financière d'un shop
type ShopReconciliationResponse struct {
	ShopID        string   `json:"shop_id"`
	WalletBalance int64    `json:"wallet_balance_cents"`
	LedgerBalance int64    `json:"ledger_balance_cents"`
	BalanceDiff   int64    `json:"balance_diff_cents"`
	WalletHeld    int64    `json:"wallet_held_cents"`
	Status        string   `json:"status"`        // "OK", "WARNING", "CRITICAL"
	Discrepancies []string `json:"discrepancies"` // Liste des anomalies détectées
	LastCheckedAt string   `json:"last_checked_at"`
}

// GlobalReconciliationResponse agrège les résultats pour l'admin
type GlobalReconciliationResponse struct {
	TotalShopsChecked int                          `json:"total_shops_checked"`
	ShopsOK           int                          `json:"shops_ok"`
	ShopsWarning      int                          `json:"shops_warning"`
	ShopsCritical     int                          `json:"shops_critical"`
	ShopsFailed       int                          `json:"shops_failed"`  // 🆕 PR-D FIX : Comptabilise les shops dont la vérification a échoué
	HasMore           bool                         `json:"has_more"`      // 🆕 PR-D FIX : Indique si d'autres pages de shops existent
	FlaggedShops      []ShopReconciliationResponse `json:"flagged_shops"` // Uniquement ceux avec des écarts
}
