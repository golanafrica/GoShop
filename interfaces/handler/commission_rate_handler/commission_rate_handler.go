package commissionratehandler

import (
	"context"
	"encoding/json"
	"net/http"

	appscheduler "Goshop/application/scheduler"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ============================================================
// COMMISSION RATE HANDLER
// ============================================================

// CommissionRateHandler gère les endpoints de configuration des taux
// et les triggers manuels pour les schedulers
type CommissionRateHandler struct {
	rateRepo           repository.CommissionRateRepository
	onlinePaymentSched *appscheduler.OnlinePaymentScheduler
	tontineSched       *appscheduler.TontineScheduler
	creditSched        *appscheduler.CreditScheduler
}

// NewCommissionRateHandler crée une nouvelle instance
func NewCommissionRateHandler(
	rateRepo repository.CommissionRateRepository,
	onlinePaymentSched *appscheduler.OnlinePaymentScheduler,
	tontineSched *appscheduler.TontineScheduler,
	creditSched *appscheduler.CreditScheduler,
) *CommissionRateHandler {
	return &CommissionRateHandler{
		rateRepo:           rateRepo,
		onlinePaymentSched: onlinePaymentSched,
		tontineSched:       tontineSched,
		creditSched:        creditSched,
	}
}

// ============================================================
// ENDPOINTS : CONFIGURATION DES TAUX
// ============================================================

// UpdateRate met à jour le taux de commission pour une boutique
// PUT /api/admin/commission-rates
func (h *CommissionRateHandler) UpdateRate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	var req struct {
		ShopID             string `json:"shop_id"`
		TransactionType    string `json:"transaction_type"`
		RateBps            int    `json:"rate_bps"`
		MinCommissionCents int64  `json:"min_commission_cents"`
		MaxCommissionCents int64  `json:"max_commission_cents"`
		IsActive           bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid request body", http.StatusBadRequest)
	}

	// Valider le type de transaction
	validTypes := map[string]bool{
		entity.TransactionTypeOnlinePayment: true,
		entity.TransactionTypeCOD:           true,
		"tontine_commercial":                true,
		"tontine_corporate":                 true,
		"tontine_family":                    true,
		entity.TransactionTypeCredit:        true,
	}

	if !validTypes[req.TransactionType] {
		return utils.NewAppError("INVALID_TRANSACTION_TYPE", "Invalid transaction type", http.StatusBadRequest)
	}

	// Valider le taux (max 15% = 1500 bps, BCEAO)
	if req.RateBps < 0 || req.RateBps > 1500 {
		return utils.NewAppError("INVALID_RATE", "Rate must be between 0 and 1500 bps (15%)", http.StatusBadRequest)
	}

	// Upsert
	rate := &entity.CommissionRate{
		ID:                 uuid.New().String(),
		ShopID:             req.ShopID,
		TransactionType:    req.TransactionType,
		RateBps:            req.RateBps,
		MinCommissionCents: req.MinCommissionCents,
		MaxCommissionCents: req.MaxCommissionCents,
		IsActive:           req.IsActive,
	}

	err := h.rateRepo.Update(ctx, rate)
	if err != nil {
		if err := h.rateRepo.Create(ctx, rate); err != nil {
			logger.Error().Err(err).Msg("Failed to upsert rate")
			return utils.NewAppError("RATE_UPSERT_FAILED", "Failed to update rate", http.StatusInternalServerError)
		}
	}

	logger.Info().
		Str("shop_id", req.ShopID).
		Str("transaction_type", req.TransactionType).
		Int("rate_bps", req.RateBps).
		Msg("Commission rate updated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Commission rate updated",
		"rate":    rate,
	})
	return nil
}

// GetRates récupère tous les taux d'une boutique
// GET /api/admin/commission-rates?shop_id=xxx
func (h *CommissionRateHandler) GetRates(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	shopID := r.URL.Query().Get("shop_id")
	if shopID == "" {
		// ✅ FIX : Utiliser utils.NewAppError pour que le middleware le reconnaisse et renvoie un 400
		return utils.NewAppError("MISSING_SHOP_ID", "shop_id is required", http.StatusBadRequest)
	}

	rates, err := h.rateRepo.FindByShop(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch rates")
		return utils.NewAppError("RATES_FETCH_FAILED", "Failed to fetch rates", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(rates),
		"rates":   rates,
	})
	return nil
}

// ============================================================
// ENDPOINTS : TRIGGERS MANUELS
// ============================================================

// TriggerOnlineCollection déclenche manuellement la collecte en ligne
// POST /api/admin/commission-rates/trigger-online
func (h *CommissionRateHandler) TriggerOnlineCollection(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())
	logger.Info().Msg("🔧 Manual online payment collection triggered")

	go func() {
		bgCtx := context.Background()
		if h.onlinePaymentSched != nil {
			if err := h.onlinePaymentSched.RunCollection(bgCtx); err != nil {
				logger.Error().Err(err).Msg("❌ Manual online payment collection failed")
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Online payment collection triggered in background",
	})
	return nil
}

// TriggerTontineCollection déclenche manuellement la collecte tontine
// POST /api/admin/commission-rates/trigger-tontine
func (h *CommissionRateHandler) TriggerTontineCollection(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())
	logger.Info().Msg("🎯 Manual tontine collection triggered")

	go func() {
		bgCtx := context.Background()
		if h.tontineSched != nil {
			if err := h.tontineSched.RunCollection(bgCtx); err != nil {
				logger.Error().Err(err).Msg("❌ Manual tontine collection failed")
			}
		} else {
			logger.Warn().Msg("⚠️ Tontine scheduler not initialized")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Tontine collection triggered in background",
	})
	return nil
}

// TriggerCreditCollection déclenche manuellement la collecte credit
// POST /api/admin/commission-rates/trigger-credit
func (h *CommissionRateHandler) TriggerCreditCollection(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())
	logger.Info().Msg("💰 Manual credit collection triggered")

	go func() {
		bgCtx := context.Background()
		if h.creditSched != nil {
			if err := h.creditSched.RunCollection(bgCtx); err != nil {
				logger.Error().Err(err).Msg("❌ Manual credit collection failed")
			}
		} else {
			logger.Warn().Msg("⚠️ Credit scheduler not initialized")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Credit collection triggered in background",
	})
	return nil
}
