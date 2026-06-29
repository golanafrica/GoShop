package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ENUMS CRÉDIT
// ============================================================

// CreditApplicationStatus représente le statut d'une demande de crédit
type CreditApplicationStatus string

const (
	CreditApplicationPending   CreditApplicationStatus = "pending"
	CreditApplicationApproved  CreditApplicationStatus = "approved"
	CreditApplicationRejected  CreditApplicationStatus = "rejected"
	CreditApplicationCancelled CreditApplicationStatus = "cancelled"
)

// IsValid vérifie si le statut est valide
func (s CreditApplicationStatus) IsValid() bool {
	switch s {
	case CreditApplicationPending, CreditApplicationApproved,
		CreditApplicationRejected, CreditApplicationCancelled:
		return true
	}
	return false
}

// CreditContractStatus représente le statut d'un contrat de crédit
type CreditContractStatus string

const (
	CreditContractActive    CreditContractStatus = "active"
	CreditContractCompleted CreditContractStatus = "completed"
	CreditContractDefaulted CreditContractStatus = "defaulted"
	CreditContractCancelled CreditContractStatus = "cancelled"
)

// IsValid vérifie si le statut est valide
func (s CreditContractStatus) IsValid() bool {
	switch s {
	case CreditContractActive, CreditContractCompleted,
		CreditContractDefaulted, CreditContractCancelled:
		return true
	}
	return false
}

// InstallmentStatus représente le statut d'une échéance
type InstallmentStatus string

const (
	InstallmentPending   InstallmentStatus = "pending"
	InstallmentPaid      InstallmentStatus = "paid"
	InstallmentLate      InstallmentStatus = "late"
	InstallmentDefaulted InstallmentStatus = "defaulted"
)

// IsValid vérifie si le statut est valide
func (s InstallmentStatus) IsValid() bool {
	switch s {
	case InstallmentPending, InstallmentPaid,
		InstallmentLate, InstallmentDefaulted:
		return true
	}
	return false
}

// ============================================================
// CONSTANTES CRÉDIT
// ============================================================

const (
	// Limites configuration crédit
	MinDownPaymentPercent = 0
	MaxDownPaymentPercent = 50
	MinDurationMonths     = 1
	MaxDurationMonths     = 36
	MinInterestRateBps    = 0
	MaxInterestRateBps    = 1500 // 15% max
	MinPenaltyRateBps     = 0
	MaxPenaltyRateBps     = 2000 // 20% max
	MinCreditScore        = 0
	MaxCreditScore        = 1000

	// Seuils de score
	ScoreExcellent = 700 // Crédit auto-approuvé
	ScoreGood      = 500 // Validation marchand requise
	ScoreMedium    = 300 // Apport initial 30%
	ScoreBad       = 0   // Crédit refusé

	// Pénalités
	LateThresholdDays    = 7  // Après 7 jours → statut "late"
	DefaultThresholdDays = 90 // Après 90 jours → statut "defaulted"
)

// ============================================================
// CREDIT PLAN (Configuration par produit)
// ============================================================

