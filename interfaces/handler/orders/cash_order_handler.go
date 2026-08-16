package orders

import (
	"encoding/json"
	"net/http"
	"time"

	"Goshop/application/metrics"
	orderusecase "Goshop/application/usecase/order_usecase"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// CashOrderHandler gère les endpoints pour le workflow cash à la livraison
type CashOrderHandler struct {
	acceptUC         *orderusecase.AcceptOrderUsecase
	rejectUC         *orderusecase.RejectOrderUsecase
	outForDeliveryUC *orderusecase.OutForDeliveryUsecase
	deliverUC        *orderusecase.DeliverOrderUsecase
	cancelUC         *orderusecase.CancelOrderUsecase
}

// NewCashOrderHandler crée une nouvelle instance
func NewCashOrderHandler(
	acceptUC *orderusecase.AcceptOrderUsecase,
	rejectUC *orderusecase.RejectOrderUsecase,
	outForDeliveryUC *orderusecase.OutForDeliveryUsecase,
	deliverUC *orderusecase.DeliverOrderUsecase,
	cancelUC *orderusecase.CancelOrderUsecase,
) *CashOrderHandler {
	return &CashOrderHandler{
		acceptUC:         acceptUC,
		rejectUC:         rejectUC,
		outForDeliveryUC: outForDeliveryUC,
		deliverUC:        deliverUC,
		cancelUC:         cancelUC,
	}
}

// RejectRequest représente la requête de rejet
type RejectRequest struct {
	Reason string `json:"reason"`
}

// Validate valide la requête
func (r *RejectRequest) Validate() error {
	if r.Reason == "" {
		return utils.NewAppError("VALIDATION_FAILED", "reason is required", http.StatusBadRequest)
	}
	if len(r.Reason) > 500 {
		return utils.NewAppError("VALIDATION_FAILED", "reason too long (max 500 chars)", http.StatusBadRequest)
	}
	return nil
}

// DeliverRequest représente la requête de livraison
type DeliverRequest struct {
	AmountReceived int64  `json:"amount_received"`
	Notes          string `json:"notes"`
}

// Validate valide la requête
func (r *DeliverRequest) Validate() error {
	if r.AmountReceived <= 0 {
		return utils.NewAppError("VALIDATION_FAILED", "amount_received must be positive", http.StatusBadRequest)
	}
	if len(r.Notes) > 1000 {
		return utils.NewAppError("VALIDATION_FAILED", "notes too long (max 1000 chars)", http.StatusBadRequest)
	}
	return nil
}

// @Summary Accepter une commande Cash on Delivery
// @Description Permet au marchand d'accepter une commande en attente de confirmation pour le paiement à la livraison.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande (UUID)"
// @Success 200 {object} entity.Order
// @Failure 400 {object} utils.AppError "Payload invalide ou commande non éligible"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Commande introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/accept [post]
func (h *CashOrderHandler) AcceptOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	logger.Info().
		Str("order_id", orderID).
		Msg("Accepting order")

	order, err := h.acceptUC.Execute(ctx, orderID)
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Msg("Failed to accept order")

		// 📊 MÉTRIQUES : Échec acceptation
		metrics.CODOperationTotal.WithLabelValues("accept", "error").Inc()
		metrics.CODOperationDuration.WithLabelValues("accept").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_accept", "cash_order_handler").Inc()

		return utils.NewAppError("ORDER_ACCEPT_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès acceptation
	metrics.CODOperationTotal.WithLabelValues("accept", "success").Inc()
	metrics.CODOperationDuration.WithLabelValues("accept").Observe(duration)

	logger.Info().
		Str("order_id", orderID).
		Float64("duration_seconds", duration).
		Msg("Order accepted successfully")

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// @Summary Rejeter une commande Cash on Delivery
// @Description Permet au marchand de rejeter une commande avec un motif obligatoire.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande (UUID)"
// @Param request body orders.RejectRequest true "Motif du rejet"
// @Success 200 {object} entity.Order
// @Failure 400 {object} utils.AppError "Payload invalide ou motif manquant"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Commande introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/reject [post]
func (h *CashOrderHandler) RejectOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	var req RejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Failed to decode reject request")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		return err
	}

	logger.Info().
		Str("order_id", orderID).
		Str("reason", req.Reason).
		Msg("Rejecting order")

	order, err := h.rejectUC.Execute(ctx, orderID, req.Reason)
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Msg("Failed to reject order")

		// 📊 MÉTRIQUES : Échec rejet
		metrics.CODOperationTotal.WithLabelValues("reject", "error").Inc()
		metrics.CODOperationDuration.WithLabelValues("reject").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_reject", "cash_order_handler").Inc()

		return utils.NewAppError("ORDER_REJECT_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès rejet
	metrics.CODOperationTotal.WithLabelValues("reject", "success").Inc()
	metrics.CODOperationDuration.WithLabelValues("reject").Observe(duration)

	logger.Info().
		Str("order_id", orderID).
		Float64("duration_seconds", duration).
		Msg("Order rejected successfully")

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// @Summary Marquer une commande comme en cours de livraison
// @Description Met à jour le statut de la commande pour indiquer qu'elle est en cours de livraison.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande (UUID)"
// @Success 200 {object} entity.Order
// @Failure 400 {object} utils.AppError "Commande non éligible pour la livraison"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Commande introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/out-for-delivery [post]
func (h *CashOrderHandler) OutForDelivery(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	logger.Info().
		Str("order_id", orderID).
		Msg("Marking order as out for delivery")

	order, err := h.outForDeliveryUC.Execute(ctx, orderID)
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Msg("Failed to mark out for delivery")

		// 📊 MÉTRIQUES : Échec out_for_delivery
		metrics.CODOperationTotal.WithLabelValues("out_for_delivery", "error").Inc()
		metrics.CODOperationDuration.WithLabelValues("out_for_delivery").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_out_for_delivery", "cash_order_handler").Inc()

		return utils.NewAppError("ORDER_DELIVERY_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès out_for_delivery
	metrics.CODOperationTotal.WithLabelValues("out_for_delivery", "success").Inc()
	metrics.CODOperationDuration.WithLabelValues("out_for_delivery").Observe(duration)

	logger.Info().
		Str("order_id", orderID).
		Float64("duration_seconds", duration).
		Msg("Order marked as out for delivery")

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// @Summary Confirmer la livraison d'une commande
// @Description Marque la commande comme livrée et enregistre le paiement en espèces reçu.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande (UUID)"
// @Param request body orders.DeliverRequest true "Détails de la livraison (montant reçu, notes)"
// @Success 200 {object} entity.Order
// @Failure 400 {object} utils.AppError "Payload invalide ou montant incorrect"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Commande introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/deliver [post]
func (h *CashOrderHandler) DeliverOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	var req DeliverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Failed to decode deliver request")
		return utils.ErrInvalidPayload
	}

	if err := req.Validate(); err != nil {
		return err
	}

	logger.Info().
		Str("order_id", orderID).
		Int64("amount_received", req.AmountReceived).
		Msg("Delivering order")

	deliverReq := &orderusecase.DeliverRequest{
		AmountReceived: req.AmountReceived,
		Notes:          req.Notes,
	}

	order, err := h.deliverUC.Execute(ctx, orderID, deliverReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Msg("Failed to deliver order")

		// 📊 MÉTRIQUES : Échec livraison
		metrics.CODOperationTotal.WithLabelValues("deliver", "error").Inc()
		metrics.CODOperationDuration.WithLabelValues("deliver").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_deliver", "cash_order_handler").Inc()

		return utils.NewAppError("ORDER_DELIVER_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès livraison
	metrics.CODOperationTotal.WithLabelValues("deliver", "success").Inc()
	metrics.CODOperationDuration.WithLabelValues("deliver").Observe(duration)

	// 📊 MÉTRIQUE : Montant reçu en livraison
	if req.AmountReceived > 0 {
		metrics.CODDeliveryAmountCents.Observe(float64(req.AmountReceived))
	}

	logger.Info().
		Str("order_id", orderID).
		Int64("amount_received", req.AmountReceived).
		Float64("duration_seconds", duration).
		Msg("Order delivered successfully")

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// @Summary Annuler une commande
// @Description Annule une commande existante et libère le stock réservé.
// @Tags Cash On Delivery (COD)
// @Accept json
// @Produce json
// @Param id path string true "ID de la commande (UUID)"
// @Success 200 {object} entity.Order
// @Failure 400 {object} utils.AppError "Commande non éligible à l'annulation"
// @Failure 401 {object} utils.AppError "Non autorisé"
// @Failure 404 {object} utils.AppError "Commande introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/orders/{id}/cancel [post]
func (h *CashOrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	logger.Info().
		Str("order_id", orderID).
		Msg("Cancelling order")

	order, err := h.cancelUC.Execute(ctx, orderID)
	duration := time.Since(start).Seconds()

	if err != nil {
		logger.Error().Err(err).Msg("Failed to cancel order")

		// 📊 MÉTRIQUES : Échec annulation
		metrics.CODOperationTotal.WithLabelValues("cancel", "error").Inc()
		metrics.CODOperationDuration.WithLabelValues("cancel").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("cod_cancel", "cash_order_handler").Inc()

		return utils.NewAppError("ORDER_CANCEL_FAILED", err.Error(), http.StatusBadRequest)
	}

	// 📊 MÉTRIQUES : Succès annulation
	metrics.CODOperationTotal.WithLabelValues("cancel", "success").Inc()
	metrics.CODOperationDuration.WithLabelValues("cancel").Observe(duration)

	logger.Info().
		Str("order_id", orderID).
		Float64("duration_seconds", duration).
		Msg("Order cancelled successfully")

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}
