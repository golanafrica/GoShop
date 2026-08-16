package apikeyhandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Goshop/application/metrics"
	apikeyusecase "Goshop/application/usecase/apikey_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.3 : API KEY MANAGEMENT HANDLER
// ============================================================

// APIKeyHandler gère les endpoints de gestion des clés API
type APIKeyHandler struct {
	createUC    *apikeyusecase.CreateAPIKeyUsecase
	listUC      *apikeyusecase.ListAPIKeysUsecase
	revokeUC    *apikeyusecase.RevokeAPIKeyUsecase
	revokeAllUC *apikeyusecase.RevokeAllAPIKeysUsecase
	getStatsUC  *apikeyusecase.GetAPIKeyStatsUsecase
}

// NewAPIKeyHandler crée une nouvelle instance
func NewAPIKeyHandler(
	createUC *apikeyusecase.CreateAPIKeyUsecase,
	listUC *apikeyusecase.ListAPIKeysUsecase,
	revokeUC *apikeyusecase.RevokeAPIKeyUsecase,
	revokeAllUC *apikeyusecase.RevokeAllAPIKeysUsecase,
	getStatsUC *apikeyusecase.GetAPIKeyStatsUsecase,
) *APIKeyHandler {
	return &APIKeyHandler{
		createUC:    createUC,
		listUC:      listUC,
		revokeUC:    revokeUC,
		revokeAllUC: revokeAllUC,
		getStatsUC:  getStatsUC,
	}
}

// ============================================================
// ENDPOINT 1 : CREATE API KEY
// ============================================================

