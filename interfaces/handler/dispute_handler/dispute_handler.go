package disputehandler

import (
	"encoding/json"
	"fmt"
	"net/http"

	disputeusecase "Goshop/application/usecase/dispute_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type DisputeHandler struct {
	openDisputeUC    *disputeusecase.OpenDisputeUsecase
	resolveDisputeUC *disputeusecase.ResolveDisputeUsecase
	disputeRepo      repository.DisputeRepository
}

func NewDisputeHandler(
	openDisputeUC *disputeusecase.OpenDisputeUsecase,
	resolveDisputeUC *disputeusecase.ResolveDisputeUsecase,
	disputeRepo repository.DisputeRepository,
) *DisputeHandler {
	return &DisputeHandler{
		openDisputeUC:    openDisputeUC,
		resolveDisputeUC: resolveDisputeUC,
		disputeRepo:      disputeRepo,
	}
}

// OpenDispute permet à un client ou un marchand d'ouvrir un litige sur une commande
func (h *DisputeHandler) OpenDispute(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)
	orderID := chi.URLParam(r, "id")

	// Récupération du tenant (boutique) depuis le contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

	// Récupération de l'ID de l'utilisateur connecté
	userIDStr, ok := utils.UserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return fmt.Errorf("unauthorized: user ID not found in context")
	}

	initiatorID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID in context: %w", err)
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "invalid request body", http.StatusBadRequest)
	}

	if len(req.Reason) < 10 {
		return utils.NewAppError("INVALID_REASON", "reason must be at least 10 characters long", http.StatusBadRequest)
	}

	disputeReq := &disputeusecase.OpenDisputeRequest{
		OrderID:       orderID,
		ShopID:        shop.ID, // ✅ On passe directement l'ID de la boutique du contexte
		InitiatorID:   initiatorID,
		InitiatorRole: "customer",
		Reason:        req.Reason,
	}

	dispute, err := h.openDisputeUC.Execute(ctx, disputeReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to open dispute")
		return fmt.Errorf("failed to open dispute: %w", err)
	}

	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Dispute opened successfully",
		"dispute": dispute,
	})
	return nil
}

// ResolveDispute permet à un administrateur de trancher un litige via l'ID du litige
func (h *DisputeHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	disputeIDStr := chi.URLParam(r, "id")

	disputeID, err := uuid.Parse(disputeIDStr)
	if err != nil {
		return utils.NewAppError("INVALID_DISPUTE_ID", "invalid dispute ID format", http.StatusBadRequest)
	}

	var req struct {
		Resolution string `json:"resolution"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "invalid request body", http.StatusBadRequest)
	}

	if req.Resolution != "merchant_wins" && req.Resolution != "customer_wins" {
		return utils.NewAppError("INVALID_RESOLUTION", "resolution must be 'merchant_wins' or 'customer_wins'", http.StatusBadRequest)
	}

	userIDStr, ok := utils.UserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return fmt.Errorf("unauthorized: admin user ID not found in context")
	}
	resolverID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID in context: %w", err)
	}

	disputeReq := &disputeusecase.ResolveDisputeRequest{
		DisputeID:  disputeID,
		Resolution: req.Resolution,
		Notes:      req.Notes,
		ResolverID: resolverID,
	}

	dispute, err := h.resolveDisputeUC.Execute(ctx, disputeReq)
	if err != nil {
		return fmt.Errorf("failed to resolve dispute: %w", err)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Dispute resolved successfully",
		"dispute": dispute,
	})
	return nil
}

// ResolveOrderByDispute permet à un admin de trancher un litige via l'ID de la commande
func (h *DisputeHandler) ResolveOrderByDispute(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	orderIDStr := chi.URLParam(r, "id")
	if orderIDStr == "" {
		return utils.NewAppError("INVALID_ORDER_ID", "order_id is required", http.StatusBadRequest)
	}

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return utils.NewAppError("INVALID_ORDER_ID", "invalid order ID format", http.StatusBadRequest)
	}

	var req struct {
		Resolution string `json:"resolution"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "invalid request body", http.StatusBadRequest)
	}

	if req.Resolution != "merchant_wins" && req.Resolution != "customer_wins" {
		return utils.NewAppError("INVALID_RESOLUTION", "resolution must be 'merchant_wins' or 'customer_wins'", http.StatusBadRequest)
	}

	// 1. Retrouver le litige associé à cette commande
	dispute, err := h.disputeRepo.FindByOrderID(ctx, orderID)
	if err != nil || dispute == nil {
		return utils.NewAppError("DISPUTE_NOT_FOUND", "no active dispute found for this order", http.StatusNotFound)
	}

	if dispute.Status != entity.DisputeStatusPending && dispute.Status != entity.DisputeStatusUnderReview {
		return utils.NewAppError("DISPUTE_ALREADY_RESOLVED", "dispute is already resolved or cancelled", http.StatusBadRequest)
	}

	userIDStr, ok := utils.UserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return fmt.Errorf("unauthorized: admin user ID not found in context")
	}
	resolverID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID in context: %w", err)
	}

	// 2. Appeler le usecase de résolution
	resolveReq := &disputeusecase.ResolveDisputeRequest{
		DisputeID:  dispute.ID,
		Resolution: req.Resolution,
		Notes:      req.Notes,
		ResolverID: resolverID,
	}

	updatedDispute, err := h.resolveDisputeUC.Execute(ctx, resolveReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to resolve dispute by order")
		return fmt.Errorf("failed to resolve dispute by order: %w", err)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Dispute resolved successfully",
		"dispute": updatedDispute,
	})
	return nil
}
