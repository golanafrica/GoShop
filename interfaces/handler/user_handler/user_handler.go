// interfaces/handler/user_handler/user_handler.go
package userhandler

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	userdto "Goshop/application/dto/user_dto"
	userusecase "Goshop/application/usecase/user_usecase"
	"Goshop/config/setupLogging"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/domain/service"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.0.0 : UserHandler avec support JWT + rôle
// 🆕 v4.4.2 : + Intégration Session Management + Logout
// 🆕 v4.4.21 : + Rate Limiting pour le login
// ============================================================

type UserHandler struct {
	registerUc   *userusecase.RegisterUsecase
	loginUc      *userusecase.LoginUsecase
	getProfileUc *userusecase.GetProfileUsecase
	sessionRepo  repository.UserSessionRepository // 🆕 v4.4.2
}

// 🆕 v4.4.21 : Nouveau constructeur avec rateLimiter
func NewUserHandler(
	repo userrepository.UserRepository,
	sessionRepo repository.UserSessionRepository, // 🆕 v4.4.2
	rateLimiter service.LoginRateLimiter, // 🆕 v4.4.21
	logger *setupLogging.Logger,
) *UserHandler {
	handlerLogger := logger.WithComponent("user_handler")
	return &UserHandler{
		registerUc:   userusecase.NewRegisterUsecase(repo, handlerLogger),
		loginUc:      userusecase.NewLoginUsecase(repo, sessionRepo, rateLimiter, handlerLogger), // 🆕 v4.4.21
		getProfileUc: userusecase.NewGetProfileUsecase(repo),
		sessionRepo:  sessionRepo,
	}
}

// -----------------------
// REGISTER
// -----------------------

// @Summary User Registration
// @Description Register a new user account
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body userdto.RegisterUserRequest true "User registration data"
// @Success 201 {object} map[string]string "{'message': 'user registered', 'user_id': 'uuid'}"
// @Failure 400 {object} utils.AppError "Invalid request payload"
// @Failure 422 {object} utils.AppError "Validation failed"
// @Failure 500 {object} utils.AppError "Internal server error"
// @Router /register [post]
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("📝 Début inscription utilisateur")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "read_body_error").
			Msg("❌ Impossible de lire le body HTTP")
		return utils.ErrInvalidPayload
	}

	// 🛡️ SÉCURITÉ : Ne jamais logger le raw_body brut (contient le mot de passe en clair)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req userdto.RegisterUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "invalid_json").
			Msg("❌ Échec décodage JSON inscription")
		return utils.ErrInvalidPayload
	}

	logger.Debug().
		Str("decoded_email", req.Email).
		Int("decoded_password_length", len(req.Password)).
		Msg("✅ JSON décodé avec succès")

	if err := req.Validate(); err != nil {
		logger.Warn().
			Err(err).
			Str("error_type", "validation_error").
			Str("email", req.Email).
			Int("password_length", len(req.Password)).
			Msg("❌ Validation inscription échouée")
		return utils.ErrValidationFailed
	}

	logger.Info().
		Str("user_email", req.Email).
		Msg("✅ Validation réussie, tentative création utilisateur")

	user, err := h.registerUc.Execute(ctx, req.Email, req.Password)
	if err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "usecase_error").
			Str("user_email", req.Email).
			Msg("❌ Échec création utilisateur")
		return err
	}

	logger.Info().
		Str("user_id", user.ID).
		Str("user_email", user.Email).
		Msg("🎉 Utilisateur créé avec succès")

	utils.WriteJSON(w, http.StatusCreated, map[string]string{
		"message": "user registered",
		"user_id": user.ID,
	})
	return nil
}

// -----------------------
// LOGIN
// -----------------------

// @Summary User Login
// @Description Authenticate user and return JWT tokens (access + refresh)
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body userdto.LoginRequest true "Login credentials"
// @Success 200 {object} map[string]string "{'access_token': 'jwt', 'refresh_token': 'jwt', 'role': 'merchant'}"
// @Failure 400 {object} utils.AppError "Invalid request payload"
// @Failure 401 {object} utils.AppError "Invalid credentials"
// @Failure 429 {object} utils.AppError "Too many attempts"
// @Failure 500 {object} utils.AppError "Internal server error"
// @Router /login [post]
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("🔐 Tentative de connexion")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "read_body_error").
			Msg("❌ Impossible de lire le body HTTP")
		return utils.ErrInvalidPayload
	}

	// 🛡️ SÉCURITÉ : Ne jamais logger le raw_body brut (contient le mot de passe en clair)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req userdto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "invalid_json").
			Msg("❌ Échec décodage JSON connexion")
		return utils.ErrInvalidPayload
	}

	logger.Debug().
		Str("decoded_email", req.Email).
		Int("decoded_password_length", len(req.Password)).
		Msg("✅ JSON décodé avec succès")

	logger.Debug().
		Str("user_email", req.Email).
		Bool("has_password", req.Password != "").
		Msg("🔑 Tentative connexion reçue")

	logger.Info().Str("user_email", req.Email).Msg("🔑 Authentification en cours")

	// 🆕 v4.4.2 : Utiliser ExecuteWithContext avec IP + UserAgent
	ipAddress := extractIPWithoutPort(r.RemoteAddr)
	userAgent := r.UserAgent()

	accessToken, refreshToken, err := h.loginUc.ExecuteWithContext(
		ctx, req.Email, req.Password, ipAddress, userAgent,
	)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("error_type", "auth_failed").
			Str("user_email", req.Email).
			Msg("❌ Échec authentification")
		return err
	}

	logger.Info().
		Str("user_email", req.Email).
		Int("access_token_length", len(accessToken)).
		Int("refresh_token_length", len(refreshToken)).
		Msg("✅ Connexion réussie, tokens générés")

	// Extraire le rôle depuis l'access token pour le retourner au client
	role := "merchant" // Valeur par défaut
	if claims, err := utils.ValidateToken(accessToken); err == nil {
		if r, ok := claims["role"].(string); ok && r != "" {
			role = r
		}
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token":         accessToken, // 🔄 Rétrocompatibilité (ancien nom)
		"role":          role,        // 🆕 v4.0.0 : Rôle pour le frontend
		"token_type":    "Bearer",
		"expires_in":    900, // 15 minutes en secondes
	})
	return nil
}

