// interfaces/handler/refresh_handler/refresh_handler.go
package refreshhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type RefreshUseCase interface {
	Execute(ctx context.Context, refreshToken string) (string, string, error)
}

type RefreshHandler struct {
	uc RefreshUseCase
}

func NewRefreshHandler(uc RefreshUseCase) *RefreshHandler {
	return &RefreshHandler{uc: uc}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// @Summary Rafraîchir les tokens d'authentification
// @Description Génère un nouveau couple de tokens (access et refresh) à partir d'un refresh token valide.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body refreshhandler.refreshRequest true "Le refresh token actuel (peut aussi être passé dans le header X-Refresh-Token)"
// @Success 200 {object} map[string]string "Nouveaux tokens d'accès"
// @Failure 400 {object} utils.AppError "Refresh token manquant ou format invalide"
// @Failure 401 {object} utils.AppError "Refresh token invalide ou expiré"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Router /auth/refresh [post]
func (h *RefreshHandler) Refresh(w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()

	// Récupère le logger enriched avec request_id depuis le contexte
	logger := zerolog.Ctx(r.Context()).With().Str("operation", "refresh_token").Logger()

	logger.Debug().Msg("Traitement de la requête refresh token")

	var req refreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() != "EOF" {
			logger.Warn().
				Err(err).
				Str("error_type", "invalid_json").
				Msg("Échec du décodage JSON")

			utils.WriteError(w, http.StatusBadRequest, "Invalid JSON format")
			return nil
		}
		logger.Debug().Msg("Body vide, vérification du header")
	}
	defer r.Body.Close()

	if req.RefreshToken == "" {
		req.RefreshToken = r.Header.Get("X-Refresh-Token")
		if req.RefreshToken != "" {
			logger.Debug().Msg("Refresh token récupéré depuis l'en-tête")
		}
	}

	if req.RefreshToken == "" {
		logger.Warn().
			Str("error_type", "missing_token").
			Msg("Refresh token manquant")

		utils.WriteError(w, http.StatusBadRequest, "Refresh token is required in body or X-Refresh-Token header")
		return nil
	}

	logger.Debug().
		Int("token_length", len(req.RefreshToken)).
		Msg("Exécution du usecase refresh")

	// Appel avec le contexte
	access, refresh, err := h.uc.Execute(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleUseCaseError(w, logger, err)
		return nil
	}

	resp := map[string]string{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    "3600",
	}

	duration := time.Since(startTime)
	logger.Info().
		Int("new_access_token_length", len(access)).
		Int("new_refresh_token_length", len(refresh)).
		Dur("duration_ms", duration).
		Msg("Refresh token traité avec succès")

	utils.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// handleUseCaseError — utilise zerolog.Logger
func (h *RefreshHandler) handleUseCaseError(w http.ResponseWriter, logger zerolog.Logger, err error) {
	logger.Warn().
		Err(err).
		Msg("Échec du refresh token")

	utils.WriteError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
}