// CreditPlan représente la configuration du crédit pour un produit
type CreditPlan struct {
	ID        string `json:"id" db:"id"`
	ProductID string `json:"product_id" db:"product_id"`
	ShopID    string `json:"shop_id" db:"shop_id"`

	// Activation
	IsEnabled bool `json:"is_enabled" db:"is_enabled"`

	// Configuration
	MinDownPaymentPercent int `json:"min_down_payment_percent" db:"min_down_payment_percent"`
	MaxDurationMonths     int `json:"max_duration_months" db:"max_duration_months"`
	InterestRateBps       int `json:"interest_rate_bps" db:"interest_rate_bps"`
	PenaltyRateBps        int `json:"penalty_rate_bps" db:"penalty_rate_bps"`

	// Score minimum requis
	MinCreditScore int `json:"min_credit_score" db:"min_credit_score"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewCreditPlan crée un nouveau plan de crédit avec valeurs par défaut
func NewCreditPlan(productID, shopID string) *CreditPlan {
	return &CreditPlan{
		ProductID:             productID,
		ShopID:                shopID,
		IsEnabled:             false,
		MinDownPaymentPercent: 20,
		MaxDurationMonths:     12,
		InterestRateBps:       0,
		PenaltyRateBps:        500,
		MinCreditScore:        300,
	}
}

// Validate valide le plan de crédit
func (p *CreditPlan) Validate() error {
	if p.ProductID == "" {
		return errors.New("product_id is required")
	}
	if p.ShopID == "" {
		return errors.New("shop_id is required")
	}
	if p.MinDownPaymentPercent < MinDownPaymentPercent || p.MinDownPaymentPercent > MaxDownPaymentPercent {
		return fmt.Errorf("min_down_payment_percent must be between %d and %d", MinDownPaymentPercent, MaxDownPaymentPercent)
	}
	if p.MaxDurationMonths < MinDurationMonths || p.MaxDurationMonths > MaxDurationMonths {
		return fmt.Errorf("max_duration_months must be between %d and %d", MinDurationMonths, MaxDurationMonths)
	}
	if p.InterestRateBps < MinInterestRateBps || p.InterestRateBps > MaxInterestRateBps {
		return fmt.Errorf("interest_rate_bps must be between %d and %d", MinInterestRateBps, MaxInterestRateBps)
	}
	if p.PenaltyRateBps < MinPenaltyRateBps || p.PenaltyRateBps > MaxPenaltyRateBps {
		return fmt.Errorf("penalty_rate_bps must be between %d and %d", MinPenaltyRateBps, MaxPenaltyRateBps)
	}
	if p.MinCreditScore < MinCreditScore || p.MinCreditScore > MaxCreditScore {
		return fmt.Errorf("min_credit_score must be between %d and %d", MinCreditScore, MaxCreditScore)
	}
	return nil
}

// InterestRatePercent retourne le taux d'intérêt en pourcentage (pour affichage)
func (p *CreditPlan) InterestRatePercent() float64 {
	return float64(p.InterestRateBps) / 100.0
}

// ============================================================
// CREDIT APPLICATION (Demande de crédit)
// ============================================================

// CreditApplication représente une demande de crédit soumise par un client
type CreditApplication struct {
	ID         string `json:"id" db:"id"`
	CustomerID string `json:"customer_id" db:"customer_id"`
	ProductID  string `json:"product_id" db:"product_id"`
	ShopID     string `json:"shop_id" db:"shop_id"`

	// Demande
	RequestedDurationMonths  int `json:"requested_duration_months" db:"requested_duration_months"`
	CreditScoreAtApplication int `json:"credit_score_at_application" db:"credit_score_at_application"`

	// Calculs
	ProductPriceCents   int64 `json:"product_price_cents" db:"product_price_cents"`
	DownPaymentCents    int64 `json:"down_payment_cents" db:"down_payment_cents"`
	FinancedAmountCents int64 `json:"financed_amount_cents" db:"financed_amount_cents"`
	InterestAmountCents int64 `json:"interest_amount_cents" db:"interest_amount_cents"`
	TotalAmountCents    int64 `json:"total_amount_cents" db:"total_amount_cents"`
	MonthlyPaymentCents int64 `json:"monthly_payment_cents" db:"monthly_payment_cents"`

	// Statut
	Status          CreditApplicationStatus `json:"status" db:"status"`
	RejectionReason *string                 `json:"rejection_reason,omitempty" db:"rejection_reason"`
	ReviewedBy      *string                 `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt      *time.Time              `json:"reviewed_at,omitempty" db:"reviewed_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewCreditApplication crée une nouvelle demande de crédit
func NewCreditApplication(
	customerID, productID, shopID string,
	requestedDurationMonths int,
	creditScore int,
	productPriceCents int64,
	downPaymentPercent int,
	interestRateBps int,
) (*CreditApplication, error) {
	// Validation durée
	if requestedDurationMonths < MinDurationMonths || requestedDurationMonths > MaxDurationMonths {
		return nil, fmt.Errorf("requested_duration_months must be between %d and %d",
			MinDurationMonths, MaxDurationMonths)
	}

	// Calcul apport initial
	downPaymentCents := (productPriceCents * int64(downPaymentPercent)) / 100
	financedAmountCents := productPriceCents - downPaymentCents

	// Calcul intérêts
	interestAmountCents := (financedAmountCents * int64(interestRateBps)) / 10000
	totalAmountCents := financedAmountCents + interestAmountCents

	// Calcul mensualité
	monthlyPaymentCents := totalAmountCents / int64(requestedDurationMonths)

	return &CreditApplication{
		CustomerID:               customerID,
		ProductID:                productID,
		ShopID:                   shopID,
		RequestedDurationMonths:  requestedDurationMonths,
		CreditScoreAtApplication: creditScore,
		ProductPriceCents:        productPriceCents,
		DownPaymentCents:         downPaymentCents,
		FinancedAmountCents:      financedAmountCents,
		InterestAmountCents:      interestAmountCents,
		TotalAmountCents:         totalAmountCents,
		MonthlyPaymentCents:      monthlyPaymentCents,
		Status:                   CreditApplicationPending,
	}, nil
}

// Approve approuve la demande de crédit
func (a *CreditApplication) Approve(reviewerID string) error {
	if a.Status != CreditApplicationPending {
		return fmt.Errorf("cannot approve application with status: %s", a.Status)
	}
	now := time.Now().UTC()
	a.Status = CreditApplicationApproved
	a.ReviewedBy = &reviewerID
	a.ReviewedAt = &now
	a.UpdatedAt = now
	return nil
}

// Reject rejette la demande de crédit
func (a *CreditApplication) Reject(reviewerID, reason string) error {
	if a.Status != CreditApplicationPending {
		return fmt.Errorf("cannot reject application with status: %s", a.Status)
	}
	if reason == "" {
		return errors.New("rejection reason is required")
	}
	now := time.Now().UTC()
	a.Status = CreditApplicationRejected
	a.ReviewedBy = &reviewerID
	a.ReviewedAt = &now
	a.RejectionReason = &reason
	a.UpdatedAt = now
	return nil
}

// Cancel annule la demande
func (a *CreditApplication) Cancel() error {
	if a.Status != CreditApplicationPending {
		return fmt.Errorf("cannot cancel application with status: %s", a.Status)
	}
	a.Status = CreditApplicationCancelled
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// IsPending vérifie si la demande est en attente
func (a *CreditApplication) IsPending() bool {
	return a.Status == CreditApplicationPending
}

// IsApproved vérifie si la demande est approuvée
func (a *CreditApplication) IsApproved() bool {
	return a.Status == CreditApplicationApproved
}

// ============================================================
// CREDIT CONTRACT (Contrat de crédit)
// ============================================================

// CreditContract représente un contrat de crédit actif
type CreditContract struct {
	ID            string `json:"id" db:"id"`
	ApplicationID string `json:"application_id" db:"application_id"`
	CustomerID    string `json:"customer_id" db:"customer_id"`
	ProductID     string `json:"product_id" db:"product_id"`
	ShopID        string `json:"shop_id" db:"shop_id"`

	// Montants
	ProductPriceCents   int64 `json:"product_price_cents" db:"product_price_cents"`
	DownPaymentCents    int64 `json:"down_payment_cents" db:"down_payment_cents"`
	FinancedAmountCents int64 `json:"financed_amount_cents" db:"financed_amount_cents"`
	InterestAmountCents int64 `json:"interest_amount_cents" db:"interest_amount_cents"`
	TotalAmountCents    int64 `json:"total_amount_cents" db:"total_amount_cents"`
	MonthlyPaymentCents int64 `json:"monthly_payment_cents" db:"monthly_payment_cents"`

	// Durée
	DurationMonths int       `json:"duration_months" db:"duration_months"`
	StartDate      time.Time `json:"start_date" db:"start_date"`
	EndDate        time.Time `json:"end_date" db:"end_date"`

	// Statut
	Status            CreditContractStatus `json:"status" db:"status"`
	DownPaymentPaidAt *time.Time           `json:"down_payment_paid_at,omitempty" db:"down_payment_paid_at"`
	CompletedAt       *time.Time           `json:"completed_at,omitempty" db:"completed_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewCreditContract crée un nouveau contrat à partir d'une demande approuvée
func NewCreditContract(app *CreditApplication) (*CreditContract, error) {
	if !app.IsApproved() {
		return nil, errors.New("application must be approved to create contract")
	}

	now := time.Now().UTC()
	endDate := now.AddDate(0, app.RequestedDurationMonths, 0)

	return &CreditContract{
		ApplicationID:       app.ID,
		CustomerID:          app.CustomerID,
		ProductID:           app.ProductID,
		ShopID:              app.ShopID,
		ProductPriceCents:   app.ProductPriceCents,
		DownPaymentCents:    app.DownPaymentCents,
		FinancedAmountCents: app.FinancedAmountCents,
		InterestAmountCents: app.InterestAmountCents,
		TotalAmountCents:    app.TotalAmountCents,
		MonthlyPaymentCents: app.MonthlyPaymentCents,
		DurationMonths:      app.RequestedDurationMonths,
		StartDate:           now,
		EndDate:             endDate,
		Status:              CreditContractActive,
	}, nil
}

// MarkDownPaymentPaid marque l'apport initial comme payé
func (c *CreditContract) MarkDownPaymentPaid() error {
	if c.DownPaymentPaidAt != nil {
		return errors.New("down payment already paid")
	}
	now := time.Now().UTC()
	c.DownPaymentPaidAt = &now
	c.UpdatedAt = now
	return nil
}

// Complete marque le contrat comme terminé
func (c *CreditContract) Complete() error {
	if c.Status != CreditContractActive {
		return fmt.Errorf("cannot complete contract with status: %s", c.Status)
	}
	now := time.Now().UTC()
	c.Status = CreditContractCompleted
	c.CompletedAt = &now
	c.UpdatedAt = now
	return nil
}

// Default marque le contrat comme en défaut
func (c *CreditContract) Default() error {
	if c.Status != CreditContractActive {
		return fmt.Errorf("cannot default contract with status: %s", c.Status)
	}
	c.Status = CreditContractDefaulted
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// IsActive vérifie si le contrat est actif
func (c *CreditContract) IsActive() bool {
	return c.Status == CreditContractActive
}

// IsCompleted vérifie si le contrat est terminé
func (c *CreditContract) IsCompleted() bool {
	return c.Status == CreditContractCompleted
}

// ============================================================
// CREDIT INSTALLMENT (Échéance mensuelle)
// ============================================================

// CreditInstallment représente une échéance mensuelle
type CreditInstallment struct {
	ID                string    `json:"id" db:"id"`
	ContractID        string    `json:"contract_id" db:"contract_id"`
	InstallmentNumber int       `json:"installment_number" db:"installment_number"`
	DueDate           time.Time `json:"due_date" db:"due_date"`
	AmountCents       int64     `json:"amount_cents" db:"amount_cents"`

	// Paiement
	PaymentID *string    `json:"payment_id,omitempty" db:"payment_id"`
	PaidAt    *time.Time `json:"paid_at,omitempty" db:"paid_at"`

	// Statut
	Status       InstallmentStatus `json:"status" db:"status"`
	LateFeeCents int64             `json:"late_fee_cents" db:"late_fee_cents"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewCreditInstallment crée une nouvelle échéance
func NewCreditInstallment(
	contractID string,
	installmentNumber int,
	dueDate time.Time,
	amountCents int64,
) *CreditInstallment {
	return &CreditInstallment{
		ContractID:        contractID,
		InstallmentNumber: installmentNumber,
		DueDate:           dueDate,
		AmountCents:       amountCents,
		Status:            InstallmentPending,
	}
}

// MarkPaid marque l'échéance comme payée
func (i *CreditInstallment) MarkPaid(paymentID string) error {
	if i.Status == InstallmentPaid {
		return errors.New("installment already paid")
	}
	now := time.Now().UTC()
	i.Status = InstallmentPaid
	i.PaymentID = &paymentID
	i.PaidAt = &now
	i.UpdatedAt = now
	return nil
}

// MarkLate marque l'échéance comme en retard
func (i *CreditInstallment) MarkLate(lateFeeCents int64) error {
	if i.Status != InstallmentPending {
		return fmt.Errorf("cannot mark as late with status: %s", i.Status)
	}
	i.Status = InstallmentLate
	i.LateFeeCents = lateFeeCents
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkDefaulted marque l'échéance comme en défaut
func (i *CreditInstallment) MarkDefaulted() error {
	if i.Status != InstallmentLate {
		return fmt.Errorf("cannot default installment with status: %s", i.Status)
	}
	i.Status = InstallmentDefaulted
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// IsPending vérifie si l'échéance est en attente
func (i *CreditInstallment) IsPending() bool {
	return i.Status == InstallmentPending
}

// IsPaid vérifie si l'échéance est payée
func (i *CreditInstallment) IsPaid() bool {
	return i.Status == InstallmentPaid
}

// IsLate vérifie si l'échéance est en retard
func (i *CreditInstallment) IsLate() bool {
	return i.Status == InstallmentLate
}

// IsOverdue vérifie si l'échéance est échue (due_date < now)
func (i *CreditInstallment) IsOverdue() bool {
	if i.IsPaid() {
		return false
	}
	return time.Now().UTC().After(i.DueDate)
}

// DaysOverdue retourne le nombre de jours de retard
func (i *CreditInstallment) DaysOverdue() int {
	if !i.IsOverdue() {
		return 0
	}
	return int(time.Since(i.DueDate).Hours() / 24)
}

// ============================================================
// CREDIT SCORE (Score de fiabilité)
// ============================================================

// CreditScore représente le score de fiabilité d'un client
type CreditScore struct {
	ID         string `json:"id" db:"id"`
	CustomerID string `json:"customer_id" db:"customer_id"`
	ShopID     string `json:"shop_id" db:"shop_id"`

	// Score (0-1000)
	Score int `json:"score" db:"score"`

	// Statistiques
	TotalContracts     int `json:"total_contracts" db:"total_contracts"`
	CompletedContracts int `json:"completed_contracts" db:"completed_contracts"`
	OnTimePayments     int `json:"on_time_payments" db:"on_time_payments"`
	LatePayments       int `json:"late_payments" db:"late_payments"`
	Defaults           int `json:"defaults" db:"defaults"`

	LastUpdatedAt time.Time `json:"last_updated_at" db:"last_updated_at"`
}

// NewCreditScore crée un nouveau score avec valeur par défaut (500)
func NewCreditScore(customerID, shopID string) *CreditScore {
	now := time.Now().UTC()
	return &CreditScore{
		CustomerID:    customerID,
		ShopID:        shopID,
		Score:         500, // Score initial neutre
		LastUpdatedAt: now,
	}
}

// UpdateScore recalcule le score basé sur l'historique
func (s *CreditScore) UpdateScore() {
	// Formule de calcul
	// Score initial : 500
	// +10 par paiement à temps
	// -20 par paiement en retard
	// -100 par défaut
	// +50 bonus après 1er contrat complété

	score := 500
	score += s.OnTimePayments * 10
	score -= s.LatePayments * 20
	score -= s.Defaults * 100

	if s.CompletedContracts > 0 {
		score += 50
	}

	// Clamp entre 0 et 1000
	if score < 0 {
		score = 0
	}
	if score > 1000 {
		score = 1000
	}

	s.Score = score
	s.LastUpdatedAt = time.Now().UTC()
}

// RecordOnTimePayment enregistre un paiement à temps
func (s *CreditScore) RecordOnTimePayment() {
	s.OnTimePayments++
	s.UpdateScore()
}

// RecordLatePayment enregistre un paiement en retard
func (s *CreditScore) RecordLatePayment() {
	s.LatePayments++
	s.UpdateScore()
}

// RecordDefault enregistre un défaut
func (s *CreditScore) RecordDefault() {
	s.Defaults++
	s.UpdateScore()
}

// RecordCompletedContract enregistre un contrat terminé
func (s *CreditScore) RecordCompletedContract() {
	s.CompletedContracts++
	s.TotalContracts++
	s.UpdateScore()
}

// RecordNewContract enregistre un nouveau contrat
func (s *CreditScore) RecordNewContract() {
	s.TotalContracts++
	s.UpdateScore()
}

// IsExcellent vérifie si le score est excellent (≥700)
func (s *CreditScore) IsExcellent() bool {
	return s.Score >= ScoreExcellent
}

// IsGood vérifie si le score est bon (≥500)
func (s *CreditScore) IsGood() bool {
	return s.Score >= ScoreGood
}

// IsMedium vérifie si le score est moyen (≥300)
func (s *CreditScore) IsMedium() bool {
	return s.Score >= ScoreMedium
}

// IsBad vérifie si le score est mauvais (<300)
func (s *CreditScore) IsBad() bool {
	return s.Score < ScoreMedium
}

// MeetsMinRequirement vérifie si le score atteint le minimum requis
func (s *CreditScore) MeetsMinRequirement(minScore int) bool {
	return s.Score >= minScore
}

// ============================================================
// HELPERS DE CALCUL
// ============================================================

// CalculateCreditDetails calcule tous les détails d'un crédit
// Retourne : downPayment, financedAmount, interestAmount, totalAmount, monthlyPayment
func CalculateCreditDetails(
	productPriceCents int64,
	downPaymentPercent int,
	durationMonths int,
	interestRateBps int,
) (downPayment, financedAmount, interestAmount, totalAmount, monthlyPayment int64) {
	// Apport initial
	downPayment = (productPriceCents * int64(downPaymentPercent)) / 100

	// Montant financé
	financedAmount = productPriceCents - downPayment

	// Intérêts
	interestAmount = (financedAmount * int64(interestRateBps)) / 10000

	// Total à rembourser
	totalAmount = financedAmount + interestAmount

	// Mensualité
	monthlyPayment = totalAmount / int64(durationMonths)

	return
}

// CalculateLateFee calcule la pénalité de retard
func CalculateLateFee(amountCents int64, penaltyRateBps int) int64 {
	return (amountCents * int64(penaltyRateBps)) / 10000
}
