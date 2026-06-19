package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ShopPlan représente le plan d'abonnement d'une boutique
type ShopPlan string

const (
	ShopPlanFree     ShopPlan = "free"
	ShopPlanPro      ShopPlan = "pro"
	ShopPlanBusiness ShopPlan = "business"
)

// Shop représente une boutique dans le système multi-tenant
type Shop struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	CustomDomain *string
	OwnerID      string // ⚠️ string pour être compatible avec User.ID
	LogoURL      *string
	Theme        map[string]interface{}
	Plan         ShopPlan
	DBSchema     *string // Pour migration Option A future
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewShop crée une nouvelle boutique
func NewShop(name, slug string, ownerID string) (*Shop, error) {
	if name == "" {
		return nil, errors.New("shop name is required")
	}
	if slug == "" {
		return nil, errors.New("shop slug is required")
	}

	return &Shop{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		OwnerID:   ownerID,
		Theme:     make(map[string]interface{}),
		Plan:      ShopPlanFree,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// SetCustomDomain définit un domaine personnalisé
func (s *Shop) SetCustomDomain(domain string) error {
	if domain == "" {
		s.CustomDomain = nil
		return nil
	}
	s.CustomDomain = &domain
	s.UpdatedAt = time.Now()
	return nil
}

// UpdateTheme met à jour le thème de la boutique
func (s *Shop) UpdateTheme(theme map[string]interface{}) {
	s.Theme = theme
	s.UpdatedAt = time.Now()
}

// Deactivate désactive la boutique
func (s *Shop) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

// Activate active la boutique
func (s *Shop) Activate() {
	s.IsActive = true
	s.UpdatedAt = time.Now()
}

// IsValidPlan vérifie si le plan est valide
func (s *Shop) IsValidPlan() bool {
	switch s.Plan {
	case ShopPlanFree, ShopPlanPro, ShopPlanBusiness:
		return true
	default:
		return false
	}
}
