// interfaces/handler/refresh_handler/refresh_handler.go
// interfaces/handler/refresh_handler/refresh_handler.go
package refreshhandler

import (
	"context" // ← AJOUTÉ
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog" // ← AJOUTÉ

	"Goshop/interfaces/utils"
)

type RefreshUseCase interface {
	// ✅ Ajout du contexte
	Execute(ctx context.Context, refreshToken string) (string, string, error)
}

type RefreshHandler struct {
	uc RefreshUseCase
	// logger *setupLogging.Logger // ← SUPPRIMÉ (inutile avec zerolog.Ctx)
}

func NewRefreshHandler(uc RefreshUseCase) *RefreshHandler {
	// Le logger est maintenant injecté via le contexte → pas besoin de le stocker
	return &RefreshHandler{uc: uc}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *RefreshHandler) Refresh(w http.ResponseWriter, r *http.Request) error {
	startTime := time.Now()

	// ✅ Récupère le logger enriched avec request_id depuis le contexte
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

	// ✅ Appel avec le contexte
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
