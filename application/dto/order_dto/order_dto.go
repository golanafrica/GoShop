package orderdto

import (
	"errors"

	orderitemdto "Goshop/application/dto/orderItem_dto"
)

// OrderRequestDto représente la requête de création de commande
type OrderRequestDto struct {
	CustomerID string                              `json:"customer_id"`
	Items      []*orderitemdto.OrderItemRequestDto `json:"items"`
	// 🆕 Méthode de paiement (optionnel, défaut: mobile_money)
	PaymentMethod string `json:"payment_method,omitempty"`
}

// OrderResponseDto représente la réponse détaillée d'une commande
type OrderResponseDto struct {
	ID         string                               `json:"id"`
	CustomerID string                               `json:"customer_id"`
	TotalCents int64                                `json:"total_cents"`
	Status     string                               `json:"status"`
	Items      []*orderitemdto.OrderItemResponseDto `json:"items,omitempty"`

	// 🆕 Champs cash à la livraison
	PaymentMethod       string `json:"payment_method,omitempty"`
	AcceptedAt          string `json:"accepted_at,omitempty"`
	RejectedAt          string `json:"rejected_at,omitempty"`
	DeliveredAt         string `json:"delivered_at,omitempty"`
	CancelledAt         string `json:"cancelled_at,omitempty"`
	DeliveryNotes       string `json:"delivery_notes,omitempty"`
	AmountReceivedCents *int64 `json:"amount_received_cents,omitempty"`
	ReservedUntil       string `json:"reserved_until,omitempty"`

	// Timestamps
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Validate valide la requête
func (o *OrderRequestDto) Validate() error {
	if o.CustomerID == "" {
		return errors.New("customer_id is required")
	}

	if len(o.Items) == 0 {
		return errors.New("order must contain at least one item")
	}

	for _, it := range o.Items {
		if it.ProductID == "" {
			return errors.New("product_id is required")
		}
		if it.Quantity <= 0 {
			return errors.New("quantity must be greater than 0")
		}
	}

	// 🆕 Validation du payment_method
	if o.PaymentMethod != "" &&
		o.PaymentMethod != "mobile_money" &&
		o.PaymentMethod != "cash_on_delivery" {
		return errors.New("invalid payment_method: must be 'mobile_money' or 'cash_on_delivery'")
	}

	return nil
}
