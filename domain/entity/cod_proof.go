package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ENUMS COD
// ============================================================

// CODProofStatus représente le statut d'une preuve COD
type CODProofStatus string

const (
	CODProofPendingProofs     CODProofStatus = "pending_proofs"      // En attente des preuves
	CODProofClientProofSent   CODProofStatus = "client_proof_sent"   // Client a uploadé
	CODProofMerchantProofSent CODProofStatus = "merchant_proof_sent" // Marchand a uploadé
	CODProofConfirmed         CODProofStatus = "confirmed"           // Les deux preuves cohérentes
	CODProofDisputed          CODProofStatus = "disputed"            // Litige en cours
	CODProofResolved          CODProofStatus = "resolved"            // Litige résolu
	CODProofCompleted         CODProofStatus = "completed"           // Commission collectée
)

// IsValid vérifie si le statut est valide
func (s CODProofStatus) IsValid() bool {
	switch s {
	case CODProofPendingProofs, CODProofClientProofSent, CODProofMerchantProofSent,
		CODProofConfirmed, CODProofDisputed, CODProofResolved, CODProofCompleted:
		return true
	}
	return false
}

// IsTerminal vérifie si le statut est terminal
func (s CODProofStatus) IsTerminal() bool {
	return s == CODProofCompleted || s == CODProofResolved
}

// CODCommissionStatus représente le statut de la commission COD
type CODCommissionStatus string

const (
	CODCommissionPending   CODCommissionStatus = "pending"   // Commission pas encore collectée
	CODCommissionCollected CODCommissionStatus = "collected" // Commission prélevée du wallet
	CODCommissionDue       CODCommissionStatus = "due"       // Commission due (wallet négatif)
	CODCommissionWaived    CODCommissionStatus = "waived"    // Commission annulée (litige)
)

// IsValid vérifie si le statut est valide
func (s CODCommissionStatus) IsValid() bool {
	switch s {
	case CODCommissionPending, CODCommissionCollected,
		CODCommissionDue, CODCommissionWaived:
		return true
	}
	return false
}

// ============================================================
// CONSTANTES COD
// ============================================================

const (
	// Délais
	CODProofDeadlineDays = 7   // 7 jours pour soumettre les preuves
	CODCommissionRateBps = 250 // 2.50% par défaut

	// Tolérances
	CODAmountToleranceCents = 100 // Tolérance de 1 FCFA (arrondis)
	CODDateToleranceHours   = 24  // Tolérance de 24h entre dates
)

// ============================================================
// COD PROOF (Preuve de paiement à la livraison)
// ============================================================

