package merchanthandler

import (
	"net/http"
	"time"

	"Goshop/application/metrics"
	merchantusecase "Goshop/application/usecase/merchant_usecase"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type MerchantOverviewHandler struct {
	getOverviewUC *merchantusecase.GetMerchantOverviewUsecase
}

func NewMerchantOverviewHandler(getOverviewUC *merchantusecase.GetMerchantOverviewUsecase) *MerchantOverviewHandler {
	return &MerchantOverviewHandler{getOverviewUC: getOverviewUC}
}

// @Summary Obtenir l'aperçu du marchand
// @Description Retourne les statistiques et l'aperçu général de la boutique du marchand connecté (ventes, commandes en cours, etc.).
// @Tags Merchant Overview
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/merchant/overview [get]
func (h *MerchantOverviewHandler) GetOverview(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("Handling merchant overview request")

	response, err := h.getOverviewUC.Execute(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec overview
		metrics.MerchantOverviewRequestTotal.WithLabelValues("error").Inc()
		metrics.MerchantOverviewDuration.Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("merchant_overview", "merchant_overview_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Failed to get merchant overview")

		return utils.NewAppError("MERCHANT_OVERVIEW_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// 📊 MÉTRIQUES : Succès overview
	metrics.MerchantOverviewRequestTotal.WithLabelValues("success").Inc()
	metrics.MerchantOverviewDuration.Observe(duration)

	logger.Info().
		Float64("duration_seconds", duration).
		Msg("✅ Merchant overview retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}
