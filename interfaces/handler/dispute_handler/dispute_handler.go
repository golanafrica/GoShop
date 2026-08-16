package disputehandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"Goshop/application/metrics"
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
	adminDisputeUC   *disputeusecase.AdminDisputeUsecase
	disputeRepo      repository.DisputeRepository
}

func NewDisputeHandler(
	openDisputeUC *disputeusecase.OpenDisputeUsecase,
	resolveDisputeUC *disputeusecase.ResolveDisputeUsecase,
	adminDisputeUC *disputeusecase.AdminDisputeUsecase,
	disputeRepo repository.DisputeRepository,
) *DisputeHandler {
	return &DisputeHandler{
		openDisputeUC:    openDisputeUC,
		resolveDisputeUC: resolveDisputeUC,
		adminDisputeUC:   adminDisputeUC,
		disputeRepo:      disputeRepo,
	}
}

// mapResolveErr mappe les conflits concurrent → HTTP 409
func mapResolveErr(err error) error {
	switch {
	case errors.Is(err, disputeusecase.ErrDisputeAlreadyResolved),
		errors.Is(err, disputeusecase.ErrEscrowClaimConflict),
		errors.Is(err, disputeusecase.ErrEscrowNotDisputed):
		return utils.NewAppError("DISPUTE_CONFLICT", err.Error(), http.StatusConflict)
	default:
		return err
	}
}

// ============================================================
// ROUTES UTILISATEUR / MARCHAND
// ============================================================

func (h *DisputeHandler) OpenDispute(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)
	orderID := chi.URLParam(r, "id")

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

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
		ShopID:        shop.ID,
		InitiatorID:   initiatorID,
		InitiatorRole: "customer",
		Reason:        req.Reason,
	}

	dispute, err := h.openDisputeUC.Execute(ctx, disputeReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec ouverture
		metrics.DisputeOpenTotal.WithLabelValues("error", "customer").Inc()
		metrics.DisputeOperationDuration.WithLabelValues("open").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("dispute_open", "dispute_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", orderID).
			Float64("duration_seconds", duration).
			Msg("Failed to open dispute")

		return fmt.Errorf("failed to open dispute: %w", err)
	}

	// 📊 MÉTRIQUES : Succès ouverture
	metrics.DisputeOpenTotal.WithLabelValues("success", "customer").Inc()
	metrics.DisputeOperationDuration.WithLabelValues("open").Observe(duration)

	logger.Info().
		Str("order_id", orderID).
		Str("dispute_id", dispute.ID.String()).
		Float64("duration_seconds", duration).
		Msg("✅ Dispute opened successfully")

	utils.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Dispute opened successfully",
		"dispute": dispute,
	})
	return nil
}

// ============================================================
// ROUTES ADMIN
// ============================================================

func (h *DisputeHandler) GetAllDisputes(w http.ResponseWriter, r *http.Request) error {
	start := time.Now()
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 20
	}

	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	req := &disputeusecase.GetAllDisputesRequest{
		Status: status,
		Limit:  limit,
		Offset: offset,
	}

	disputes, total, err := h.adminDisputeUC.GetAllDisputes(ctx, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec listing
		metrics.DisputeOperationDuration.WithLabelValues("list").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("dispute_list", "dispute_handler").Inc()

		logger.Error().Err(err).
			Int("limit", limit).
			Int("offset", offset).
			Float64("duration_seconds", duration).
			Msg("Failed to fetch disputes")

		return fmt.Errorf("failed to fetch disputes: %w", err)
	}

	// 📊 MÉTRIQUES : Succès listing
	metrics.DisputeListTotal.Inc()
	metrics.DisputeOperationDuration.WithLabelValues("list").Observe(duration)
	metrics.DisputeListedCount.Observe(float64(len(disputes)))

	logger.Info().
		Int("disputes_returned", len(disputes)).
		Int("total", total).
		Int("limit", limit).
		Int("offset", offset).
		Float64("duration_seconds", duration).
		Msg("✅ Disputes listed successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    disputes,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
	return nil
}

func (h *DisputeHandler) GetDisputeByID(w http.ResponseWriter, r *http.Request) error {
	start := time.Now()
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	id := chi.URLParam(r, "id")

	dispute, err := h.adminDisputeUC.GetDisputeByID(ctx, id)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec get
		metrics.DisputeOperationDuration.WithLabelValues("get").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("dispute_get", "dispute_handler").Inc()

		logger.Error().Err(err).
			Str("dispute_id", id).
			Float64("duration_seconds", duration).
			Msg("Failed to get dispute by ID")

		return fmt.Errorf("dispute not found: %w", err)
	}

	// 📊 MÉTRIQUES : Succès get
	metrics.DisputeGetTotal.Inc()
	metrics.DisputeOperationDuration.WithLabelValues("get").Observe(duration)

	logger.Info().
		Str("dispute_id", id).
		Float64("duration_seconds", duration).
		Msg("✅ Dispute retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    dispute,
	})
	return nil
}