// CODProof représente les preuves de paiement cash à la livraison
type CODProof struct {
	ID         string `json:"id" db:"id"`
	OrderID    string `json:"order_id" db:"order_id"`
	ShopID     string `json:"shop_id" db:"shop_id"`
	CustomerID string `json:"customer_id" db:"customer_id"`

	// ========================================================
	// PREUVE CLIENT (a payé au marchand)
	// ========================================================
	ClientPaymentProofURL    *string    `json:"client_payment_proof_url,omitempty" db:"client_payment_proof_url"`
	ClientPaymentAmountCents *int64     `json:"client_payment_amount_cents,omitempty" db:"client_payment_amount_cents"`
	ClientPaymentDate        *time.Time `json:"client_payment_date,omitempty" db:"client_payment_date"`
	ClientReceiptNumber      *string    `json:"client_receipt_number,omitempty" db:"client_receipt_number"`
	ClientNotes              *string    `json:"client_notes,omitempty" db:"client_notes"`
	ClientSubmittedAt        *time.Time `json:"client_submitted_at,omitempty" db:"client_submitted_at"`

	// ========================================================
	// PREUVE MARCHAND (a reçu du client)
	// ========================================================
	MerchantReceiptProofURL     *string    `json:"merchant_receipt_proof_url,omitempty" db:"merchant_receipt_proof_url"`
	MerchantReceivedAmountCents *int64     `json:"merchant_received_amount_cents,omitempty" db:"merchant_received_amount_cents"`
	MerchantReceiptDate         *time.Time `json:"merchant_receipt_date,omitempty" db:"merchant_receipt_date"`
	MerchantNotes               *string    `json:"merchant_notes,omitempty" db:"merchant_notes"`
	MerchantSubmittedAt         *time.Time `json:"merchant_submitted_at,omitempty" db:"merchant_submitted_at"`

	// ========================================================
	// COHÉRENCE
	// ========================================================
	AmountsMatch *bool `json:"amounts_match,omitempty" db:"amounts_match"`
	DatesMatch   *bool `json:"dates_match,omitempty" db:"dates_match"`

	// ========================================================
	// COMMISSION
	// ========================================================
	CommissionCents       int64               `json:"commission_cents" db:"commission_cents"`
	CommissionStatus      CODCommissionStatus `json:"commission_status" db:"commission_status"`
	CommissionCollectedAt *time.Time          `json:"commission_collected_at,omitempty" db:"commission_collected_at"`

	// ========================================================
	// STATUT GLOBAL
	// ========================================================
	Status CODProofStatus `json:"status" db:"status"`

	// ========================================================
	// LITIGE
	// ========================================================
	DisputeRaisedAt   *time.Time `json:"dispute_raised_at,omitempty" db:"dispute_raised_at"`
	DisputeReason     *string    `json:"dispute_reason,omitempty" db:"dispute_reason"`
	DisputeResolvedAt *time.Time `json:"dispute_resolved_at,omitempty" db:"dispute_resolved_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ============================================================
// CONSTRUCTEUR
// ============================================================

// NewCODProof crée une nouvelle preuve COD
func NewCODProof(orderID, shopID, customerID string, orderAmountCents int64) (*CODProof, error) {
	if orderID == "" {
		return nil, errors.New("order_id is required")
	}
	if shopID == "" {
		return nil, errors.New("shop_id is required")
	}
	if customerID == "" {
		return nil, errors.New("customer_id is required")
	}
	if orderAmountCents <= 0 {
		return nil, errors.New("order_amount_cents must be positive")
	}

	// Calculer la commission (2.50% par défaut)
	commissionCents := CalculateCommission(orderAmountCents, CODCommissionRateBps)

	now := time.Now().UTC()
	return &CODProof{
		OrderID:          orderID,
		ShopID:           shopID,
		CustomerID:       customerID,
		CommissionCents:  commissionCents,
		CommissionStatus: CODCommissionPending,
		Status:           CODProofPendingProofs,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// ============================================================
// MÉTHODES : CLIENT (soumission preuve)
// ============================================================

// SubmitClientProof permet au client de soumettre sa preuve de paiement
func (p *CODProof) SubmitClientProof(
	proofURL string,
	amountCents int64,
	paymentDate time.Time,
	receiptNumber, notes string,
) error {
	if p.Status != CODProofPendingProofs && p.Status != CODProofMerchantProofSent {
		return fmt.Errorf("cannot submit client proof with status: %s", p.Status)
	}
	if proofURL == "" {
		return errors.New("client_payment_proof_url is required")
	}
	if amountCents <= 0 {
		return errors.New("client_payment_amount_cents must be positive")
	}

	now := time.Now().UTC()
	p.ClientPaymentProofURL = &proofURL
	p.ClientPaymentAmountCents = &amountCents
	p.ClientPaymentDate = &paymentDate
	p.ClientSubmittedAt = &now

	if receiptNumber != "" {
		p.ClientReceiptNumber = &receiptNumber
	}
	if notes != "" {
		p.ClientNotes = &notes
	}

	// Mise à jour statut
	switch p.Status {
	case CODProofPendingProofs, CODProofMerchantProofSent:
		// OK, on continue
	default:
		return fmt.Errorf("cannot submit client proof with status: %s", p.Status)
	}

	p.UpdatedAt = now
	return nil
}

// HasClientProof vérifie si le client a soumis sa preuve
func (p *CODProof) HasClientProof() bool {
	return p.ClientPaymentProofURL != nil && p.ClientPaymentDate != nil
}

// ============================================================
// MÉTHODES : MARCHAND (soumission preuve)
// ============================================================

// SubmitMerchantProof permet au marchand de soumettre sa preuve de réception
func (p *CODProof) SubmitMerchantProof(
	proofURL string,
	amountCents int64,
	receiptDate time.Time,
	notes string,
) error {
	if p.Status != CODProofPendingProofs && p.Status != CODProofClientProofSent {
		return fmt.Errorf("cannot submit merchant proof with status: %s", p.Status)
	}
	if proofURL == "" {
		return errors.New("merchant_receipt_proof_url is required")
	}
	if amountCents <= 0 {
		return errors.New("merchant_received_amount_cents must be positive")
	}

	now := time.Now().UTC()
	p.MerchantReceiptProofURL = &proofURL
	p.MerchantReceivedAmountCents = &amountCents
	p.MerchantReceiptDate = &receiptDate
	p.MerchantSubmittedAt = &now

	if notes != "" {
		p.MerchantNotes = &notes
	}

	// Mise à jour statut
	switch p.Status {
	case CODProofPendingProofs, CODProofClientProofSent:
		// OK, on continue
	default:
		return fmt.Errorf("cannot submit merchant proof with status: %s", p.Status)
	}

	p.UpdatedAt = now
	return nil
}

// HasMerchantProof vérifie si le marchand a soumis sa preuve
func (p *CODProof) HasMerchantProof() bool {
	return p.MerchantReceiptProofURL != nil && p.MerchantReceiptDate != nil
}

// ============================================================
// MÉTHODES : VÉRIFICATION COHÉRENCE
// ============================================================

// VerifyCoherence vérifie la cohérence entre les preuves client et marchand
func (p *CODProof) VerifyCoherence() {
	if !p.HasClientProof() || !p.HasMerchantProof() {
		return
	}

	// Vérifier cohérence montants
	amountsMatch := false
	if p.ClientPaymentAmountCents != nil && p.MerchantReceivedAmountCents != nil {
		diff := *p.ClientPaymentAmountCents - *p.MerchantReceivedAmountCents
		if diff < 0 {
			diff = -diff
		}
		amountsMatch = diff <= CODAmountToleranceCents
	}
	p.AmountsMatch = &amountsMatch

	// Vérifier cohérence dates
	datesMatch := false
	if p.ClientPaymentDate != nil && p.MerchantReceiptDate != nil {
		diff := p.ClientPaymentDate.Sub(*p.MerchantReceiptDate)
		if diff < 0 {
			diff = -diff
		}
		datesMatch = diff.Hours() <= CODDateToleranceHours
	}
	p.DatesMatch = &datesMatch
}

// IsCoherent vérifie si les preuves sont cohérentes
func (p *CODProof) IsCoherent() bool {
	if !p.IsComplete() {
		return false
	}
	return p.AmountsMatch != nil && *p.AmountsMatch &&
		p.DatesMatch != nil && *p.DatesMatch
}

// ============================================================
// MÉTHODES : COMMISSION
// ============================================================

// MarkCommissionCollected marque la commission comme collectée
func (p *CODProof) MarkCommissionCollected() error {
	if p.CommissionStatus != CODCommissionPending && p.CommissionStatus != CODCommissionDue {
		return fmt.Errorf("cannot collect commission with status: %s", p.CommissionStatus)
	}
	if !p.IsCoherent() {
		return errors.New("proofs must be coherent before collecting commission")
	}

	now := time.Now().UTC()
	p.CommissionStatus = CODCommissionCollected
	p.CommissionCollectedAt = &now
	p.Status = CODProofCompleted
	p.UpdatedAt = now
	return nil
}

// MarkCommissionDue marque la commission comme due (wallet négatif)
func (p *CODProof) MarkCommissionDue() error {
	if p.CommissionStatus != CODCommissionPending {
		return fmt.Errorf("cannot mark commission due with status: %s", p.CommissionStatus)
	}

	p.CommissionStatus = CODCommissionDue
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// WaiveCommission annule la commission (après litige résolu en faveur du client)
func (p *CODProof) WaiveCommission() error {
	if p.CommissionStatus == CODCommissionCollected {
		return errors.New("cannot waive already collected commission")
	}

	p.CommissionStatus = CODCommissionWaived
	p.Status = CODProofResolved
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// CanCollectCommission vérifie si la commission peut être collectée
func (p *CODProof) CanCollectCommission() bool {
	return p.IsCoherent() &&
		(p.CommissionStatus == CODCommissionPending || p.CommissionStatus == CODCommissionDue)
}

// ============================================================
// MÉTHODES : LITIGE
// ============================================================

// RaiseDispute ouvre un litige
func (p *CODProof) RaiseDispute(reason string) error {
	if p.Status.IsTerminal() {
		return fmt.Errorf("cannot raise dispute with terminal status: %s", p.Status)
	}
	if reason == "" {
		return errors.New("dispute reason is required")
	}

	now := time.Now().UTC()
	p.Status = CODProofDisputed
	p.DisputeRaisedAt = &now
	p.DisputeReason = &reason
	p.UpdatedAt = now
	return nil
}

// ResolveDispute résout un litige
func (p *CODProof) ResolveDispute(collectCommission bool) error {
	if p.Status != CODProofDisputed {
		return fmt.Errorf("cannot resolve dispute with status: %s", p.Status)
	}

	now := time.Now().UTC()
	p.DisputeResolvedAt = &now
	p.Status = CODProofResolved

	if collectCommission {
		return p.MarkCommissionCollected()
	}
	return p.WaiveCommission()
}

// ============================================================
// MÉTHODES DE REQUÊTE
// ============================================================

// IsPending vérifie si en attente de preuves
func (p *CODProof) IsPending() bool {
	return p.Status == CODProofPendingProofs
}

// IsComplete vérifie si les deux preuves sont soumises
func (p *CODProof) IsComplete() bool {
	return p.HasClientProof() && p.HasMerchantProof()
}

// IsConfirmed vérifie si confirmé (preuves cohérentes)
func (p *CODProof) IsConfirmed() bool {
	return p.Status == CODProofConfirmed
}

// IsDisputed vérifie si en litige
func (p *CODProof) IsDisputed() bool {
	return p.Status == CODProofDisputed
}

// IsCompleted vérifie si terminé (commission collectée)
func (p *CODProof) IsCompleted() bool {
	return p.Status == CODProofCompleted
}

// IsResolved vérifie si litige résolu
func (p *CODProof) IsResolved() bool {
	return p.Status == CODProofResolved
}

// ProofDeadline retourne la date limite pour soumettre les preuves
func (p *CODProof) ProofDeadline() time.Time {
	return p.CreatedAt.AddDate(0, 0, CODProofDeadlineDays)
}

// IsPastDeadline vérifie si la date limite est dépassée
func (p *CODProof) IsPastDeadline() bool {
	return time.Now().UTC().After(p.ProofDeadline())
}

// DaysUntilDeadline retourne le nombre de jours restants
func (p *CODProof) DaysUntilDeadline() int {
	if p.IsPastDeadline() {
		return 0
	}
	remaining := time.Until(p.ProofDeadline())
	return int(remaining.Hours() / 24)
}

// ============================================================
// VALIDATION
// ============================================================

// Validate valide la preuve COD
func (p *CODProof) Validate() error {
	if p.OrderID == "" {
		return errors.New("order_id is required")
	}
	if p.ShopID == "" {
		return errors.New("shop_id is required")
	}
	if p.CustomerID == "" {
		return errors.New("customer_id is required")
	}
	if p.CommissionCents < 0 {
		return errors.New("commission_cents cannot be negative")
	}
	if !p.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", p.Status)
	}
	if !p.CommissionStatus.IsValid() {
		return fmt.Errorf("invalid commission status: %s", p.CommissionStatus)
	}
	return nil
}
