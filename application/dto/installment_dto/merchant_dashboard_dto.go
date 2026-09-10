package installmentdto

import "time"

// MerchantInstallmentDashboardResponse résume l'état global des tranches du marchand
type MerchantInstallmentDashboardResponse struct {
	TotalPendingOrders int   `json:"total_pending_orders"` // Commandes en cours de paiement
	TotalAmountPending int64 `json:"total_amount_pending"` // Montant total encore dû par les clients (en FCFA)
	TotalHeldAmount    int64 `json:"total_held_amount"`    // Montant actuellement bloqué en séquestre (en FCFA)
	TotalOverdueOrders int   `json:"total_overdue_orders"` // Commandes avec au moins une tranche en retard
}

// InstallmentOrderSummary résume une commande spécifique pour le dashboard
type InstallmentOrderSummary struct {
	OrderID             string               `json:"order_id"`
	CustomerName        string               `json:"customer_name"`
	TotalAmount         int64                `json:"total_amount"`
	PaidAmount          int64                `json:"paid_amount"`
	RemainingAmount     int64                `json:"remaining_amount"`
	Status              string               `json:"status"` // "pending", "partial", "complete", "overdue"
	NextDueDate         *time.Time           `json:"next_due_date,omitempty"`
	DeliveryZoneName    string               `json:"delivery_zone_name"`
	ReleaseDelayDays    int                  `json:"release_delay_days"`
	ExpectedReleaseDate *time.Time           `json:"expected_release_date,omitempty"`
	Installments        []*InstallmentDetail `json:"installments"`
}

// InstallmentDetail représente une tranche individuelle
type InstallmentDetail struct {
	TrancheNumber int        `json:"tranche_number"`
	AmountCents   int64      `json:"amount_cents"`
	DueDate       time.Time  `json:"due_date"`
	Status        string     `json:"status"` // "pending", "paid", "overdue"
	PaidAt        *time.Time `json:"paid_at,omitempty"`
}
