package mapper

import (
	"time"

	orderitemdto "Goshop/application/dto/orderItem_dto"
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/domain/entity"
)

// ToOrderEntity convertit un DTO de requête en entité Order
func ToOrderEntity(req *orderdto.OrderRequestDto) *entity.Order {
	items := make([]*entity.OrderItem, len(req.Items))

	for i, it := range req.Items {
		items[i] = &entity.OrderItem{
			ProductID: it.ProductID,
			Quantity:  int(it.Quantity),
		}
	}

	// 🆕 Gestion du payment_method (défaut: mobile_money)
	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = string(entity.PaymentMethodMobileMoney)
	}

	return &entity.Order{
		CustomerID:    req.CustomerID,
		Items:         items,
		PaymentMethod: paymentMethod,
	}
}

// ToOrderResponse convertit une entité Order en DTO de réponse
func ToOrderResponse(order *entity.Order) *orderdto.OrderResponseDto {
	items := make([]*orderitemdto.OrderItemResponseDto, len(order.Items))

	for i, it := range order.Items {
		items[i] = &orderitemdto.OrderItemResponseDto{
			ID:            it.ID,
			ProductID:     it.ProductID,
			Quantity:      int(it.Quantity),
			PriceCents:    it.PriceCents,
			SubTotalCents: int64(it.SubTotal_Cents),
		}
	}

	resp := &orderdto.OrderResponseDto{
		ID:            order.ID,
		CustomerID:    order.CustomerID,
		TotalCents:    order.TotalCents,
		Status:        order.Status,
		Items:         items,
		PaymentMethod: order.PaymentMethod,
		CreatedAt:     formatTime(order.CreatedAt),
		UpdatedAt:     formatTime(order.UpdatedAt),
	}

	// 🆕 Mapping des champs cash (pointeurs)
	if order.AcceptedAt != nil {
		s := formatTime(*order.AcceptedAt)
		resp.AcceptedAt = s
	}
	if order.RejectedAt != nil {
		s := formatTime(*order.RejectedAt)
		resp.RejectedAt = s
	}
	if order.DeliveredAt != nil {
		s := formatTime(*order.DeliveredAt)
		resp.DeliveredAt = s
	}
	if order.CancelledAt != nil {
		s := formatTime(*order.CancelledAt)
		resp.CancelledAt = s
	}
	if order.DeliveryNotes != nil {
		resp.DeliveryNotes = *order.DeliveryNotes
	}
	if order.AmountReceivedCents != nil {
		resp.AmountReceivedCents = order.AmountReceivedCents
	}
	if order.ReservedUntil != nil {
		s := formatTime(*order.ReservedUntil)
		resp.ReservedUntil = s
	}

	return resp
}

// formatTime formate un time.Time en string ISO ou retourne "" si zero
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
