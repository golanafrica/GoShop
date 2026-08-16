package customerhandler

import (
	"errors"
	"net/http"
	"time"

	"Goshop/application/metrics"
	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type CustomerDashboardHandler struct {
	getDashboardUC *customerusecase.GetClientDashboardUsecase
	customerRepo   repository.CustomerRepositoryInterface
}

func NewCustomerDashboardHandler(
	getDashboardUC *customerusecase.GetClientDashboardUsecase,
	customerRepo repository.CustomerRepositoryInterface,
) *CustomerDashboardHandler {
	return &CustomerDashboardHandler{
		getDashboardUC: getDashboardUC,
		customerRepo:   customerRepo,
	}
}

// @Summary Obtenir le tableau de bord financier du client
// @Description Retourne la vue d'ensemble financière du client connecté. L'ID client est résolu de manière sécurisée via le JWT.
// @Tags Customer Dashboard
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} utils.AppError "Non autorisé (JWT manquant ou invalide)"
// @Failure 404 {object} utils.AppError "Profil client introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/client/dashboard [get]
func (h *CustomerDashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer l'ID utilisateur du JWT (users.id)
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		logger.Warn().Msg("User ID not found in context")
		return utils.ErrUnauthorized
	}

	// 2. 🛡️ TRADUCTION SÉCURISÉE : Trouver le customer.id correspondant à ce user.id
	// (FindByUserID lit automatiquement le shop_id depuis le contexte multi-tenant)
	customer, err := h.customerRepo.FindByUserID(ctx, authUserID)
	if err != nil {
		logger.Warn().Err(err).Str("user_id", authUserID).Msg("Customer profile not found for dashboard")
		return utils.NewAppError("CUSTOMER_NOT_FOUND", "Customer profile not found", http.StatusNotFound)
	}

	// 3. Exécuter le usecase avec le VRAI customer.id
	dashboard, err := h.getDashboardUC.Execute(ctx, customer.ID)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec dashboard
		metrics.CustomerDashboardRequestTotal.WithLabelValues("error").Inc()
		metrics.CustomerDashboardDuration.Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("customer_dashboard", "dashboard_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("Failed to get dashboard")

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return utils.ErrInternalServer
	}

	// 📊 MÉTRIQUES : Succès dashboard
	metrics.CustomerDashboardRequestTotal.WithLabelValues("success").Inc()
	metrics.CustomerDashboardDuration.Observe(duration)

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    dashboard,
	})

	return nil
}
