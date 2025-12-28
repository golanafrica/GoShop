package orderitemdto

import "errors"

type OrderItemRequestDto struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type OrderItemResponseDto struct {
	ID            string `json:"id"`
	ProductID     string `json:"product_id"`
	Quantity      int    `json:"quantity"`
	PriceCents    int64  `json:"price_cents"`
	SubTotalCents int64  `json:"sub_total_cents"`
}

func (i *OrderItemRequestDto) Validate() error {
	if i.ProductID == "" {
		return errors.New("product_id is required")
	}
	if i.Quantity <= 0 {
		return errors.New("quantity must be greater than 0")
	}
	return nil
}
