package entity

import (
	"errors"
	"fmt"
	"time"
)

// ============================================================
// ENUMS ESCROW / PREUVES DE LIVRAISON
// ============================================================

// EscrowStatus représente le statut du compte séquestre
type EscrowStatus string

const (
	EscrowPendingShipment EscrowStatus = "pending_shipment" // En attente envoi marchand
	EscrowShipped         EscrowStatus = "shipped"          // Marchand a envoyé preuve
	EscrowDelivered       EscrowStatus = "delivered"        // Client a confirmé réception
	EscrowDisputed        EscrowStatus = "disputed"         // Litige en cours
	EscrowReleased        EscrowStatus = "released"         // Argent débloqué au marchand
	EscrowRefunded        EscrowStatus = "refunded"         // Argent remboursé au client
)

// IsValid vérifie si le statut est valide
func (s EscrowStatus) IsValid() bool {
	switch s {
	case EscrowPendingShipment, EscrowShipped, EscrowDelivered,
		EscrowDisputed, EscrowReleased, EscrowRefunded:
		return true
	}
	return false
}

// IsTerminal vérifie si le statut est terminal (plus de transition possible)
func (s EscrowStatus) IsTerminal() bool {
	return s == EscrowReleased || s == EscrowRefunded
}

// ProofEventType représente le type d'événement d'audit escrow
type ProofEventType string

const (
	ProofEventSubmitted       ProofEventType = "proof_submitted"
	ProofEventStatusChanged   ProofEventType = "status_changed"
	ProofEventDisputeRaised   ProofEventType = "dispute_raised"
	ProofEventDisputeResolved ProofEventType = "dispute_resolved"
	ProofEventFundsReleased   ProofEventType = "funds_released"
	ProofEventFundsRefunded   ProofEventType = "funds_refunded"
)

// IsValid vérifie si le type d'événement est valide
func (t ProofEventType) IsValid() bool {
	switch t {
	case ProofEventSubmitted, ProofEventStatusChanged,
		ProofEventDisputeRaised, ProofEventDisputeResolved,
		ProofEventFundsReleased, ProofEventFundsRefunded:
		return true
	}
	return false
}

// ============================================================
// CONSTANTES ESCROW
// ============================================================

const (
	// Délais
	DisputeWindowHours = 72 // 72h pour contester après livraison
	AutoReleaseDays    = 3  // Déblocage auto après 3 jours sans litige
	MinRating          = 1  // Note minimale
	MaxRating          = 5  // Note maximale

	// Types de preuve
	ProofTypeShipping = "shipping"
	ProofTypeDelivery = "delivery"
)

// ============================================================
// DELIVERY PROOF (Preuve de livraison unifiée)
// ============================================================

