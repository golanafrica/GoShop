package shopdto

import (
	"errors"

	"github.com/google/uuid"
)

// UpdatePaymentSettingsRequest représente la requête de mise à jour des settings
type UpdatePaymentSettingsRequest struct {
	ShopID             string               `json:"shop_id"`
	OrangeMoneyEnabled *bool                `json:"orange_money_enabled,omitempty"`
	MoovMoneyEnabled   *bool                `json:"moov_money_enabled,omitempty"`
	WaveEnabled        *bool                `json:"wave_enabled,omitempty"`
	YengaPay           *YengaPaySettingsDTO `json:"yenga_pay,omitempty"`
}

// YengaPaySettingsDTO représente les settings Yenga Pay
type YengaPaySettingsDTO struct {
	Enabled        *bool    `json:"enabled,omitempty"`
	APIKey         *string  `json:"api_key,omitempty"`
	OrganizationID *string  `json:"organization_id,omitempty"`
	ProjectID      *string  `json:"project_id,omitempty"`
	WebhookSecret  *string  `json:"webhook_secret,omitempty"`
	Operators      []string `json:"operators,omitempty"`
	Env            *string  `json:"env,omitempty"`
}

// Validate valide la requête
func (r *UpdatePaymentSettingsRequest) Validate() error {
	if r.ShopID == "" {
		return errors.New("shop_id is required")
	}

	if _, err := uuid.Parse(r.ShopID); err != nil {
		return errors.New("invalid shop_id format")
	}

	if r.YengaPay != nil {
		// Valider les opérateurs
		validOperators := map[string]bool{
			"orange_money": true,
			"moov_money":   true,
			"telecel":      true,
			"coris_money":  true,
			"sank_money":   true,
			"mtn":          true,
		}

		for _, op := range r.YengaPay.Operators {
			if !validOperators[op] {
				return errors.New("invalid operator: " + op)
			}
		}

		// Valider l'environnement
		if r.YengaPay.Env != nil {
			if *r.YengaPay.Env != "test" && *r.YengaPay.Env != "prod" {
				return errors.New("env must be 'test' or 'prod'")
			}
		}
	}

	return nil
}

// PaymentSettingsResponse représente la réponse des settings
type PaymentSettingsResponse struct {
	ShopID             string              `json:"shop_id"`
	OrangeMoneyEnabled bool                `json:"orange_money_enabled"`
	MoovMoneyEnabled   bool                `json:"moov_money_enabled"`
	WaveEnabled        bool                `json:"wave_enabled"`
	YengaPay           YengaPayResponseDTO `json:"yenga_pay"`
}

// YengaPayResponseDTO représente la réponse Yenga Pay (sans les clés sensibles)
type YengaPayResponseDTO struct {
	Enabled       bool     `json:"enabled"`
	HasAPIKey     bool     `json:"has_api_key"` // true si configuré
	HasOrgID      bool     `json:"has_organization_id"`
	HasProjectID  bool     `json:"has_project_id"`
	HasWebhook    bool     `json:"has_webhook_secret"`
	Operators     []string `json:"operators"`
	Env           string   `json:"env"`
	IsUsingGlobal bool     `json:"is_using_global"` // true si fallback sur config globale
}
