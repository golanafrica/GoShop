package commissionratehandler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"Goshop/application/metrics"
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

type CommissionRateHandler struct {
	rateRepo           repository.CommissionRateRepository
	onlinePaymentSched *appscheduler.OnlinePaymentScheduler
	tontineSched       *appscheduler.TontineScheduler
}

func NewCommissionRateHandler(
	rateRepo repository.CommissionRateRepository,
	onlinePaymentSched *appscheduler.OnlinePaymentScheduler,
	tontineSched *appscheduler.TontineScheduler,
) *CommissionRateHandler {
	return &CommissionRateHandler{
		rateRepo:           rateRepo,
		onlinePaymentSched: onlinePaymentSched,
		tontineSched:       tontineSched,
	}
}

// ============================================================
// REQUEST/RESPONSE TYPES
// ============================================================

type CommissionRateRequest struct {
	ShopID             string `json:"shop_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	TransactionType    string `json:"transaction_type" example:"online_payment"`
	RateBps            int    `json:"rate_bps" example:"250"`
	MinCommissionCents int64  `json:"min_commission_cents" example:"10000"`
	MaxCommissionCents int64  `json:"max_commission_cents" example:"50000"`
	IsActive           bool   `json:"is_active" example:"true"`
}

// ============================================================
// ENDPOINTS : CONFIGURATION DES TAUX
// ============================================================

// @Summary Mettre à jour le taux de commission d'une boutique
// @Description Met à jour ou crée un nouveau taux de commission pour un type de transaction spécifique. Le taux est limité à 15% (1500 bps) conformément à la BCEAO.
// @Tags Commission Management
// @Accept json
// @Produce json
// @Param request body commissionratehandler.CommissionRateRequest true "Détails du taux de commission"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "Payload invalide ou taux hors limites"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/commission-rates [put]
func (h *CommissionRateHandler) UpdateRate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	var req CommissionRateRequest
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
	}

	if !validTypes[req.TransactionType] {
		return utils.NewAppError("INVALID_TRANSACTION_TYPE", "Invalid transaction type", http.StatusBadRequest)
	}

	// Valider le taux (max 15% = 1500 bps, BCEAO)
	if req.RateBps < 0 || req.RateBps > 1500 {
		return utils.NewAppError("INVALID_RATE", "Rate must be between 0 and 1500 bps (15%)", http.StatusBadRequest)
	}

	now := time.Now().UTC()

	// Vrai upsert : chercher d'abord, puis créer ou mettre à jour
	existing, findErr := h.rateRepo.FindByShopAndType(ctx, req.ShopID, req.TransactionType)
	if findErr != nil || existing == nil {
		// CREATE
		rate := &entity.CommissionRate{
			ID:                 uuid.New().String(),
			ShopID:             req.ShopID,
			TransactionType:    req.TransactionType,
			RateBps:            req.RateBps,
			MinCommissionCents: req.MinCommissionCents,
			MaxCommissionCents: req.MaxCommissionCents,
			IsActive:           req.IsActive,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := h.rateRepo.Create(ctx, rate); err != nil {
			duration := time.Since(start).Seconds()

			metrics.CommissionRateUpdateTotal.WithLabelValues("create", "error", req.TransactionType).Inc()
			metrics.CommissionRateOperationDuration.WithLabelValues("create").Observe(duration)
			metrics.ApplicationErrorsTotal.WithLabelValues("commission_rate_create", "commission_rate_handler").Inc()

			logger.Error().Err(err).
				Str("shop_id", req.ShopID).
				Str("transaction_type", req.TransactionType).
				Float64("duration_seconds", duration).
				Msg("Failed to create commission rate")

			return utils.NewAppError("RATE_CREATE_FAILED", "Failed to create rate", http.StatusInternalServerError)
		}

		duration := time.Since(start).Seconds()

		metrics.CommissionRateUpdateTotal.WithLabelValues("create", "success", req.TransactionType).Inc()
		metrics.CommissionRateOperationDuration.WithLabelValues("create").Observe(duration)
		metrics.CommissionRateConfigured.WithLabelValues(req.ShopID, req.TransactionType).Inc()
		metrics.CommissionRateValueBps.WithLabelValues(req.TransactionType).Observe(float64(req.RateBps))

		logger.Info().
			Str("shop_id", req.ShopID).
			Str("transaction_type", req.TransactionType).
			Int("rate_bps", req.RateBps).
			Float64("duration_seconds", duration).
			Msg("✅ Commission rate created")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Commission rate created",
			"rate":    rate,
		})
		return nil
	}

	// UPDATE
	existing.RateBps = req.RateBps
	existing.MinCommissionCents = req.MinCommissionCents
	existing.MaxCommissionCents = req.MaxCommissionCents
	existing.IsActive = req.IsActive
	existing.UpdatedAt = now

	if err := h.rateRepo.Update(ctx, existing); err != nil {
		duration := time.Since(start).Seconds()

		metrics.CommissionRateUpdateTotal.WithLabelValues("update", "error", req.TransactionType).Inc()
		metrics.CommissionRateOperationDuration.WithLabelValues("update").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("commission_rate_update", "commission_rate_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", req.ShopID).
			Str("transaction_type", req.TransactionType).
			Float64("duration_seconds", duration).
			Msg("Failed to update commission rate")

		return utils.NewAppError("RATE_UPDATE_FAILED", "Failed to update rate", http.StatusInternalServerError)
	}

	duration := time.Since(start).Seconds()

	metrics.CommissionRateUpdateTotal.WithLabelValues("update", "success", req.TransactionType).Inc()
	metrics.CommissionRateOperationDuration.WithLabelValues("update").Observe(duration)
	metrics.CommissionRateValueBps.WithLabelValues(req.TransactionType).Observe(float64(req.RateBps))

	logger.Info().
		Str("shop_id", req.ShopID).
		Str("transaction_type", req.TransactionType).
		Int("rate_bps", req.RateBps).
		Float64("duration_seconds", duration).
		Msg("✅ Commission rate updated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Commission rate updated",
		"rate":    existing,
	})
	return nil
}

// @Summary Récupérer les taux de commission d'une boutique
// @Description Retourne la liste de tous les taux de commission configurés pour une boutique donnée.
// @Tags Commission Management
// @Accept json
// @Produce json
// @Param shop_id query string true "ID de la boutique (UUID)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError "shop_id manquant"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/commission-rates [get]
func (h *CommissionRateHandler) GetRates(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	shopID := r.URL.Query().Get("shop_id")
	if shopID == "" {
		return utils.NewAppError("MISSING_SHOP_ID", "shop_id is required", http.StatusBadRequest)
	}

	rates, err := h.rateRepo.FindByShop(ctx, shopID)
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.CommissionRateGetTotal.WithLabelValues("error").Inc()
		metrics.CommissionRateOperationDuration.WithLabelValues("get").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("commission_rate_get", "commission_rate_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("Failed to fetch rates")

		return utils.NewAppError("RATES_FETCH_FAILED", "Failed to fetch rates", http.StatusInternalServerError)
	}

	metrics.CommissionRateGetTotal.WithLabelValues("success").Inc()
	metrics.CommissionRateOperationDuration.WithLabelValues("get").Observe(duration)

	logger.Info().
		Str("shop_id", shopID).
		Int("rates_count", len(rates)).
		Float64("duration_seconds", duration).
		Msg("✅ Commission rates fetched successfully")

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

// @Summary Déclencher manuellement la collecte des commissions en ligne
// @Description Lance la collecte des commissions pour les paiements en ligne en arrière-plan.
// @Tags Commission Management
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/commission-rates/trigger-online [post]
func (h *CommissionRateHandler) TriggerOnlineCollection(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	metrics.CommissionTriggerTotal.WithLabelValues("online", "triggered").Inc()
	logger.Info().Msg("🔧 Manual online payment collection triggered")

	go func() {
		bgCtx := context.Background()
		bgStart := time.Now()

		if h.onlinePaymentSched != nil {
			if err := h.onlinePaymentSched.RunCollection(bgCtx); err != nil {
				metrics.CommissionTriggerTotal.WithLabelValues("online", "error").Inc()
				metrics.ApplicationErrorsTotal.WithLabelValues("commission_trigger_online", "commission_rate_handler").Inc()

				logger.Error().Err(err).
					Float64("duration_seconds", time.Since(bgStart).Seconds()).
					Msg("❌ Manual online payment collection failed")
				return
			}

			metrics.CommissionTriggerTotal.WithLabelValues("online", "success").Inc()
			logger.Info().
				Float64("duration_seconds", time.Since(bgStart).Seconds()).
				Msg("✅ Online payment collection completed")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Online payment collection triggered in background",
	})
	return nil
}

// @Summary Déclencher manuellement la collecte des commissions Tontine
// @Description Lance la collecte des commissions pour les tontines en arrière-plan.
// @Tags Commission Management
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/admin/commission-rates/trigger-tontine [post]
func (h *CommissionRateHandler) TriggerTontineCollection(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

	metrics.CommissionTriggerTotal.WithLabelValues("tontine", "triggered").Inc()
	logger.Info().Msg("🎯 Manual tontine collection triggered")

	go func() {
		bgCtx := context.Background()
		bgStart := time.Now()

		if h.tontineSched != nil {
			if err := h.tontineSched.RunCollection(bgCtx); err != nil {
				metrics.CommissionTriggerTotal.WithLabelValues("tontine", "error").Inc()
				metrics.ApplicationErrorsTotal.WithLabelValues("commission_trigger_tontine", "commission_rate_handler").Inc()

				logger.Error().Err(err).
					Float64("duration_seconds", time.Since(bgStart).Seconds()).
					Msg("❌ Manual tontine collection failed")
				return
			}

			metrics.CommissionTriggerTotal.WithLabelValues("tontine", "success").Inc()
			logger.Info().
				Float64("duration_seconds", time.Since(bgStart).Seconds()).
				Msg("✅ Tontine collection completed")
		} else {
			metrics.CommissionTriggerTotal.WithLabelValues("tontine", "not_initialized").Inc()
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
