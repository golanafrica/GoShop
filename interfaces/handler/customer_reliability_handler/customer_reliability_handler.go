package customerreliabilityhandler

import (
	"net/http"

	customerreliabilityusecase "Goshop/application/usecase/customer_reliability_usecase"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type CustomerReliabilityHandler struct {
	calculateUC *customerreliabilityusecase.CalculateReliabilityScoreUsecase
}

func NewCustomerReliabilityHandler(
	calculateUC *customerreliabilityusecase.CalculateReliabilityScoreUsecase,
) *CustomerReliabilityHandler {
	return &CustomerReliabilityHandler{
		calculateUC: calculateUC,
	}
}

// GetCustomerScore godoc
// @Summary Obtenir le score de fiabilité d'un client
// @Tags Customer Reliability
// @Produce json
// @Param customer_id path string true "ID du client"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.AppError
// @Failure 404 {object} utils.AppError
// @Security ApiKeyAuth
// @Router /api/customers/{customer_id}/reliability-score [get]
func (h *CustomerReliabilityHandler) GetCustomerScore(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	customerID := chi.URLParam(r, "customer_id")
	if customerID == "" {
		return utils.NewAppError("INVALID_CUSTOMER_ID", "customer_id is required", http.StatusBadRequest)
	}

	// On calcule (ou met à jour) le score à la volée pour avoir la donnée la plus fraîche
	score, err := h.calculateUC.Execute(ctx, customerID)
	if err != nil {
		logger.Error().Err(err).Str("customer_id", customerID).Msg("Failed to get reliability score")
		return utils.NewAppError("SCORE_CALCULATION_FAILED", err.Error(), http.StatusInternalServerError)
	}

	// ✅ Correction : WriteJSON ne retourne pas d'error, on l'appelle puis on retourne nil
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"customer_id":                 score.CustomerID,
			"score":                       score.Score,
			"tier":                        score.Tier,
			"max_installments":            score.GetMaxInstallments(),
			"has_installment_limit":       score.HasInstallmentLimit(),
			"can_join_commercial_tontine": score.CanJoinCommercialTontine(),
			"last_calculated_at":          score.LastCalculatedAt,
		},
	})

	return nil
}
