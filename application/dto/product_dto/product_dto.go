package dto

import "errors"

type CreateProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
}

type ProductResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type UpdateProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
}

// ListProductsRequest contient les paramètres de recherche et de filtrage venant du handler
type ListProductsRequest struct {
	Search        string `json:"search"`
	MinPriceCents int64  `json:"min_price_cents"`
	MaxPriceCents int64  `json:"max_price_cents"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

func (p *CreateProductRequest) Validate() error {
	if p.Name == "" {
		return errors.New("product name is required")
	}
	if p.PriceCents <= 0 {
		return errors.New("price must be greater than 0")
	}
	if p.Stock < 0 {
		return errors.New("stock cannot be negative")
	}
	return nil
}

func (p *UpdateProductRequest) Validate() error {
	if p.Name == "" {
		return errors.New("product name is required")
	}
	if p.PriceCents <= 0 {
		return errors.New("price must be greater than 0")
	}
	if p.Stock < 0 {
		return errors.New("stock cannot be negative")
	}
	return nil
}