// DeliveryProof représente les preuves de livraison (marchand + client)
// Utilisé pour TOUS les modes d'achat : Cash, Crédit, Tontine, COD
type DeliveryProof struct {
	ID string `json:"id" db:"id"`

	// Référence (une seule doit être non-nulle)
	OrderID          *string `json:"order_id,omitempty" db:"order_id"`
	CreditContractID *string `json:"credit_contract_id,omitempty" db:"credit_contract_id"`
	TontineVoucherID *string `json:"tontine_voucher_id,omitempty" db:"tontine_voucher_id"`

	// ========================================================
	// PREUVE MARCHAND (envoi)
	// ========================================================
	ShippingProofURL       *string    `json:"shipping_proof_url,omitempty" db:"shipping_proof_url"`
	ShippingTrackingNumber *string    `json:"shipping_tracking_number,omitempty" db:"shipping_tracking_number"`
	ShippingCarrier        *string    `json:"shipping_carrier,omitempty" db:"shipping_carrier"`
	ShippingDate           *time.Time `json:"shipping_date,omitempty" db:"shipping_date"`
	ShippingNotes          *string    `json:"shipping_notes,omitempty" db:"shipping_notes"`

	// ========================================================
	// PREUVE CLIENT (réception)
	// ========================================================
	DeliveryProofURL  *string    `json:"delivery_proof_url,omitempty" db:"delivery_proof_url"`
	DeliverySignature *string    `json:"delivery_signature,omitempty" db:"delivery_signature"`
	DeliveryDate      *time.Time `json:"delivery_date,omitempty" db:"delivery_date"`
	DeliveryNotes     *string    `json:"delivery_notes,omitempty" db:"delivery_notes"`
	DeliveryRating    *int       `json:"delivery_rating,omitempty" db:"delivery_rating"`

	// ========================================================
	// STATUT ESCROW
	// ========================================================
	EscrowStatus EscrowStatus `json:"escrow_status" db:"escrow_status"`

	// ========================================================
	// LITIGE
	// ========================================================
	DisputeRaisedAt   *time.Time `json:"dispute_raised_at,omitempty" db:"dispute_raised_at"`
	DisputeReason     *string    `json:"dispute_reason,omitempty" db:"dispute_reason"`
	DisputeResolvedAt *time.Time `json:"dispute_resolved_at,omitempty" db:"dispute_resolved_at"`
	DisputeResolution *string    `json:"dispute_resolution,omitempty" db:"dispute_resolution"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NewDeliveryProofForOrder crée une preuve de livraison pour une commande
func NewDeliveryProofForOrder(orderID string) *DeliveryProof {
	now := time.Now().UTC()
	return &DeliveryProof{
		OrderID:      &orderID,
		EscrowStatus: EscrowPendingShipment,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// NewDeliveryProofForCreditContract crée une preuve pour un contrat de crédit
func NewDeliveryProofForCreditContract(contractID string) *DeliveryProof {
	now := time.Now().UTC()
	return &DeliveryProof{
		CreditContractID: &contractID,
		EscrowStatus:     EscrowPendingShipment,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// NewDeliveryProofForTontineVoucher crée une preuve pour un voucher tontine
func NewDeliveryProofForTontineVoucher(voucherID string) *DeliveryProof {
	now := time.Now().UTC()
	return &DeliveryProof{
		TontineVoucherID: &voucherID,
		EscrowStatus:     EscrowPendingShipment,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// ============================================================
// MÉTHODES : MARCHAND (envoi)
// ============================================================

// SubmitShippingProof permet au marchand de soumettre la preuve d'envoi
func (p *DeliveryProof) SubmitShippingProof(
	proofURL, trackingNumber, carrier, notes string,
) error {
	if p.EscrowStatus != EscrowPendingShipment {
		return fmt.Errorf("cannot submit shipping proof with escrow status: %s", p.EscrowStatus)
	}
	if proofURL == "" {
		return errors.New("shipping proof URL is required")
	}

	now := time.Now().UTC()
	p.ShippingProofURL = &proofURL
	p.ShippingDate = &now

	if trackingNumber != "" {
		p.ShippingTrackingNumber = &trackingNumber
	}
	if carrier != "" {
		p.ShippingCarrier = &carrier
	}
	if notes != "" {
		p.ShippingNotes = &notes
	}

	p.EscrowStatus = EscrowShipped
	p.UpdatedAt = now
	return nil
}

// HasShippingProof vérifie si le marchand a soumis la preuve d'envoi
func (p *DeliveryProof) HasShippingProof() bool {
	return p.ShippingProofURL != nil && p.ShippingDate != nil
}

// ============================================================
// MÉTHODES : CLIENT (réception)
// ============================================================

// SubmitDeliveryProof permet au client de confirmer la réception
func (p *DeliveryProof) SubmitDeliveryProof(
	proofURL, signature, notes string,
	rating *int,
) error {
	if p.EscrowStatus != EscrowShipped {
		return fmt.Errorf("cannot submit delivery proof with escrow status: %s", p.EscrowStatus)
	}
	if !p.HasShippingProof() {
		return errors.New("merchant must submit shipping proof first")
	}
	if proofURL == "" {
		return errors.New("delivery proof URL is required")
	}
	if rating != nil && (*rating < MinRating || *rating > MaxRating) {
		return fmt.Errorf("rating must be between %d and %d", MinRating, MaxRating)
	}

	now := time.Now().UTC()
	p.DeliveryProofURL = &proofURL
	p.DeliveryDate = &now

	if signature != "" {
		p.DeliverySignature = &signature
	}
	if notes != "" {
		p.DeliveryNotes = &notes
	}
	if rating != nil {
		p.DeliveryRating = rating
	}

	p.EscrowStatus = EscrowDelivered
	p.UpdatedAt = now
	return nil
}

// HasDeliveryProof vérifie si le client a soumis la preuve de réception
func (p *DeliveryProof) HasDeliveryProof() bool {
	return p.DeliveryProofURL != nil && p.DeliveryDate != nil
}

// ============================================================
// MÉTHODES : DÉBLOCAGE / REMBOURSEMENT
// ============================================================

// ReleaseFunds marque les fonds comme débloqués au marchand
func (p *DeliveryProof) ReleaseFunds() error {
	if p.EscrowStatus != EscrowDelivered {
		return fmt.Errorf("cannot release funds with escrow status: %s", p.EscrowStatus)
	}

	// Vérifier que la période de litige est passée
	if p.DeliveryDate != nil {
		disputeDeadline := p.DeliveryDate.Add(time.Duration(DisputeWindowHours) * time.Hour)
		if time.Now().UTC().Before(disputeDeadline) {
			return errors.New("dispute window still open")
		}
	}

	now := time.Now().UTC()
	p.EscrowStatus = EscrowReleased
	p.UpdatedAt = now
	return nil
}

// ForceReleaseFunds force le déblocage (admin)
func (p *DeliveryProof) ForceReleaseFunds() error {
	if p.EscrowStatus.IsTerminal() {
		return fmt.Errorf("cannot release funds with terminal status: %s", p.EscrowStatus)
	}

	now := time.Now().UTC()
	p.EscrowStatus = EscrowReleased
	p.UpdatedAt = now
	return nil
}

// RefundFunds marque les fonds comme remboursés au client
func (p *DeliveryProof) RefundFunds() error {
	if p.EscrowStatus.IsTerminal() {
		return fmt.Errorf("cannot refund funds with terminal status: %s", p.EscrowStatus)
	}

	now := time.Now().UTC()
	p.EscrowStatus = EscrowRefunded
	p.UpdatedAt = now
	return nil
}

// ============================================================
// MÉTHODES : LITIGE
// ============================================================

// RaiseDispute ouvre un litige
func (p *DeliveryProof) RaiseDispute(reason string) error {
	// Un litige peut être ouvert tant que les fonds ne sont pas débloqués
	if p.EscrowStatus == EscrowReleased || p.EscrowStatus == EscrowRefunded {
		return fmt.Errorf("cannot raise dispute with terminal status: %s", p.EscrowStatus)
	}
	if reason == "" {
		return errors.New("dispute reason is required")
	}

	// Vérifier la fenêtre de litige (72h après livraison)
	if p.DeliveryDate != nil {
		disputeDeadline := p.DeliveryDate.Add(time.Duration(DisputeWindowHours) * time.Hour)
		if time.Now().UTC().After(disputeDeadline) {
			return errors.New("dispute window has expired")
		}
	}

	now := time.Now().UTC()
	p.EscrowStatus = EscrowDisputed
	p.DisputeRaisedAt = &now
	p.DisputeReason = &reason
	p.UpdatedAt = now
	return nil
}

// ResolveDispute résout un litige
func (p *DeliveryProof) ResolveDispute(resolution string) error {
	if p.EscrowStatus != EscrowDisputed {
		return fmt.Errorf("cannot resolve dispute with status: %s", p.EscrowStatus)
	}
	if resolution == "" {
		return errors.New("dispute resolution is required")
	}

	now := time.Now().UTC()
	p.DisputeResolvedAt = &now
	p.DisputeResolution = &resolution
	p.UpdatedAt = now

	// Selon la résolution, transition vers released ou refunded
	switch resolution {
	case "released_to_merchant":
		p.EscrowStatus = EscrowReleased
	case "refunded_to_customer":
		p.EscrowStatus = EscrowRefunded
	default:
		return fmt.Errorf("invalid resolution: %s", resolution)
	}

	return nil
}

// ============================================================
// MÉTHODES : REQUÊTES
// ============================================================

// IsPendingShipment vérifie si en attente d'envoi
func (p *DeliveryProof) IsPendingShipment() bool {
	return p.EscrowStatus == EscrowPendingShipment
}

// IsShipped vérifie si expédié
func (p *DeliveryProof) IsShipped() bool {
	return p.EscrowStatus == EscrowShipped
}

// IsDelivered vérifie si livré
func (p *DeliveryProof) IsDelivered() bool {
	return p.EscrowStatus == EscrowDelivered
}

// IsDisputed vérifie si en litige
func (p *DeliveryProof) IsDisputed() bool {
	return p.EscrowStatus == EscrowDisputed
}

// IsReleased vérifie si débloqué
func (p *DeliveryProof) IsReleased() bool {
	return p.EscrowStatus == EscrowReleased
}

// IsRefunded vérifie si remboursé
func (p *DeliveryProof) IsRefunded() bool {
	return p.EscrowStatus == EscrowRefunded
}

// IsComplete vérifie si le processus est complet (les 2 preuves soumises)
func (p *DeliveryProof) IsComplete() bool {
	return p.HasShippingProof() && p.HasDeliveryProof()
}

// CanRaiseDispute vérifie si un litige peut être ouvert
func (p *DeliveryProof) CanRaiseDispute() bool {
	if p.EscrowStatus == EscrowReleased || p.EscrowStatus == EscrowRefunded {
		return false
	}
	if p.DeliveryDate == nil {
		return false
	}
	disputeDeadline := p.DeliveryDate.Add(time.Duration(DisputeWindowHours) * time.Hour)
	return time.Now().UTC().Before(disputeDeadline)
}

// DisputeDeadline retourne la date limite pour ouvrir un litige
func (p *DeliveryProof) DisputeDeadline() *time.Time {
	if p.DeliveryDate == nil {
		return nil
	}
	deadline := p.DeliveryDate.Add(time.Duration(DisputeWindowHours) * time.Hour)
	return &deadline
}

// AutoReleaseEligible vérifie si le déblocage auto est possible
func (p *DeliveryProof) AutoReleaseEligible() bool {
	if p.EscrowStatus != EscrowDelivered {
		return false
	}
	if p.DeliveryDate == nil {
		return false
	}
	autoReleaseDate := p.DeliveryDate.AddDate(0, 0, AutoReleaseDays)
	return time.Now().UTC().After(autoReleaseDate)
}

// ============================================================
// DELIVERY PROOF EVENT (Audit trail)
// ============================================================

// DeliveryProofEvent représente un événement d'audit escrow
type DeliveryProofEvent struct {
	ID              string                 `json:"id" db:"id"`
	DeliveryProofID string                 `json:"delivery_proof_id" db:"delivery_proof_id"`
	EventType       ProofEventType         `json:"event_type" db:"event_type"`
	EventData       map[string]interface{} `json:"event_data,omitempty" db:"event_data"`
	PerformedBy     string                 `json:"performed_by" db:"performed_by"`
	PerformedAt     time.Time              `json:"performed_at" db:"performed_at"`
}

// NewDeliveryProofEvent crée un nouvel événement d'audit
func NewDeliveryProofEvent(
	deliveryProofID string,
	eventType ProofEventType,
	performedBy string,
	eventData map[string]interface{},
) *DeliveryProofEvent {
	return &DeliveryProofEvent{
		DeliveryProofID: deliveryProofID,
		EventType:       eventType,
		EventData:       eventData,
		PerformedBy:     performedBy,
		PerformedAt:     time.Now().UTC(),
	}
}

// ============================================================
// HELPERS : CRÉATION D'ÉVÉNEMENTS
// ============================================================

// CreateShippingProofEvent crée l'événement pour soumission preuve marchand
func (p *DeliveryProof) CreateShippingProofEvent(performedBy string) *DeliveryProofEvent {
	data := map[string]interface{}{
		"proof_url":       p.ShippingProofURL,
		"tracking_number": p.ShippingTrackingNumber,
		"carrier":         p.ShippingCarrier,
		"shipping_date":   p.ShippingDate,
	}
	return NewDeliveryProofEvent(p.ID, ProofEventSubmitted, performedBy, data)
}

// CreateDeliveryProofEvent crée l'événement pour soumission preuve client
func (p *DeliveryProof) CreateDeliveryProofEvent(performedBy string) *DeliveryProofEvent {
	data := map[string]interface{}{
		"proof_url":     p.DeliveryProofURL,
		"delivery_date": p.DeliveryDate,
		"rating":        p.DeliveryRating,
	}
	return NewDeliveryProofEvent(p.ID, ProofEventSubmitted, performedBy, data)
}

// CreateStatusChangeEvent crée l'événement pour changement de statut
func (p *DeliveryProof) CreateStatusChangeEvent(
	oldStatus, newStatus EscrowStatus,
	performedBy string,
) *DeliveryProofEvent {
	data := map[string]interface{}{
		"old_status": oldStatus,
		"new_status": newStatus,
	}
	return NewDeliveryProofEvent(p.ID, ProofEventStatusChanged, performedBy, data)
}

// CreateDisputeEvent crée l'événement pour ouverture de litige
func (p *DeliveryProof) CreateDisputeEvent(performedBy string) *DeliveryProofEvent {
	data := map[string]interface{}{
		"reason": p.DisputeReason,
	}
	return NewDeliveryProofEvent(p.ID, ProofEventDisputeRaised, performedBy, data)
}

// CreateDisputeResolutionEvent crée l'événement pour résolution de litige
func (p *DeliveryProof) CreateDisputeResolutionEvent(performedBy string) *DeliveryProofEvent {
	data := map[string]interface{}{
		"resolution": p.DisputeResolution,
	}
	return NewDeliveryProofEvent(p.ID, ProofEventDisputeResolved, performedBy, data)
}

// CreateFundsReleasedEvent crée l'événement pour déblocage des fonds
func (p *DeliveryProof) CreateFundsReleasedEvent(performedBy string) *DeliveryProofEvent {
	data := map[string]interface{}{
		"released_at": time.Now().UTC(),
	}
	return NewDeliveryProofEvent(p.ID, ProofEventFundsReleased, performedBy, data)
}

// CreateFundsRefundedEvent crée l'événement pour remboursement
func (p *DeliveryProof) CreateFundsRefundedEvent(performedBy string) *DeliveryProofEvent {
	data := map[string]interface{}{
		"refunded_at": time.Now().UTC(),
	}
	return NewDeliveryProofEvent(p.ID, ProofEventFundsRefunded, performedBy, data)
}
