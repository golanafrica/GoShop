package orderdto

// application/dto/order_dto/order_filter_dto.go

// OrderFilter represents query filters for listing orders.
// Use pointers to distinguish between "not set" and "zero value".
type OrderFilter struct {
	Status     *string `json:"status,omitempty"`
	CustomerID *string `json:"customer_id,omitempty"`
}
