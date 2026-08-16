package adminshophandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Goshop/application/metrics"
	adminshopusecase "Goshop/application/usecase/admin_shop_usecase"
	"Goshop/domain/entity"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.2.0 : ADMIN SHOP HANDLER
// ============================================================

// AdminShopHandler gère les endpoints admin pour les shops
type AdminShopHandler struct {
	listUC     *adminshopusecase.ListShopsUsecase
	detailsUC  *adminshopusecase.GetShopDetailsUsecase
	healthUC   *adminshopusecase.GetShopHealthUsecase
	suspendUC  *adminshopusecase.SuspendShopUsecase
	activateUC *adminshopusecase.ActivateShopUsecase
}

// NewAdminShopHandler crée une nouvelle instance
func NewAdminShopHandler(
	listUC *adminshopusecase.ListShopsUsecase,
	detailsUC *adminshopusecase.GetShopDetailsUsecase,
	healthUC *adminshopusecase.GetShopHealthUsecase,
	suspendUC *adminshopusecase.SuspendShopUsecase,
	activateUC *adminshopusecase.ActivateShopUsecase,
) *AdminShopHandler {
	return &AdminShopHandler{
		listUC:     listUC,
		detailsUC:  detailsUC,
		healthUC:   healthUC,
		suspendUC:  suspendUC,
		activateUC: activateUC,
	}
}

// ============================================================
// ENDPOINTS HTTP
// ============================================================

