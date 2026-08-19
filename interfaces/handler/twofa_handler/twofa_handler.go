package twofahandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"Goshop/application/metrics"
	twofausecase "Goshop/application/usecase/twofa_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.0 : 2FA HANDLER
// ============================================================

type TwoFAHandler struct {
	setupUC           *twofausecase.Setup2FAUsecase
	verifyEnableUC    *twofausecase.VerifyAndEnable2FAUsecase
	disableUC         *twofausecase.Disable2FAUsecase
	getStatusUC       *twofausecase.Get2FAStatusUsecase
	regenerateCodesUC *twofausecase.RegenerateRecoveryCodesUsecase
}

func NewTwoFAHandler(
	setupUC *twofausecase.Setup2FAUsecase,
	verifyEnableUC *twofausecase.VerifyAndEnable2FAUsecase,
	disableUC *twofausecase.Disable2FAUsecase,
	getStatusUC *twofausecase.Get2FAStatusUsecase,
	regenerateCodesUC *twofausecase.RegenerateRecoveryCodesUsecase,
) *TwoFAHandler {
	return &TwoFAHandler{
		setupUC:           setupUC,
		verifyEnableUC:    verifyEnableUC,
		disableUC:         disableUC,
		getStatusUC:       getStatusUC,
		regenerateCodesUC: regenerateCodesUC,
	}
}

