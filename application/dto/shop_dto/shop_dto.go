package shopdto

import (
	"errors"
	"regexp"
	"strings"
)

// CreateShopRequest représente la requête de création d'une boutique
type CreateShopRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	CustomDomain string `json:"custom_domain,omitempty"`
}

// UpdateShopRequest représente la requête de mise à jour d'une boutique
type UpdateShopRequest struct {
	Name         *string `json:"name,omitempty"`
	CustomDomain *string `json:"custom_domain,omitempty"`
	Plan         *string `json:"plan,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
}

// ShopResponse représente la réponse d'une boutique
type ShopResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	CustomDomain string `json:"custom_domain,omitempty"`
	OwnerID      string `json:"owner_id"`
	Plan         string `json:"plan"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// slugRegex valide le format d'un slug (alphanumérique + tirets)
var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Validate valide la requête de création
func (r *CreateShopRequest) Validate() error {
	// Validation du nom
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("shop name is required")
	}
	if len(r.Name) < 2 {
		return errors.New("shop name must be at least 2 characters")
	}
	if len(r.Name) > 255 {
		return errors.New("shop name must be at most 255 characters")
	}

	// Validation du slug
	r.Slug = strings.TrimSpace(strings.ToLower(r.Slug))
	if r.Slug == "" {
		return errors.New("shop slug is required")
	}
	if len(r.Slug) < 2 {
		return errors.New("shop slug must be at least 2 characters")
	}
	if len(r.Slug) > 100 {
		return errors.New("shop slug must be at most 100 characters")
	}
	if !slugRegex.MatchString(r.Slug) {
		return errors.New("shop slug must contain only lowercase letters, numbers, and hyphens")
	}

	// Slugs réservés
	reservedSlugs := []string{"admin", "api", "www", "mail", "shop", "boutique", "store"}
	for _, reserved := range reservedSlugs {
		if r.Slug == reserved {
			return errors.New("this slug is reserved and cannot be used")
		}
	}

	// Validation du custom_domain (optionnel)
	if r.CustomDomain != "" {
		r.CustomDomain = strings.TrimSpace(r.CustomDomain)
		if len(r.CustomDomain) > 255 {
			return errors.New("custom domain must be at most 255 characters")
		}
	}

	return nil
}

// Validate valide la requête de mise à jour
func (r *UpdateShopRequest) Validate() error {
	hasUpdate := false

	if r.Name != nil {
		hasUpdate = true
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return errors.New("shop name cannot be empty")
		}
		if len(name) < 2 {
			return errors.New("shop name must be at least 2 characters")
		}
		if len(name) > 255 {
			return errors.New("shop name must be at most 255 characters")
		}
	}

	if r.CustomDomain != nil {
		hasUpdate = true
		domain := strings.TrimSpace(*r.CustomDomain)
		if len(domain) > 255 {
			return errors.New("custom domain must be at most 255 characters")
		}
	}

	if r.Plan != nil {
		hasUpdate = true
		validPlans := []string{"free", "pro", "business"}
		plan := strings.ToLower(strings.TrimSpace(*r.Plan))
		isValid := false
		for _, p := range validPlans {
			if plan == p {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.New("invalid plan. Must be one of: free, pro, business")
		}
	}

	if r.IsActive != nil {
		hasUpdate = true
	}

	if !hasUpdate {
		return errors.New("no fields provided for update")
	}

	return nil
}
