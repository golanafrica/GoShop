package dto

// PublicProductResponse représente un produit visible publiquement avec les infos de sa boutique
type PublicProductResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
	ShopID      string `json:"shop_id"`
	ShopName    string `json:"shop_name"`
	ShopSlug    string `json:"shop_slug"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