// @Summary Initialiser la configuration 2FA
func (h *TwoFAHandler) Setup2FA(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req twofausecase.Setup2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Msg("🔐 Setup 2FA request")

	response, err := h.setupUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec setup
		metrics.TwoFAOperationTotal.WithLabelValues("setup", "error").Inc()
		metrics.TwoFAOperationDuration.WithLabelValues("setup").Observe(duration)
		metrics.ApplicationErrorsTotal.WithLabelValues("2fa_setup", "2fa_handler").Inc()

		logger.Error().Err(err).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to setup 2FA")

		return handle2FAError(err)
	}

	// 📊 MÉTRIQUES : Succès setup
	metrics.TwoFAOperationTotal.WithLabelValues("setup", "success").Inc()
	metrics.TwoFAOperationDuration.WithLabelValues("setup").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Float64("duration_seconds", duration).
		Msg("✅ 2FA setup completed successfully")

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// @Summary Vérifier et activer la 2FA
func (h *TwoFAHandler) VerifyAndEnable2FA(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req twofausecase.VerifyAndEnable2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	if len(req.Code) != entity.TOTPDigits {
		return utils.NewAppError("INVALID_CODE", "Code must be 6 digits", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Msg("🔐 Verify & Enable 2FA request")

	response, err := h.verifyEnableUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec vérification
		metrics.TwoFAVerificationAttempts.WithLabelValues("error").Inc()
		metrics.TwoFAOperationTotal.WithLabelValues("verify", "error").Inc()
		metrics.TwoFAOperationDuration.WithLabelValues("verify").Observe(duration)

		// Détecter les codes invalides
		if errors.Is(err, entity.Err2FAInvalidCode) {
			metrics.TwoFAInvalidCodes.Inc()
		}

		logger.Error().Err(err).
			Str("admin_id", admin.AdminID).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to verify and enable 2FA")

		return handle2FAError(err)
	}

	// 📊 MÉTRIQUES : Succès vérification
	metrics.TwoFAVerificationAttempts.WithLabelValues("success").Inc()
	metrics.TwoFAOperationTotal.WithLabelValues("verify", "success").Inc()
	metrics.TwoFAOperationDuration.WithLabelValues("verify").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Float64("duration_seconds", duration).
		Msg("✅ 2FA verified and enabled successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Désactiver la 2FA
func (h *TwoFAHandler) Disable2FA(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req twofausecase.Disable2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	logger.Info().
		Str("admin_id", admin.AdminID).
		Bool("is_recovery", req.IsRecovery).
		Msg("🔐 Disable 2FA request")

	response, err := h.disableUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec disable
		metrics.TwoFAOperationTotal.WithLabelValues("disable", "error").Inc()
		metrics.TwoFAOperationDuration.WithLabelValues("disable").Observe(duration)

		// Détecter l'utilisation de codes de récupération
		if req.IsRecovery && errors.Is(err, entity.Err2FARecoveryCodeUsed) {
			metrics.TwoFARecoveryCodesUsed.Inc()
		}

		logger.Error().Err(err).
			Str("admin_id", admin.AdminID).
			Bool("is_recovery", req.IsRecovery).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to disable 2FA")

		return handle2FAError(err)
	}

	// 📊 MÉTRIQUES : Succès disable
	metrics.TwoFAOperationTotal.WithLabelValues("disable", "success").Inc()
	metrics.TwoFAOperationDuration.WithLabelValues("disable").Observe(duration)

	if req.IsRecovery {
		metrics.TwoFARecoveryCodesUsed.Inc()
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Bool("is_recovery", req.IsRecovery).
		Float64("duration_seconds", duration).
		Msg("✅ 2FA disabled successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Obtenir le statut 2FA d'un utilisateur
func (h *TwoFAHandler) Get2FAStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = admin.AdminID
	}

	req := &twofausecase.Get2FAStatusRequest{
		UserID: userID,
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Msg("🔐 Get 2FA status request")

	response, err := h.getStatusUC.Execute(ctx, admin, req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec get status
		metrics.TwoFAOperationTotal.WithLabelValues("get_status", "error").Inc()
		metrics.TwoFAOperationDuration.WithLabelValues("get_status").Observe(duration)

		logger.Error().Err(err).
			Str("target_user_id", userID).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to get 2FA status")

		return handle2FAError(err)
	}

	// 📊 MÉTRIQUES : Succès get status
	metrics.TwoFAOperationTotal.WithLabelValues("get_status", "success").Inc()
	metrics.TwoFAOperationDuration.WithLabelValues("get_status").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", userID).
		Float64("duration_seconds", duration).
		Msg("✅ 2FA status retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Régénérer les codes de récupération 2FA
func (h *TwoFAHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	admin, err := extractAdminContext(r)
	if err != nil {
		return utils.ErrUnauthorized
	}

	var req twofausecase.RegenerateRecoveryCodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return utils.NewAppError("INVALID_PAYLOAD", "Invalid JSON payload", http.StatusBadRequest)
	}
	defer r.Body.Close()

	if len(req.Code) != entity.TOTPDigits {
		return utils.NewAppError("INVALID_CODE", "Code must be 6 digits", http.StatusBadRequest)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Msg("🔐 Regenerate recovery codes request")

	response, err := h.regenerateCodesUC.Execute(ctx, admin, &req)
	duration := time.Since(start).Seconds()

	if err != nil {
		// 📊 MÉTRIQUES : Échec regenerate
		metrics.TwoFAOperationTotal.WithLabelValues("regenerate_codes", "error").Inc()
		metrics.TwoFAOperationDuration.WithLabelValues("regenerate_codes").Observe(duration)

		if errors.Is(err, entity.Err2FAInvalidCode) {
			metrics.TwoFAInvalidCodes.Inc()
		}

		logger.Error().Err(err).
			Str("admin_id", admin.AdminID).
			Float64("duration_seconds", duration).
			Msg("❌ Failed to regenerate recovery codes")

		return handle2FAError(err)
	}

	// 📊 MÉTRIQUES : Succès regenerate
	metrics.TwoFAOperationTotal.WithLabelValues("regenerate_codes", "success").Inc()
	metrics.TwoFAOperationDuration.WithLabelValues("regenerate_codes").Observe(duration)

	logger.Info().
		Str("admin_id", admin.AdminID).
		Float64("duration_seconds", duration).
		Msg("✅ Recovery codes regenerated successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ROUTES REGISTRATION
// ============================================================

func (h *TwoFAHandler) RegisterRoutes(r chi.Router) {
	r.Post("/setup", middl.ErrorHandler(h.Setup2FA))
	r.Post("/verify", middl.ErrorHandler(h.VerifyAndEnable2FA))
	r.Post("/disable", middl.ErrorHandler(h.Disable2FA))
	r.Get("/status", middl.ErrorHandler(h.Get2FAStatus))
	r.Post("/regenerate-codes", middl.ErrorHandler(h.RegenerateRecoveryCodes))
}

// ============================================================
// HELPERS
// ============================================================

func extractAdminContext(r *http.Request) (*twofausecase.AdminContext, error) {
	adminID, ok := utils.UserIDFromContext(r.Context())
	if !ok || adminID == "" {
		return nil, errors.New("admin_id not found in context")
	}

	adminRole, _ := utils.UserRoleFromContext(r.Context())

	return &twofausecase.AdminContext{
		AdminID:   adminID,
		AdminRole: adminRole,
		IPAddress: extractIPWithoutPort(r.RemoteAddr),
		UserAgent: r.UserAgent(),
	}, nil
}

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

func handle2FAError(err error) error {
	errMsg := err.Error()

	switch {
	case errors.Is(err, entity.Err2FANotEnabled):
		return utils.NewAppError("2FA_NOT_ENABLED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.Err2FAAlreadyEnabled):
		return utils.NewAppError("2FA_ALREADY_ENABLED", errMsg, http.StatusConflict)
	case errors.Is(err, entity.Err2FANotSetup):
		return utils.NewAppError("2FA_NOT_SETUP", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.Err2FAInvalidCode):
		return utils.NewAppError("2FA_INVALID_CODE", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.Err2FAAccountLocked):
		return utils.NewAppError("2FA_ACCOUNT_LOCKED", errMsg, http.StatusTooManyRequests)
	case errors.Is(err, entity.Err2FARecoveryCodeUsed):
		return utils.NewAppError("2FA_RECOVERY_CODE_USED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.Err2FARecoveryCodeNotFound):
		return utils.NewAppError("2FA_RECOVERY_CODE_NOT_FOUND", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.Err2FASecretRequired):
		return utils.NewAppError("2FA_SECRET_REQUIRED", errMsg, http.StatusBadRequest)
	case errors.Is(err, entity.Err2FACodeAlreadyUsed):
		return utils.NewAppError("2FA_CODE_ALREADY_USED", errMsg, http.StatusTooManyRequests)

	case errors.Is(err, repository.ErrUser2FANotFound):
		return utils.NewAppError("2FA_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, repository.ErrUser2FAAlreadyExists):
		return utils.NewAppError("2FA_ALREADY_EXISTS", errMsg, http.StatusConflict)
	case errors.Is(err, repository.ErrUser2FAInvalidData):
		return utils.NewAppError("2FA_INVALID_DATA", errMsg, http.StatusBadRequest)

	case errMsg == "user not found":
		return utils.NewAppError("USER_NOT_FOUND", errMsg, http.StatusNotFound)

	default:
		return err
	}
}