// @Summary Liste cross-tenant des boutiques
func (h *AdminShopHandler) ListShops(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire AdminContext
	admin, err := extractAdminContext(r)
	if err != nil {
		logger.Warn().Err(err).Msg("❌ Erreur extraction AdminContext")
		return utils.ErrUnauthorized
	}

	// 2. Parser les query parameters
	req := &adminshopusecase.ListShopsRequest{
		Limit:     parseIntParam(r, "limit", 20),
		Offset:    parseIntParam(r, "offset", 0),
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
		Search:    r.URL.Query().Get("search"),
	}

	// Filtres optionnels
	if kycStatus := r.URL.Query().Get("kyc_status"); kycStatus != "" {
		status := entity.ShopKYCStatus(kycStatus)
		req.KYCStatus = &status
	}
	if plan := r.URL.Query().Get("plan"); plan != "" {
		p := entity.ShopPlan(plan)
		req.Plan = &p
	}
	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		b := isActive == "true"
		req.IsActive = &b
	}
	if healthLevel := r.URL.Query().Get("health_level"); healthLevel != "" {
		level := entity.ShopHealthLevel(healthLevel)
		req.HealthLevel = &level
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Int("limit", req.Limit).
		Int("offset", req.Offset).
		Msg("📋 Liste cross-tenant des shops")

	// 3. Exécuter le usecase
	response, err := h.listUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec listing
		metrics.AdminShopOperationTotal.WithLabelValues("list", "error").Inc()
		metrics.AdminShopOperationDuration.WithLabelValues("list").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("admin_shop_list", "admin_shop_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur liste shops")

		return err
	}

	// 📊 MÉTRIQUES : Succès listing
	metrics.AdminShopOperationTotal.WithLabelValues("list", "success").Inc()
	metrics.AdminShopOperationDuration.WithLabelValues("list").Observe(duration)
	metrics.AdminShopListedCount.Observe(float64(len(response.Shops)))

	logger.Info().
		Str("admin_id", admin.AdminID).
		Int("shops_returned", len(response.Shops)).
		Int("total", response.Total).
		Float64("duration_seconds", duration).
		Msg("✅ Shops listed successfully")

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Détails complets d'une boutique
func (h *AdminShopHandler) GetShopDetails(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire AdminContext
	admin, err := extractAdminContext(r)
	if err != nil {
		logger.Warn().Err(err).Msg("❌ Erreur extraction AdminContext")
		return utils.ErrUnauthorized
	}

	// 2. Récupérer l'ID depuis l'URL
	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.NewAppError("VALIDATION_ERROR", "shop_id is required in URL", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Msg("🔍 Récupération détails shop")

	// 3. Exécuter le usecase
	req := &adminshopusecase.GetShopDetailsRequest{ShopID: shopID}
	response, err := h.detailsUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec get details
		metrics.AdminShopOperationTotal.WithLabelValues("get_details", "error").Inc()
		metrics.AdminShopOperationDuration.WithLabelValues("get_details").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("admin_shop_get_details", "admin_shop_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur récupération détails")

		return handleShopError(err)
	}

	// 📊 MÉTRIQUES : Succès get details
	metrics.AdminShopOperationTotal.WithLabelValues("get_details", "success").Inc()
	metrics.AdminShopOperationDuration.WithLabelValues("get_details").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Float64("duration_seconds", duration).
		Msg("✅ Shop details retrieved successfully")

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Score de santé détaillé d'une boutique
func (h *AdminShopHandler) GetShopHealth(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire AdminContext
	admin, err := extractAdminContext(r)
	if err != nil {
		logger.Warn().Err(err).Msg("❌ Erreur extraction AdminContext")
		return utils.ErrUnauthorized
	}

	// 2. Récupérer l'ID depuis l'URL
	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.NewAppError("VALIDATION_ERROR", "shop_id is required in URL", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Msg("🏥 Récupération health score détaillé")

	// 3. Exécuter le usecase
	req := &adminshopusecase.GetShopHealthRequest{ShopID: shopID}
	response, err := h.healthUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec health check
		metrics.AdminShopOperationTotal.WithLabelValues("get_health", "error").Inc()
		metrics.AdminShopOperationDuration.WithLabelValues("get_health").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("admin_shop_get_health", "admin_shop_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur récupération health score")

		return handleShopError(err)
	}

	// 📊 MÉTRIQUES : Succès health check
	metrics.AdminShopOperationTotal.WithLabelValues("get_health", "success").Inc()
	metrics.AdminShopOperationDuration.WithLabelValues("get_health").Observe(duration)
	metrics.AdminShopHealthChecksTotal.Inc()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Float64("duration_seconds", duration).
		Msg("✅ Shop health score retrieved successfully")

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// SuspendShopRequest représente la requête de suspension
type SuspendShopRequest struct {
	Reason string `json:"reason"`
}

// @Summary Suspendre une boutique
func (h *AdminShopHandler) SuspendShop(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire AdminContext
	admin, err := extractAdminContext(r)
	if err != nil {
		logger.Warn().Err(err).Msg("❌ Erreur extraction AdminContext")
		return utils.ErrUnauthorized
	}

	// 2. Récupérer l'ID depuis l'URL
	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.NewAppError("VALIDATION_ERROR", "shop_id is required in URL", http.StatusBadRequest)
	}

	// 3. Parser le body
	var reqBody SuspendShopRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur décodage JSON")
		return utils.ErrInvalidPayload
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Str("reason", reqBody.Reason).
		Msg("⛔ Suspension boutique")

	// 4. Exécuter le usecase
	req := &adminshopusecase.SuspendShopRequest{
		ShopID: shopID,
		Reason: reqBody.Reason,
	}
	response, err := h.suspendUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec suspension
		metrics.AdminShopOperationTotal.WithLabelValues("suspend", "error").Inc()
		metrics.AdminShopOperationDuration.WithLabelValues("suspend").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("admin_shop_suspend", "admin_shop_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur suspension shop")

		return handleShopError(err)
	}

	// 📊 MÉTRIQUES : Succès suspension
	metrics.AdminShopOperationTotal.WithLabelValues("suspend", "success").Inc()
	metrics.AdminShopOperationDuration.WithLabelValues("suspend").Observe(duration)
	metrics.AdminShopSuspendTotal.Inc()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Str("reason", reqBody.Reason).
		Float64("duration_seconds", duration).
		Msg("✅ Shop suspended successfully")

	// 5. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ActivateShopRequest représente la requête de réactivation
type ActivateShopRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// @Summary Réactiver une boutique suspendue
func (h *AdminShopHandler) ActivateShop(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// 1. Extraire AdminContext
	admin, err := extractAdminContext(r)
	if err != nil {
		logger.Warn().Err(err).Msg("❌ Erreur extraction AdminContext")
		return utils.ErrUnauthorized
	}

	// 2. Récupérer l'ID depuis l'URL
	shopID := chi.URLParam(r, "id")
	if shopID == "" {
		return utils.NewAppError("VALIDATION_ERROR", "shop_id is required in URL", http.StatusBadRequest)
	}

	// 3. Parser le body (optionnel)
	var reqBody ActivateShopRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			logger.Warn().Err(err).Msg("⚠️ Erreur décodage JSON (non bloquant)")
		}
		defer r.Body.Close()
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Msg("✅ Réactivation boutique")

	// 4. Exécuter le usecase
	req := &adminshopusecase.ActivateShopRequest{
		ShopID: shopID,
		Reason: reqBody.Reason,
	}
	response, err := h.activateUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec activation
		metrics.AdminShopOperationTotal.WithLabelValues("activate", "error").Inc()
		metrics.AdminShopOperationDuration.WithLabelValues("activate").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("admin_shop_activate", "admin_shop_handler").Inc()

		logger.Error().Err(err).
			Str("shop_id", shopID).
			Float64("duration_seconds", duration).
			Msg("❌ Erreur réactivation shop")

		return handleShopError(err)
	}

	// 📊 MÉTRIQUES : Succès activation
	metrics.AdminShopOperationTotal.WithLabelValues("activate", "success").Inc()
	metrics.AdminShopOperationDuration.WithLabelValues("activate").Observe(duration)
	metrics.AdminShopActivateTotal.Inc()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("shop_id", shopID).
		Float64("duration_seconds", duration).
		Msg("✅ Shop activated successfully")

	// 5. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// HELPERS
