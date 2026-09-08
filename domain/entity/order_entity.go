package entity

import (
	"errors"
	"time"
)

// OrderStatus représente le statut d'une commande
type OrderStatus string

const (
	// Statuts existants
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusCancelled OrderStatus = "cancelled"

	// 🆕 Statuts pour cash à la livraison et litiges
	OrderStatusPendingConfirmation OrderStatus = "pending_confirmation"
	OrderStatusConfirmed           OrderStatus = "confirmed"
	OrderStatusRejected            OrderStatus = "rejected"
	OrderStatusExpired             OrderStatus = "expired"
	OrderStatusOutForDelivery      OrderStatus = "out_for_delivery"
	OrderStatusDelivered           OrderStatus = "delivered"
	OrderStatusDisputed            OrderStatus = "disputed" // 🆕 Ajouté pour la gestion des litiges
)

// PaymentMethod représente la méthode de paiement
type PaymentMethod string

const (
	PaymentMethodMobileMoney    PaymentMethod = "mobile_money"
	PaymentMethodCashOnDelivery PaymentMethod = "cash_on_delivery"
)

// Order représente une commande dans le système
type Order struct {
	ID         string       `json:"id"`
	ShopID     string       `json:"shop_id"` // 🆕 AJOUT CRITIQUE : Identifiant de la boutique pour le multi-tenant et les litiges
	CustomerID string       `json:"customer_id"`
	TotalCents int64        `json:"total_cents"`
	Status     string       `json:"status"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	Items      []*OrderItem `json:"items,omitempty"`

	// 🆕 Champs cash à la livraison et suivi
	PaymentMethod       string     `json:"payment_method"`
	AcceptedAt          *time.Time `json:"accepted_at,omitempty"`
	RejectedAt          *time.Time `json:"rejected_at,omitempty"`
	DeliveredAt         *time.Time `json:"delivered_at,omitempty"`
	CancelledAt         *time.Time `json:"cancelled_at,omitempty"`
	DeliveryNotes       *string    `json:"delivery_notes,omitempty"`
	AmountReceivedCents *int64     `json:"amount_received_cents,omitempty"`
	ReservedUntil       *time.Time `json:"reserved_until,omitempty"`
	// 🆕 v5.1.0 : Pour le calcul dynamique du délai de libération des tranches
	DeliveryZoneID              *string `json:"delivery_zone_id,omitempty"`
	InstallmentReleaseDelayDays int     `json:"installment_release_delay_days"`
}

// IsCashOnDelivery retourne true si la commande est en cash à la livraison
func (o *Order) IsCashOnDelivery() bool {
	return o.PaymentMethod == string(PaymentMethodCashOnDelivery)
}

// IsExpired vérifie si la commande a expiré (lazy expiration)
func (o *Order) IsExpired() bool {
	if o.Status != string(OrderStatusPendingConfirmation) {
		return false
	}
	if o.ReservedUntil == nil {
		return false
	}
	return time.Now().UTC().After(*o.ReservedUntil)
}

// CanTransitionTo vérifie si la transition vers le statut cible est autorisée
func (o *Order) CanTransitionTo(target OrderStatus) bool {
	// Vérifier d'abord l'expiration lazy
	if o.IsExpired() && target != OrderStatusExpired {
		return false
	}

	transitions := map[OrderStatus][]OrderStatus{
		OrderStatusPending: {
			OrderStatusCancelled,
			OrderStatusConfirmed,
		},
		OrderStatusPendingConfirmation: {
			OrderStatusConfirmed,
			OrderStatusRejected,
			OrderStatusExpired,
			OrderStatusCancelled,
		},
		OrderStatusConfirmed: {
			OrderStatusOutForDelivery,
			OrderStatusCancelled,
			OrderStatusDisputed, // 🆕 Permet d'ouvrir un litige sur une commande confirmée
		},
		OrderStatusOutForDelivery: {
			OrderStatusDelivered,
			OrderStatusCancelled,
			OrderStatusDisputed, // 🆕 Permet d'ouvrir un litige sur une commande en livraison
		},
		// Statuts terminaux - pas de transition possible
		OrderStatusDelivered: {},
		OrderStatusRejected:  {},
		OrderStatusExpired:   {},
		OrderStatusCancelled: {},
		OrderStatusDisputed:  {}, // Un litige doit être résolu avant de changer de statut
	}

	allowed, exists := transitions[OrderStatus(o.Status)]
	if !exists {
		return false
	}

	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// CalculateCashCommission calcule la commission GoShop sur un paiement cash
// commissionRate est en basis points (250 = 2.50%)
func (o *Order) CalculateCashCommission(commissionRate int) (feesCents, netAmountCents int64) {
	if commissionRate < 0 || commissionRate > 10000 {
		return 0, o.TotalCents
	}

	feesCents = (o.TotalCents * int64(commissionRate)) / 10000
	netAmountCents = o.TotalCents - feesCents
	return feesCents, netAmountCents
}

// MarkAccepted marque la commande comme acceptée par le marchand
func (o *Order) MarkAccepted() error {
	if !o.CanTransitionTo(OrderStatusConfirmed) {
		return errors.New("invalid status transition from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusConfirmed)
	o.AcceptedAt = &now
	o.UpdatedAt = now
	return nil
}

// MarkRejected marque la commande comme rejetée par le marchand
func (o *Order) MarkRejected() error {
	if !o.CanTransitionTo(OrderStatusRejected) {
		return errors.New("invalid status transition from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusRejected)
	o.RejectedAt = &now
	o.UpdatedAt = now
	return nil
}

// MarkExpired marque la commande comme expirée
func (o *Order) MarkExpired() error {
	if !o.CanTransitionTo(OrderStatusExpired) {
		return errors.New("invalid status transition from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusExpired)
	o.UpdatedAt = now
	return nil
}

// MarkOutForDelivery marque la commande comme en cours de livraison
func (o *Order) MarkOutForDelivery() error {
	if !o.CanTransitionTo(OrderStatusOutForDelivery) {
		return errors.New("invalid status transition from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusOutForDelivery)
	o.UpdatedAt = now
	return nil
}

// MarkDelivered marque la commande comme livrée et payée
func (o *Order) MarkDelivered(amountReceived int64, notes string) error {
	if !o.CanTransitionTo(OrderStatusDelivered) {
		return errors.New("invalid status transition from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusDelivered)
	o.DeliveredAt = &now
	o.AmountReceivedCents = &amountReceived
	if notes != "" {
		o.DeliveryNotes = &notes
	}
	o.UpdatedAt = now
	return nil
}

// MarkDisputed marque la commande comme étant en litige
func (o *Order) MarkDisputed() error {
	if !o.CanTransitionTo(OrderStatusDisputed) {
		return errors.New("invalid status transition to disputed from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusDisputed)
	o.UpdatedAt = now
	return nil
}

// MarkCancelled marque la commande comme annulée
func (o *Order) MarkCancelled() error {
	if !o.CanTransitionTo(OrderStatusCancelled) {
		return errors.New("invalid status transition from " + o.Status)
	}
	now := time.Now().UTC()
	o.Status = string(OrderStatusCancelled)
	o.CancelledAt = &now
	o.UpdatedAt = now
	return nil
}
