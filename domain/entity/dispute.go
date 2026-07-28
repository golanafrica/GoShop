package entity

import (
	"time"

	"github.com/google/uuid"
)

// DisputeStatus représente le statut d'un litige
type DisputeStatus string

const (
	DisputeStatusPending          DisputeStatus = "pending"
	DisputeStatusUnderReview      DisputeStatus = "under_review"
	DisputeStatusResolvedMerchant DisputeStatus = "resolved_merchant"
	DisputeStatusResolvedCustomer DisputeStatus = "resolved_customer"
	DisputeStatusCancelled        DisputeStatus = "cancelled"
)

// InitiatorRole définit qui a ouvert le litige
type InitiatorRole string

const (
	RoleCustomer InitiatorRole = "customer"
	RoleMerchant InitiatorRole = "merchant"
	RoleAdmin    InitiatorRole = "admin"
)

// Dispute représente un litige sur une commande/paiement
type Dispute struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	OrderID   uuid.UUID  `json:"order_id" db:"order_id"`
	ShopID    uuid.UUID  `json:"shop_id" db:"shop_id"`
	PaymentID *uuid.UUID `json:"payment_id,omitempty" db:"payment_id"` // Optionnel, mais utile pour le remboursement

	InitiatorID   uuid.UUID     `json:"initiator_id" db:"initiator_id"`
	InitiatorRole InitiatorRole `json:"initiator_role" db:"initiator_role"`

	Reason          string        `json:"reason" db:"reason"`
	Status          DisputeStatus `json:"status" db:"status"`
	ResolutionNotes *string       `json:"resolution_notes,omitempty" db:"resolution_notes"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CanTransitionTo vérifie si le litige peut passer à un nouveau statut
func (d *Dispute) CanTransitionTo(newStatus DisputeStatus) bool {
	switch d.Status {
	case DisputeStatusPending:
		return newStatus == DisputeStatusUnderReview || newStatus == DisputeStatusCancelled
	case DisputeStatusUnderReview:
		return newStatus == DisputeStatusResolvedMerchant ||
			newStatus == DisputeStatusResolvedCustomer ||
			newStatus == DisputeStatusCancelled
	case DisputeStatusResolvedMerchant, DisputeStatusResolvedCustomer, DisputeStatusCancelled:
		// Un litige résolu ou annulé est terminal, on ne peut plus le modifier
		return false
	default:
		return false
	}

}