// ============================================================

// extractAdminContext extrait AdminContext depuis le contexte HTTP
func extractAdminContext(r *http.Request) (*adminshopusecase.AdminContext, error) {
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())

	ipAddress := parseIP(r.RemoteAddr)

	return &adminshopusecase.AdminContext{
		AdminID:    adminID,
		AdminEmail: "",
		AdminRole:  adminRole,
		IPAddress:  ipAddress,
		UserAgent:  r.UserAgent(),
		RequestID:  r.Header.Get("X-Request-ID"),
	}, nil
}

// parseIP extrait l'IP depuis RemoteAddr (enlève le port)
func parseIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	if strings.HasPrefix(remoteAddr, "[") {
		endBracket := strings.Index(remoteAddr, "]")
		if endBracket > 0 {
			return remoteAddr[1:endBracket]
		}
	}

	lastColon := strings.LastIndex(remoteAddr, ":")
	if lastColon > 0 {
		if strings.Count(remoteAddr, ":") == 1 {
			return remoteAddr[:lastColon]
		}
	}

	return remoteAddr
}

// handleShopError convertit les erreurs usecase en AppError HTTP
func handleShopError(err error) error {
	errMsg := err.Error()

	switch {
	case errors.Is(err, entity.ErrShopAlreadySuspended):
		return utils.NewAppError("SHOP_ALREADY_SUSPENDED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrShopNotSuspended):
		return utils.NewAppError("SHOP_NOT_SUSPENDED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrSuspensionReasonRequired):
		return utils.NewAppError("REASON_REQUIRED", errMsg, http.StatusBadRequest)

	case errMsg == "shop not found":
		return utils.NewAppError("SHOP_NOT_FOUND", errMsg, http.StatusNotFound)
	case errMsg == "invalid shop_id format":
		return utils.NewAppError("INVALID_SHOP_ID", errMsg, http.StatusBadRequest)
	case errMsg == "reason is required":
		return utils.NewAppError("REASON_REQUIRED", errMsg, http.StatusBadRequest)
	case strings.HasPrefix(errMsg, "reason must be at least"):
		return utils.NewAppError("REASON_TOO_SHORT", errMsg, http.StatusBadRequest)
	case strings.HasPrefix(errMsg, "reason must be at most"):
		return utils.NewAppError("REASON_TOO_LONG", errMsg, http.StatusBadRequest)

	default:
		return err
	}
}

// parseIntParam parse un paramètre entier avec valeur par défaut
func parseIntParam(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	var i int
	_, err := fmt.Sscanf(value, "%d", &i)
	if err != nil {
		return defaultValue
	}

	return i
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

func (h *AdminShopHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", middl.ErrorHandler(h.ListShops))
	r.Get("/{id}", middl.ErrorHandler(h.GetShopDetails))
	r.Get("/{id}/health", middl.ErrorHandler(h.GetShopHealth))
	r.Put("/{id}/suspend", middl.ErrorHandler(h.SuspendShop))
	r.Put("/{id}/activate", middl.ErrorHandler(h.ActivateShop))
}