// ResolveDispute — admin tranche un litige par dispute ID
func (h *DisputeHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

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
	duration := time.Since(start).Seconds()

	if err != nil {
		if mapped := mapResolveErr(err); mapped != err {
			// 📊 MÉTRIQUES : Conflit (409)
			metrics.DisputeConflictTotal.WithLabelValues("resolve").Inc()
			metrics.DisputeResolveTotal.WithLabelValues(req.Resolution, "conflict").Inc()
			metrics.DisputeOperationDuration.WithLabelValues("resolve").Observe(duration)

			logger.Warn().
				Str("dispute_id", disputeIDStr).
				Str("resolution", req.Resolution).
				Float64("duration_seconds", duration).
				Msg("⚠️ Dispute resolution conflict")

			return mapped // 409 DISPUTE_CONFLICT
		}

		// 📊 MÉTRIQUES : Erreur
		metrics.DisputeResolveTotal.WithLabelValues(req.Resolution, "error").Inc()
		metrics.DisputeOperationDuration.WithLabelValues("resolve").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("dispute_resolve", "dispute_handler").Inc()

		logger.Error().Err(err).
			Str("dispute_id", disputeIDStr).
			Str("resolution", req.Resolution).
			Float64("duration_seconds", duration).
			Msg("Failed to resolve dispute")

		return fmt.Errorf("failed to resolve dispute: %w", err)
	}

	// 📊 MÉTRIQUES : Succès résolution
	metrics.DisputeResolveTotal.WithLabelValues(req.Resolution, "success").Inc()
	metrics.DisputeOperationDuration.WithLabelValues("resolve").Observe(duration)

	logger.Info().
		Str("dispute_id", disputeIDStr).
		Str("resolution", req.Resolution).
		Float64("duration_seconds", duration).
		Msg("✅ Dispute resolved successfully")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Dispute resolved successfully",
		"dispute": dispute,
	})
	return nil
}

// ResolveOrderByDispute — admin tranche via order ID
func (h *DisputeHandler) ResolveOrderByDispute(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
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

	dispute, err := h.disputeRepo.FindByOrderID(ctx, orderID)
	if err != nil || dispute == nil {
		return utils.NewAppError("DISPUTE_NOT_FOUND", "no active dispute found for this order", http.StatusNotFound)
	}

	if dispute.Status != entity.DisputeStatusPending && dispute.Status != entity.DisputeStatusUnderReview {
		return utils.NewAppError(
			"DISPUTE_CONFLICT",
			"dispute is already resolved or cancelled",
			http.StatusConflict,
		)
	}

	userIDStr, ok := utils.UserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		return fmt.Errorf("unauthorized: admin user ID not found in context")
	}
	resolverID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID in context: %w", err)
	}

	resolveReq := &disputeusecase.ResolveDisputeRequest{
		DisputeID:  dispute.ID,
		Resolution: req.Resolution,
		Notes:      req.Notes,
		ResolverID: resolverID,
	}

	updatedDispute, err := h.resolveDisputeUC.Execute(ctx, resolveReq)
	duration := time.Since(start).Seconds()

	if err != nil {
		if mapped := mapResolveErr(err); mapped != err {
			// 📊 MÉTRIQUES : Conflit (409)
			metrics.DisputeConflictTotal.WithLabelValues("resolve_by_order").Inc()
			metrics.DisputeResolveTotal.WithLabelValues(req.Resolution, "conflict").Inc()
			metrics.DisputeOperationDuration.WithLabelValues("resolve_by_order").Observe(duration)

			logger.Warn().
				Str("order_id", orderIDStr).
				Str("resolution", req.Resolution).
				Float64("duration_seconds", duration).
				Msg("⚠️ Dispute resolution conflict (by order)")

			return mapped // 409
		}

		// 📊 MÉTRIQUES : Erreur
		metrics.DisputeResolveTotal.WithLabelValues(req.Resolution, "error").Inc()
		metrics.DisputeOperationDuration.WithLabelValues("resolve_by_order").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("dispute_resolve_by_order", "dispute_handler").Inc()

		logger.Error().Err(err).
			Str("order_id", orderIDStr).
			Str("resolution", req.Resolution).
			Float64("duration_seconds", duration).
			Msg("Failed to resolve dispute by order")

		return fmt.Errorf("failed to resolve dispute by order: %w", err)
	}

	// 📊 MÉTRIQUES : Succès résolution
	metrics.DisputeResolveTotal.WithLabelValues(req.Resolution, "success").Inc()
	metrics.DisputeOperationDuration.WithLabelValues("resolve_by_order").Observe(duration)

	logger.Info().
		Str("order_id", orderIDStr).
		Str("dispute_id", updatedDispute.ID.String()).
		Str("resolution", req.Resolution).
		Float64("duration_seconds", duration).
		Msg("✅ Dispute resolved successfully (by order)")

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Dispute resolved successfully",
		"dispute": updatedDispute,
	})
	return nil
}
