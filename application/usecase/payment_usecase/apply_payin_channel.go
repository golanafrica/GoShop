package paymentusecase

import (
	"strings"

	"Goshop/domain/entity"
	paymentinfra "Goshop/infrastructure/payment"

	"github.com/rs/zerolog"
)

// paymentNeedsChannelEnrichment : true si le MSISDN réel manque encore (seed ou vide).
func paymentNeedsChannelEnrichment(p *entity.Payment) bool {
	if p == nil {
		return false
	}
	if p.Metadata != nil {
		if n, ok := p.Metadata["customer_number"].(string); ok && strings.TrimSpace(n) != "" && !isSeedPhone(n) {
			return false
		}
	}
	if p.CustomerPhone == nil || strings.TrimSpace(*p.CustomerPhone) == "" {
		return true
	}
	return isSeedPhone(*p.CustomerPhone)
}

// isSeedPhone détecte les numéros placeholder E2E (ne pas utiliser pour un refund).
func isSeedPhone(phone string) bool {
	d := digitsOnlyPhone(phone)
	if d == "" {
		return true
	}
	seeds := []string{
		"70000000",
		"70123456",
		"70707070",
		"78787878",
		"76658060",
	}
	for _, s := range seeds {
		if strings.HasSuffix(d, s) {
			return true
		}
	}
	return false
}

func digitsOnlyPhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ApplyPayInChannelFromStatus écrit payment_source + customer_number + CustomerPhone.
// Retourne true si le payment a été modifié.
func ApplyPayInChannelFromStatus(p *entity.Payment, status *paymentinfra.PaymentStatus, logger *zerolog.Logger) bool {
	if status == nil || status.Metadata == nil || p == nil {
		return false
	}
	return ApplyPayInChannelFromMeta(p, status.Metadata, logger)
}

// ApplyPayInChannelFromMeta (webhook + CheckStatus).
func ApplyPayInChannelFromMeta(p *entity.Payment, meta map[string]interface{}, logger *zerolog.Logger) bool {
	if p == nil || meta == nil {
		return false
	}
	changed := false
	if p.Metadata == nil {
		p.Metadata = make(map[string]interface{})
	}

	if src := metaString(meta, "payment_source", "paymentSource"); src != "" {
		if p.Metadata["payment_source"] != src {
			p.Metadata["payment_source"] = src
			delete(p.Metadata, "seed_fallback")
			changed = true
		}
	}
	if op := metaString(meta, "operator", "cashout_method", "cashoutMethod"); op != "" {
		if p.Metadata["operator"] != op {
			p.Metadata["operator"] = op
			changed = true
		}
	}

	num := metaString(meta, "customer_number", "customerNumber", "customer_phone", "customerPhone")
	if num != "" && !isSeedPhone(num) {
		if p.Metadata["customer_number"] != num {
			p.Metadata["customer_number"] = num
			changed = true
		}
		ph := normalizePhoneBF(num)
		if p.CustomerPhone == nil || *p.CustomerPhone != ph {
			p.CustomerPhone = &ph
			changed = true
		}
	}

	if changed && logger != nil {
		phone := ""
		if p.CustomerPhone != nil {
			phone = *p.CustomerPhone
		}
		logger.Info().
			Interface("payment_source", p.Metadata["payment_source"]).
			Interface("customer_number", p.Metadata["customer_number"]).
			Str("customer_phone", phone).
			Msg("Pay-in channel applied")
	}
	return changed
}

func metaString(meta map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := meta[k]; ok {
			if s, ok := v.(string); ok {
				if t := strings.TrimSpace(s); t != "" {
					return t
				}
			}
		}
	}
	return ""
}

func normalizePhoneBF(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "00") {
		phone = "+" + phone[2:]
	}
	d := digitsOnlyPhone(phone)
	if strings.HasPrefix(phone, "+") {
		return "+" + d
	}
	if len(d) == 8 {
		return "+226" + d
	}
	if strings.HasPrefix(d, "226") {
		return "+" + d
	}
	return "+" + d
}