// @Summary Créer une nouvelle clé API
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req apikeyusecase.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	// Validation du nom
	if req.Name == "" {
		return utils.NewAppError("MISSING_FIELD", "name is required", http.StatusBadRequest)
	}

	// Validation des scopes
	if len(req.Scopes) == 0 {
		return utils.NewAppError("MISSING_FIELD", "at least one scope is required", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("key_name", req.Name).
		Int("scopes_count", len(req.Scopes)).
		Bool("is_test", req.IsTest).
		Msg("🔑 Create API key request")

	response, err := h.createUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec création
		isTestStr := "false"
		if req.IsTest {
			isTestStr = "true"
		}
		metrics.APIKeyCreateTotal.WithLabelValues("error", isTestStr).Inc()
		metrics.APIKeyOperationDuration.WithLabelValues("create").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("apikey_create", "apikey_handler").Inc()

		logger.Error().Err(err).
			Str("admin_id", admin.AdminID).
			Str("key_name", req.Name).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to create API key")

		return handleAPIKeyError(err)
	}

	// 📊 MÉTRIQUES : Succès création
	isTestStr := "false"
	if req.IsTest {
		isTestStr = "true"
	}
	metrics.APIKeyCreateTotal.WithLabelValues("success", isTestStr).Inc()
	metrics.APIKeyOperationDuration.WithLabelValues("create").Observe(duration)
	metrics.APIKeyScopesCount.Observe(float64(len(req.Scopes)))

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("key_name", req.Name).
		Int("scopes_count", len(req.Scopes)).
		Bool("is_test", req.IsTest).
		Float64("duration_seconds", duration).
		Msg("✅ API key created successfully")

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// ============================================================
// ENDPOINT 2 : LIST API KEYS
// ============================================================

// @Summary Lister les clés API
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	// Paramètres optionnels
	userID := r.URL.Query().Get("user_id")
	activeOnlyStr := r.URL.Query().Get("active_only")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	activeOnly := activeOnlyStr == "true"
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	req := &apikeyusecase.ListAPIKeysRequest{
		UserID:     userID,
		ActiveOnly: activeOnly,
		Limit:      limit,
		Offset:     offset,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Bool("active_only", activeOnly).
		Msg("📋 List API keys request")

	response, err := h.listUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec listing
		activeOnlyStrMetric := "false"
		if activeOnly {
			activeOnlyStrMetric = "true"
		}
		metrics.APIKeyListTotal.WithLabelValues("error", activeOnlyStrMetric).Inc()
		metrics.APIKeyOperationDuration.WithLabelValues("list").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("apikey_list", "apikey_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to list API keys")

		return handleAPIKeyError(err)
	}

	// 📊 MÉTRIQUES : Succès listing
	activeOnlyStrMetric := "false"
	if activeOnly {
		activeOnlyStrMetric = "true"
	}
	metrics.APIKeyListTotal.WithLabelValues("success", activeOnlyStrMetric).Inc()
	metrics.APIKeyOperationDuration.WithLabelValues("list").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Bool("active_only", activeOnly).
		Float64("duration_seconds", duration).
		Msg("✅ API keys listed successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 3 : REVOKE API KEY
// ============================================================

// @Summary Révoquer une clé API
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req apikeyusecase.RevokeAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	// Validation
	if req.APIKeyID == "" {
		return utils.NewAppError("MISSING_FIELD", "api_key_id is required", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("api_key_id", req.APIKeyID).
		Str("reason", req.Reason).
		Msg("🔑 Revoke API key request")

	response, err := h.revokeUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec révocation
		metrics.APIKeyRevokeTotal.WithLabelValues("error", "single").Inc()
		metrics.APIKeyOperationDuration.WithLabelValues("revoke").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("apikey_revoke", "apikey_handler").Inc()

		logger.Error().Err(err).
			Str("admin_id", admin.AdminID).
			Str("api_key_id", req.APIKeyID).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to revoke API key")

		return handleAPIKeyError(err)
	}

	// 📊 MÉTRIQUES : Succès révocation
	metrics.APIKeyRevokeTotal.WithLabelValues("success", "single").Inc()
	metrics.APIKeyOperationDuration.WithLabelValues("revoke").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("api_key_id", req.APIKeyID).
		Float64("duration_seconds", duration).
		Msg("✅ API key revoked successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 4 : REVOKE ALL API KEYS
// ============================================================

// @Summary Révoquer toutes les clés API d'un utilisateur
func (h *APIKeyHandler) RevokeAllAPIKeys(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req apikeyusecase.RevokeAllAPIKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body optionnel, continuer avec valeurs par défaut
		req = apikeyusecase.RevokeAllAPIKeysRequest{}
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Str("reason", req.Reason).
		Msg("🔑 Revoke all API keys request")

	response, err := h.revokeAllUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec révocation bulk
		metrics.APIKeyRevokeTotal.WithLabelValues("error", "bulk").Inc()
		metrics.APIKeyOperationDuration.WithLabelValues("revoke_all").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("apikey_revoke_all", "apikey_handler").Inc()

		logger.Error().Err(err).
			Str("admin_id", admin.AdminID).
			Str("target_user_id", req.UserID).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to revoke all API keys")

		return handleAPIKeyError(err)
	}

	// 📊 MÉTRIQUES : Succès révocation bulk
	metrics.APIKeyRevokeTotal.WithLabelValues("success", "bulk").Inc()
	metrics.APIKeyOperationDuration.WithLabelValues("revoke_all").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Float64("duration_seconds", duration).
		Msg("✅ All API keys revoked successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 5 : GET STATISTICS
// ============================================================

// @Summary Obtenir les statistiques des clés API
func (h *APIKeyHandler) GetStatistics(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	userID := r.URL.Query().Get("user_id")

	req := &apikeyusecase.GetAPIKeyStatsRequest{
		UserID: userID,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Msg("📊 Get API key statistics request")

	response, err := h.getStatsUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec stats
		metrics.APIKeyStatsTotal.WithLabelValues("error").Inc()
		metrics.APIKeyOperationDuration.WithLabelValues("get_stats").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("apikey_stats", "apikey_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to get API key statistics")

		return handleAPIKeyError(err)
	}

	// 📊 MÉTRIQUES : Succès stats
	metrics.APIKeyStatsTotal.WithLabelValues("success").Inc()
	metrics.APIKeyOperationDuration.WithLabelValues("get_stats").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Float64("duration_seconds", duration).
		Msg("✅ API key statistics retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

// RegisterRoutes enregistre toutes les routes de gestion des clés API
func (h *APIKeyHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", middl.ErrorHandler(h.CreateAPIKey))
	r.Get("/", middl.ErrorHandler(h.ListAPIKeys))
	r.Post("/revoke", middl.ErrorHandler(h.RevokeAPIKey))
	r.Post("/revoke-all", middl.ErrorHandler(h.RevokeAllAPIKeys))
	r.Get("/stats", middl.ErrorHandler(h.GetStatistics))
}

// ============================================================
// HELPERS
// ============================================================

// extractAdminContext extrait le contexte admin depuis la requête
func extractAdminContext(r *http.Request) (*apikeyusecase.AdminContext, error) {
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())
	adminEmail, _ := utils.UserEmailFromContext(r.Context())

	return &apikeyusecase.AdminContext{
		AdminID:    adminID,
		AdminEmail: adminEmail,
		AdminRole:  adminRole,
		IPAddress:  extractIPWithoutPort(r.RemoteAddr),
		UserAgent:  r.UserAgent(),
	}, nil
}

// extractIPWithoutPort extrait l'IP sans le port
func extractIPWithoutPort(addr string) string {
	if addr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}

	return host
}

// handleAPIKeyError convertit les erreurs usecase en AppError HTTP
func handleAPIKeyError(err error) error {
	errMsg := err.Error()

	switch {
	// Erreurs entité
	case errors.Is(err, entity.ErrAPIKeyNotFound):
		return utils.NewAppError("API_KEY_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, entity.ErrAPIKeyExpired):
		return utils.NewAppError("API_KEY_EXPIRED", errMsg, http.StatusGone)
	case errors.Is(err, entity.ErrAPIKeyRevoked):
		return utils.NewAppError("API_KEY_REVOKED", errMsg, http.StatusGone)
	case errors.Is(err, entity.ErrAPIKeyInvalid):
		return utils.NewAppError("API_KEY_INVALID", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrAPIKeyInsufficientScope):
		return utils.NewAppError("API_KEY_INSUFFICIENT_SCOPE", errMsg, http.StatusForbidden)
	case errors.Is(err, entity.ErrAPIKeyNameRequired):
		return utils.NewAppError("API_KEY_NAME_REQUIRED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.ErrAPIKeyAlreadyRevoked):
		return utils.NewAppError("API_KEY_ALREADY_REVOKED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.ErrAPIKeyInvalidPrefix):
		return utils.NewAppError("API_KEY_INVALID_PREFIX", errMsg, http.StatusBadRequest)

	// Erreurs repository
	case errors.Is(err, repository.ErrAPIKeyNotFound):
		return utils.NewAppError("API_KEY_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, repository.ErrAPIKeyAlreadyExists):
		return utils.NewAppError("API_KEY_ALREADY_EXISTS", errMsg, http.StatusConflict)
	case errors.Is(err, repository.ErrAPIKeyInvalidData):
		return utils.NewAppError("API_KEY_INVALID_DATA", errMsg, http.StatusBadRequest)

	// Erreurs de permissions (strings)
	case errMsg == "insufficient permissions: can only create own API keys":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: can only view own API keys":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: can only revoke own API keys":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "insufficient permissions: admin role required":
		return utils.NewAppError("PERMISSION_DENIED", errMsg, http.StatusForbidden)
	case errMsg == "API key is already revoked":
		return utils.NewAppError("API_KEY_ALREADY_REVOKED", errMsg, http.StatusConflict)
	case errMsg == "API key not found":
		return utils.NewAppError("API_KEY_NOT_FOUND", errMsg, http.StatusNotFound)
	case errMsg == "name is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)
	case errMsg == "at least one scope is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)
	case errMsg == "api_key_id is required":
		return utils.NewAppError("MISSING_FIELD", errMsg, http.StatusBadRequest)
	case errMsg == "maximum API keys limit reached (10 active keys per user)":
		return utils.NewAppError("API_KEY_LIMIT_REACHED", errMsg, http.StatusTooManyRequests)

	default:
		// Vérifier si c'est une erreur de scope invalide
		if strings.HasPrefix(errMsg, "invalid scope:") {
			return utils.NewAppError("INVALID_SCOPE", errMsg, http.StatusBadRequest)
		}
		return err
	}
}

// strings est importé via le package standard
var _ = strings.HasPrefix
