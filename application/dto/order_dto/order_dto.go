package orderdto

import (
	orderitemdto "Goshop/application/dto/orderItem_dto"
	"errors"
)

type OrderRequestDto struct {
	CustomerID string                              `json:"customer_id"`
	Items      []*orderitemdto.OrderItemRequestDto `json:"items"`
}

type OrderResponseDto struct {
	ID         string                               `json:"id"`
	CustomerID string                               `json:"customer_id"`
	TotalCents int64                                `json:"total_cents"`
	Status     string                               `json:"status"`
	Items      []*orderitemdto.OrderItemResponseDto `json:"items"`
}

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

	return nil
}
