package entity

type OrderItem struct {
	ID             string `json:"id"`
	OrderID        string `json:"order_id"`
	ProductID      string `json:"product_id"`
	Quantity       int    `json:"quantity"`
	PriceCents     int64  `json:"price_cents"`
	SubTotal_Cents int64  `json:"sub_total_cents"`
}
