package creditdto

import "time"

// ClientDashboardResponse représente la vue d'ensemble financière du client
type ClientDashboardResponse struct {
	CustomerID           string                   `json:"customer_id"`
	FirstName            string                   `json:"first_name"`
	LastName             string                   `json:"last_name"`
	KYCLevel             string                   `json:"kyc_level"`
	CreditScore          CreditScoreDTO           `json:"credit_score"`
	ActiveContracts      []ActiveContractDTO      `json:"active_contracts"`
	UpcomingInstallments []UpcomingInstallmentDTO `json:"upcoming_installments"`
}

type CreditScoreDTO struct {
	Score          int `json:"score"`
	OnTimePayments int `json:"on_time_payments"`
	LatePayments   int `json:"late_payments"`
	Defaults       int `json:"defaults"`
}

type ActiveContractDTO struct {
	ContractID          string `json:"contract_id"`
	Status              string `json:"status"`
	TotalAmountCents    int64  `json:"total_amount_cents"`
	MonthlyPaymentCents int64  `json:"monthly_payment_cents"`
}

type UpcomingInstallmentDTO struct {
	InstallmentID string    `json:"installment_id"`
	ContractID    string    `json:"contract_id"`
	DueDate       time.Time `json:"due_date"`
	AmountCents   int64     `json:"amount_cents"`
	Status        string    `json:"status"`
}
