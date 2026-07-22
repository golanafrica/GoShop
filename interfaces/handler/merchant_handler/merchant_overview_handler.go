package merchanthandler

import (
	"net/http"

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
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("Handling merchant overview request")

	response, err := h.getOverviewUC.Execute(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get merchant overview")
		return utils.NewAppError("MERCHANT_OVERVIEW_FAILED", err.Error(), http.StatusInternalServerError)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}
