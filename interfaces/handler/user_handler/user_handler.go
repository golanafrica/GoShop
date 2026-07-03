// interfaces/handler/user_handler/user_handler.go
package userhandler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	userdto "Goshop/application/dto/user_dto"
	userusecase "Goshop/application/usecase/user_usecase"
	"Goshop/config/setupLogging"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type UserHandler struct {
	registerUc   *userusecase.RegisterUsecase
	loginUc      *userusecase.LoginUsecase
	getProfileUc *userusecase.GetProfileUsecase
	//logger       *setupLogging.Logger
}

// NewUserHandler — maintenant reçoit un logger
// NewUserHandler — maintenant reçoit un logger et le passe aux use cases
func NewUserHandler(repo userrepository.UserRepository, logger *setupLogging.Logger) *UserHandler {
	handlerLogger := logger.WithComponent("user_handler")
	return &UserHandler{
		registerUc:   userusecase.NewRegisterUsecase(repo, handlerLogger),
		loginUc:      userusecase.NewLoginUsecase(repo, handlerLogger),
		getProfileUc: userusecase.NewGetProfileUsecase(repo),
		//logger:       handlerLogger,
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
	//logger := setupLogging.FromContext(ctx).WithOperation("register")
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

	logger.Debug().
		Str("raw_body", string(bodyBytes)).
		Int("body_length", len(bodyBytes)).
		Msg("📦 Body HTTP reçu (raw)")

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req userdto.RegisterUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "invalid_json").
			Str("raw_body", string(bodyBytes)).
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

	logger.Debug().
		Str("raw_body", string(bodyBytes)).
		Int("body_length", len(bodyBytes)).
		Msg("📦 Body HTTP reçu (raw)")

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req userdto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("error_type", "invalid_json").
			Str("raw_body", string(bodyBytes)).
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

	// ============================================================
	// 🆕 v4.0.0 : Execute retourne maintenant 3 valeurs
	// ============================================================
	accessToken, refreshToken, err := h.loginUc.Execute(ctx, req.Email, req.Password)
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

	// ============================================================
	// 🆕 v4.0.0 : Retourner les 2 tokens + extraire le rôle
	// ============================================================
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
	//logger := setupLogging.FromContext(ctx).WithOperation("get_profile")
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

// Helpers de masquage (inchangés)
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
