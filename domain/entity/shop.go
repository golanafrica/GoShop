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

// YengaPayShopSettings représente la configuration Yenga Pay d'une boutique
type YengaPayShopSettings struct {
	Enabled        bool     `json:"enabled"`
	APIKey         string   `json:"api_key,omitempty"`         // Déchiffré
	OrganizationID string   `json:"organization_id,omitempty"` // Déchiffré
	ProjectID      string   `json:"project_id,omitempty"`      // Déchiffré
	WebhookSecret  string   `json:"webhook_secret,omitempty"`  // Déchiffré
	Operators      []string `json:"operators"`
	Env            string   `json:"env"` // "test" ou "prod"
}

// ShopPaymentSettings représente tous les settings de paiement d'une boutique
type ShopPaymentSettings struct {
	ShopID      uuid.UUID            `json:"shop_id"`
	OrangeMoney bool                 `json:"orange_money_enabled"`
	MoovMoney   bool                 `json:"moov_money_enabled"`
	Wave        bool                 `json:"wave_enabled"`
	YengaPay    YengaPayShopSettings `json:"yenga_pay"`
}

// IsYengaPayEnabled vérifie si Yenga Pay est activé pour cette boutique
func (s *ShopPaymentSettings) IsYengaPayEnabled() bool {
	return s.YengaPay.Enabled
}

// IsOperatorEnabled vérifie si un opérateur Yenga Pay est activé
func (s *YengaPayShopSettings) IsOperatorEnabled(operator string) bool {
	for _, op := range s.Operators {
		if op == operator {
			return true
		}
	}
	return false
}
