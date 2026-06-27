package orders

import (
	"encoding/json"
	"net/http"

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

// AcceptOrder accepte une commande cash
func (h *CashOrderHandler) AcceptOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	logger.Info().
		Str("order_id", orderID).
		Msg("Accepting order")

	order, err := h.acceptUC.Execute(ctx, orderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to accept order")
		return utils.NewAppError("ORDER_ACCEPT_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// RejectOrder rejette une commande cash
func (h *CashOrderHandler) RejectOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
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
	if err != nil {
		logger.Error().Err(err).Msg("Failed to reject order")
		return utils.NewAppError("ORDER_REJECT_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// OutForDelivery marque une commande comme en cours de livraison
func (h *CashOrderHandler) OutForDelivery(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	logger.Info().
		Str("order_id", orderID).
		Msg("Marking order as out for delivery")

	order, err := h.outForDeliveryUC.Execute(ctx, orderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to mark out for delivery")
		return utils.NewAppError("ORDER_DELIVERY_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// DeliverOrder marque une commande comme livrée et crée le paiement cash
func (h *CashOrderHandler) DeliverOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
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
	if err != nil {
		logger.Error().Err(err).Msg("Failed to deliver order")
		return utils.NewAppError("ORDER_DELIVER_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}

// CancelOrder annule une commande
func (h *CashOrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		return utils.ErrNotFound
	}

	logger.Info().
		Str("order_id", orderID).
		Msg("Cancelling order")

	order, err := h.cancelUC.Execute(ctx, orderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to cancel order")
		return utils.NewAppError("ORDER_CANCEL_FAILED", err.Error(), http.StatusBadRequest)
	}

	utils.WriteJSON(w, http.StatusOK, order)
	return nil
}
