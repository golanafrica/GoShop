package entity

import "time"

type Order struct {
	ID         string       `json:"id"`
	CustomerID string       `json:"customer_id"`
	TotalCents int64        `json:"total_cents"`
	Status     string       `json:"status"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	Items      []*OrderItem `json:"items,omitempty"`
}
