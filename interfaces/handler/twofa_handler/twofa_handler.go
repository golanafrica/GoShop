package twofahandler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

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
// 🆕 v4.4.1 : + Gestion erreur Err2FACodeAlreadyUsed
// ============================================================

// TwoFAHandler gère les endpoints 2FA
type TwoFAHandler struct {
	setupUC           *twofausecase.Setup2FAUsecase
	verifyEnableUC    *twofausecase.VerifyAndEnable2FAUsecase
	disableUC         *twofausecase.Disable2FAUsecase
	getStatusUC       *twofausecase.Get2FAStatusUsecase
	regenerateCodesUC *twofausecase.RegenerateRecoveryCodesUsecase
}

// NewTwoFAHandler crée une nouvelle instance
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

// ============================================================
// ENDPOINT 1 : SETUP 2FA
// ============================================================

func (h *TwoFAHandler) Setup2FA(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.setupUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handle2FAError(err)
	}

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// ============================================================
// ENDPOINT 2 : VERIFY & ENABLE 2FA
// ============================================================

func (h *TwoFAHandler) VerifyAndEnable2FA(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.verifyEnableUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handle2FAError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 3 : DISABLE 2FA
// ============================================================

func (h *TwoFAHandler) Disable2FA(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.disableUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handle2FAError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 4 : GET 2FA STATUS
// ============================================================

func (h *TwoFAHandler) Get2FAStatus(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.getStatusUC.Execute(r.Context(), admin, req)
	if err != nil {
		return handle2FAError(err)
	}

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// ENDPOINT 5 : REGENERATE RECOVERY CODES
// ============================================================

func (h *TwoFAHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	logger := zerolog.Ctx(r.Context())

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

	response, err := h.regenerateCodesUC.Execute(r.Context(), admin, &req)
	if err != nil {
		return handle2FAError(err)
	}

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

// handle2FAError convertit les erreurs usecase en AppError HTTP
func handle2FAError(err error) error {
	errMsg := err.Error()

	switch {
	// Erreurs typées (entity)
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
	// 🆕 v4.4.1 : Protection anti-replay
	case errors.Is(err, entity.Err2FACodeAlreadyUsed):
		return utils.NewAppError("2FA_CODE_ALREADY_USED", errMsg, http.StatusTooManyRequests)

	// Erreurs repository
	case errors.Is(err, repository.ErrUser2FANotFound):
		return utils.NewAppError("2FA_NOT_FOUND", errMsg, http.StatusNotFound)
	case errors.Is(err, repository.ErrUser2FAAlreadyExists):
		return utils.NewAppError("2FA_ALREADY_EXISTS", errMsg, http.StatusConflict)
	case errors.Is(err, repository.ErrUser2FAInvalidData):
		return utils.NewAppError("2FA_INVALID_DATA", errMsg, http.StatusBadRequest)

	// Erreurs string
	case errMsg == "user not found":
		return utils.NewAppError("USER_NOT_FOUND", errMsg, http.StatusNotFound)

	default:
		return err
	}
}