// -----------------------
// ME (PROFILE)
// -----------------------

// @Summary Get User Profile
// @Description Get current authenticated user profile
// @Tags Authentication
// @Produce json
// @Success 200 {object} userdto.MeResponse
// @Failure 401 {object} utils.AppError "Unauthorized"
// @Failure 500 {object} utils.AppError "Internal server error"
// @Security ApiKeyAuth
// @Router /auth/me [get]
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	logger.Info().Msg("👤 Récupération profil utilisateur")

	userID, ok := utils.GetUserID(ctx)
	if !ok || userID == "" {
		logger.Warn().
			Str("error_type", "missing_user_id").
			Msg("❌ UserID manquant dans le contexte")
		return utils.ErrUnauthorized
	}

	maskedID := maskUserID(userID)
	logger.Debug().
		Str("user_id", maskedID).
		Msg("✅ UserID extrait du contexte")

	logger.Info().
		Str("user_id", maskedID).
		Msg("📊 Récupération données profil")

	user, err := h.getProfileUc.Execute(userID)
	if err != nil {
		logger.Error().
			Err(err).
			Str("user_id", maskedID).
			Str("error_type", "profile_not_found").
			Msg("❌ Profil utilisateur non trouvé")
		return err
	}

	response := userdto.MeResponse{
		ID:    userID,
		Email: maskEmail(user.Email),
	}

	logger.Info().
		Str("user_id", maskedID).
		Str("user_email", maskEmail(user.Email)).
		Msg("✅ Profil utilisateur retourné")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// ============================================================
// 🆕 v4.4.2 : LOGOUT - Révoque la session courante
// ============================================================

// @Summary User Logout
// @Description Revoke current session and invalidate token
// @Tags Authentication
// @Produce json
// @Success 200 {object} map[string]string "{'message': 'logged out successfully'}"
// @Failure 401 {object} utils.AppError "Unauthorized"
// @Failure 500 {object} utils.AppError "Internal server error"
// @Security ApiKeyAuth
// @Router /logout [post]
func (h *UserHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	logger := zerolog.Ctx(ctx)

	// Extraire les infos du contexte
	userID, _ := utils.UserIDFromContext(ctx)
	sessionID, _ := utils.SessionIDFromContext(ctx)

	logger.Info().
		Str("user_id", userID).
		Str("session_id", sessionID).
		Msg("🔐 Déconnexion utilisateur")

	// Si pas de session ID, on retourne quand même un succès
	if sessionID == "" {
		logger.Warn().
			Str("user_id", userID).
			Msg("⚠️ Session ID manquant - déconnexion partielle")
		utils.WriteJSON(w, http.StatusOK, map[string]string{
			"message": "logged out (session ID not found)",
		})
		return nil
	}

	// Révoquer la session
	if h.sessionRepo != nil {
		if err := h.sessionRepo.RevokeSession(ctx, sessionID, userID); err != nil {
			logger.Error().
				Err(err).
				Str("session_id", sessionID).
				Str("user_id", userID).
				Msg("❌ Erreur révocation session")
			return err
		}

		logger.Info().
			Str("user_id", userID).
			Str("session_id", sessionID).
			Msg("✅ Session révoquée avec succès")
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message":    "logged out successfully",
		"session_id": sessionID,
	})
	return nil
}

// ============================================================
// HELPERS
// ============================================================

func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "invalid_email"
	}
	localPart := parts[0]
	domain := parts[1]
	if len(localPart) > 3 {
		return localPart[:3] + "***@" + domain
	}
	return localPart + "***@" + domain
}

func maskUserID(userID string) string {
	if len(userID) <= 8 {
		return userID
	}
	return userID[:4] + "..." + userID[len(userID)-4:]
}

// 🆕 v4.4.2 : Extraction IP sans port
// Gère les formats : "192.168.1.1:8080", "[::1]:8080", "192.168.1.1"
func extractIPWithoutPort(addr string) string {
	if addr == "" {
		return ""
	}

	// Essayer de séparer host et port avec net.SplitHostPort
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Si erreur, retourner l'adresse telle quelle (pas de port)
		return addr
	}

	return host
}
