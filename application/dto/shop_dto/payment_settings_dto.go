package shopdto

import (
	"errors"

	"github.com/google/uuid"
)

// UpdatePaymentSettingsRequest représente la requête de mise à jour des settings
// 🛡️ MODIFICATION : Les clés API sensibles ont été supprimées.
// Tous les paiements passent par le compte YengaPay global de GoShop.
type UpdatePaymentSettingsRequest struct {
	ShopID                string   `json:"shop_id"`
	CashOnDeliveryEnabled *bool    `json:"cash_on_delivery_enabled,omitempty"`
	CashCommissionRate    *int     `json:"cash_commission_rate,omitempty"`   // en basis points (ex: 250 = 2.5%)
	OnlineCommissionRate  *int     `json:"online_commission_rate,omitempty"` // en basis points (ex: 250 = 2.5%)
	YengaPayEnabled       *bool    `json:"yenga_pay_enabled,omitempty"`
	YengaPayOperators     []string `json:"yenga_pay_operators,omitempty"` // ex: ["orange_money", "moov_money"]
}

// Validate valide la requête
func (r *UpdatePaymentSettingsRequest) Validate() error {
	if r.ShopID == "" {
		return errors.New("shop_id is required")
	}

	if _, err := uuid.Parse(r.ShopID); err != nil {
		return errors.New("invalid shop_id format")
	}

	// Valider les taux de commission (0 à 10000 basis points, soit 0% à 100%)
	if r.CashCommissionRate != nil {
		if *r.CashCommissionRate < 0 || *r.CashCommissionRate > 10000 {
			return errors.New("cash_commission_rate must be between 0 and 10000")
		}
	}

	if r.OnlineCommissionRate != nil {
		if *r.OnlineCommissionRate < 0 || *r.OnlineCommissionRate > 10000 {
			return errors.New("online_commission_rate must be between 0 and 10000")
		}
	}

	// Valider les opérateurs YengaPay autorisés
	validOperators := map[string]bool{
		"orange_money": true,
		"moov_money":   true,
		"telecel":      true,
		"coris_money":  true,
		"sank_money":   true,
		"mtn":          true,
	}

	for _, op := range r.YengaPayOperators {
		if !validOperators[op] {
			return errors.New("invalid operator: " + op)
		}
	}

	return nil
}

// PaymentSettingsResponse représente la réponse des settings
// 🛡️ MODIFICATION : Plus d'indicateurs "HasAPIKey" ou "IsUsingGlobal",
// car c'est désormais le modèle par défaut et unique.
type PaymentSettingsResponse struct {
	ShopID                string   `json:"shop_id"`
	CashOnDeliveryEnabled bool     `json:"cash_on_delivery_enabled"`
	CashCommissionRate    int      `json:"cash_commission_rate"`
	OnlineCommissionRate  int      `json:"online_commission_rate"`
	YengaPayEnabled       bool     `json:"yenga_pay_enabled"`
	YengaPayOperators     []string `json:"yenga_pay_operators"`
}
