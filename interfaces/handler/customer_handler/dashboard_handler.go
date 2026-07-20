package customerhandler

import (
	"errors"
	"net/http"

	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type CustomerDashboardHandler struct {
	getDashboardUC *customerusecase.GetClientDashboardUsecase
}

func NewCustomerDashboardHandler(getDashboardUC *customerusecase.GetClientDashboardUsecase) *CustomerDashboardHandler {
	return &CustomerDashboardHandler{
		getDashboardUC: getDashboardUC,
	}
}

// GetDashboard retourne la vue d'ensemble financière du client connecté
// GET /api/client/dashboard
func (h *CustomerDashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer l'ID du client depuis le token JWT (injecté par le middleware d'auth)
	customerID, ok := utils.UserIDFromContext(ctx)
	if !ok || customerID == "" {
		logger.Warn().Msg("Customer ID not found in context")
		return utils.ErrUnauthorized
	}

	// 2. Exécuter le usecase
	dashboard, err := h.getDashboardUC.Execute(ctx, customerID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get dashboard")

		var appErr *utils.AppError
		if err.Error() == "customer not found" {
			return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer not found", http.StatusNotFound)
		}
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrInternalServer
	}

	// 3. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    dashboard,
	})

	return nil
}
