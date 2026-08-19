package orders

import (
	"net/http"
	"time"

	"Goshop/application/metrics"
	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// SyncOrderHandler gère la synchronisation des paiements orders
type SyncOrderHandler struct {
	syncUC *orderusecase.SyncOrderPaymentUsecase
}

// NewSyncOrderHandler crée une nouvelle instance
func NewSyncOrderHandler(syncUC *orderusecase.SyncOrderPaymentUsecase) *SyncOrderHandler {
	return &SyncOrderHandler{
		syncUC: syncUC,
	}
}

// @Summary Synchroniser le statut d'un paiement order
// @Description Vérifie le statut du paiement auprès du provider et confirme l'order si nécessaire
// @Tags Orders
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande"
// @Success 200 {object} orderusecase.SyncOrderPaymentResponse
// @Failure 400 {object} utils.AppError "Order ID invalide"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 403 {object} utils.AppError "Accès refusé"
// @Failure 404 {object} utils.AppError "Order non trouvée"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/sync [post]
func (h *SyncOrderHandler) SyncOrderPayment(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 🔧 FIX PHASE 1 : "id" au lieu de "order_id" pour matcher la route /{id}/sync
	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.NewAppError("INVALID_ORDER_ID", "order_id is required", http.StatusBadRequest)
	}

	// Vérifier l'authentification
	authUserID, ok := utils.UserIDFromContext(ctx)
	if !ok || authUserID == "" {
		return utils.ErrUnauthorized
	}

	req := &orderusecase.SyncOrderPaymentRequest{
		OrderID: orderID,
	}

	resp, err := h.syncUC.Execute(ctx, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec sync
		metrics.SyncOrderTotal.WithLabelValues("error").Inc()
		metrics.SyncOrderDuration.Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("sync_order", "sync_order_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", orderID).
			Str("user_id", authUserID).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to sync order payment")

		return utils.NewAppError("SYNC_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès sync
	metrics.SyncOrderTotal.WithLabelValues("success").Inc()
	metrics.SyncOrderDuration.Observe(duration)

	// ✅ CORRECTION : On compte chaque sync réussi comme un paiement potentiellement confirmé
	// (Le usecase gère la logique de confirmation en interne)
	metrics.SyncOrderPaymentConfirmed.Inc()

	// ✅ CORRECTION : Log simplifié sans accès à resp.Status
	logger.Info().
		Str("order_id", orderID).
		Str("user_id", authUserID).
		Float64("duration_seconds", duration).
		Msg("✅ Order payment synced successfully")

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}
