package adminshophandler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
//
// 🎯 Objectif :
//   Exposer les endpoints HTTP pour la gestion admin des shops.
//   Inspiré d'Amazon Seller Central > Admin Dashboard.
//
// 📋 Endpoints :
//   - GET    /api/admin/shops              → Liste cross-tenant
//   - GET    /api/admin/shops/{id}         → Détails complets
//   - GET    /api/admin/shops/{id}/health  → Health Score détaillé
//   - PUT    /api/admin/shops/{id}/suspend → Suspendre
//   - PUT    /api/admin/shops/{id}/activate → Réactiver
//
// 🔐 Sécurité :
//   - Protégé par middleware RBAC (super_admin, admin)
//   - AdminContext extrait automatiquement du JWT
//   - Audit trail automatique pour toutes les actions
//
// 🐛 v4.2.1 : Correction parsing IP pour audit trail
//   - parseIP() extrait l'IP depuis RemoteAddr (enlève le port)
//   - Support IPv4 (192.168.1.1:8080 → 192.168.1.1)
//   - Support IPv6 ([::1]:8080 → ::1)
//
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

// ListShops gère GET /api/admin/shops
func (h *AdminShopHandler) ListShops(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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
	response, err := h.listUC.Execute(r.Context(), admin, req)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur liste shops")
		return err
	}

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// GetShopDetails gère GET /api/admin/shops/{id}
func (h *AdminShopHandler) GetShopDetails(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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
	response, err := h.detailsUC.Execute(r.Context(), admin, req)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération détails")
		return handleShopError(err)
	}

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// GetShopHealth gère GET /api/admin/shops/{id}/health
func (h *AdminShopHandler) GetShopHealth(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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
	response, err := h.healthUC.Execute(r.Context(), admin, req)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur récupération health score")
		return handleShopError(err)
	}

	// 4. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// SuspendShopRequest représente la requête de suspension
type SuspendShopRequest struct {
	Reason string `json:"reason"` // Obligatoire, min 10 caractères
}

// SuspendShop gère PUT /api/admin/shops/{id}/suspend
func (h *AdminShopHandler) SuspendShop(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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
	response, err := h.suspendUC.Execute(r.Context(), admin, req)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur suspension shop")
		return handleShopError(err)
	}

	// 5. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ActivateShopRequest représente la requête de réactivation
type ActivateShopRequest struct {
	Reason *string `json:"reason,omitempty"` // Optionnel
}

// ActivateShop gère PUT /api/admin/shops/{id}/activate
func (h *AdminShopHandler) ActivateShop(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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
	response, err := h.activateUC.Execute(r.Context(), admin, req)
	if err != nil {
		logger.Error().Err(err).Msg("❌ Erreur réactivation shop")
		return handleShopError(err)
	}

	// 5. Retourner la réponse
	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// HELPERS
// ============================================================

// extractAdminContext extrait AdminContext depuis le contexte HTTP
func extractAdminContext(r *http.Request) (*adminshopusecase.AdminContext, error) {
	// Extraire depuis le contexte (injecté par middleware)
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())

	// 🆕 v4.2.1 : Parser l'IP pour enlever le port (format INET PostgreSQL)
	ipAddress := parseIP(r.RemoteAddr)

	return &adminshopusecase.AdminContext{
		AdminID:    adminID,
		AdminEmail: "", // Non disponible dans le contexte JWT
		AdminRole:  adminRole,
		IPAddress:  ipAddress,
		UserAgent:  r.UserAgent(),
		RequestID:  r.Header.Get("X-Request-ID"),
	}, nil
}

// parseIP extrait l'IP depuis RemoteAddr (enlève le port)
// 🆕 v4.2.1 : Correction pour audit trail PostgreSQL INET
//
// Exemples :
//   - "192.168.1.1:8080"      → "192.168.1.1"
//   - "[::1]:8080"            → "::1"
//   - "127.0.0.1:12345"       → "127.0.0.1"
//   - "[2001:db8::1]:443"     → "2001:db8::1"
//   - "::1"                   → "::1" (IPv6 sans port)
//   - ""                      → "" (vide)
func parseIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	// Cas IPv6 avec crochets : [::1]:8080 ou [2001:db8::1]:443
	if strings.HasPrefix(remoteAddr, "[") {
		endBracket := strings.Index(remoteAddr, "]")
		if endBracket > 0 {
			return remoteAddr[1:endBracket] // Enlève [ et ]
		}
	}

	// Cas IPv4 avec port : 192.168.1.1:8080
	lastColon := strings.LastIndex(remoteAddr, ":")
	if lastColon > 0 {
		// Vérifier que ce n'est pas une IPv6 sans port (::1)
		// Une IPv6 sans port a plusieurs ":" mais pas de crochets
		if strings.Count(remoteAddr, ":") == 1 {
			return remoteAddr[:lastColon]
		}
	}

	return remoteAddr
}

// handleShopError convertit les erreurs usecase en AppError HTTP
// 🆕 v4.2.1 : Support erreurs string (validation raison)
func handleShopError(err error) error {
	errMsg := err.Error()

	switch {
	// Erreurs typées (entity)
	case errors.Is(err, entity.ErrShopAlreadySuspended):
		return utils.NewAppError("SHOP_ALREADY_SUSPENDED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrShopNotSuspended):
		return utils.NewAppError("SHOP_NOT_SUSPENDED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrSuspensionReasonRequired):
		return utils.NewAppError("REASON_REQUIRED", errMsg, http.StatusBadRequest)

	// 🆕 v4.2.1 : Erreurs string (validation)
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

// RegisterRoutes enregistre toutes les routes admin shops
// Les handlers sont enveloppés avec middl.ErrorHandler() pour gérer les erreurs
func (h *AdminShopHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", middl.ErrorHandler(h.ListShops))
	r.Get("/{id}", middl.ErrorHandler(h.GetShopDetails))
	r.Get("/{id}/health", middl.ErrorHandler(h.GetShopHealth))
	r.Put("/{id}/suspend", middl.ErrorHandler(h.SuspendShop))
	r.Put("/{id}/activate", middl.ErrorHandler(h.ActivateShop))
}
