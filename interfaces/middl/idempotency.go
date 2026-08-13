package middl

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

const (
	// IdempotencyKeyHeader est le nom du header pour la clé d'idempotence
	IdempotencyKeyHeader = "Idempotency-Key"

	// DefaultTTL est la durée de vie par défaut des clés (24h)
	DefaultTTL = 24 * time.Hour
)

// IdempotencyConfig configuration du middleware
type IdempotencyConfig struct {
	IdempotencyRepo repository.IdempotencyRepository
	TTL             time.Duration // Optionnel, défaut 24h
}

// responseRecorder capture la réponse HTTP pour la stocker
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	headers    map[string]string
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           bytes.NewBuffer(nil),
		headers:        make(map[string]string),
	}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

// IdempotencyMiddleware crée un middleware d'idempotence pour les requêtes POST
func IdempotencyMiddleware(config IdempotencyConfig) func(http.Handler) http.Handler {
	if config.TTL == 0 {
		config.TTL = DefaultTTL
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())

			// 1. Extraire la clé d'idempotence du header
			idempotencyKey := r.Header.Get(IdempotencyKeyHeader)
			if idempotencyKey == "" {
				// Pas de clé = pas d'idempotence, continuer normalement
				next.ServeHTTP(w, r)
				return
			}

			// 2. Extraire l'ID utilisateur du contexte (doit être authentifié)
			// Utilise le helper utils.GetUserID() qui extrait depuis le contexte
			userID, ok := utils.GetUserID(r.Context())
			if !ok || userID == "" {
				logger.Warn().
					Str("idempotency_key", idempotencyKey).
					Msg("⚠️ Idempotency key fournie mais utilisateur non authentifié")
				utils.WriteAppError(w, utils.NewAppError(
					"IDEMPOTENCY_UNAUTHORIZED",
					"Idempotency-Key header requires authentication",
					http.StatusUnauthorized,
				))
				return
			}

			// 3. Lire le body de la requête pour calculer le hash
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Error().Err(err).Msg("❌ Impossible de lire le body de la requête")
				utils.WriteAppError(w, utils.ErrInternalServer)
				return
			}
			// Restaurer le body pour les handlers suivants
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// 4. Vérifier si la clé existe déjà
			existingKey, err := config.IdempotencyRepo.FindByKey(r.Context(), idempotencyKey)
			if err != nil {
				logger.Error().Err(err).Msg("❌ Erreur recherche clé d'idempotence")
				utils.WriteAppError(w, utils.ErrInternalServer)
				return
			}

			// 5. Cas : Clé existe déjà
			if existingKey != nil {
				// Vérifier si expirée
				if existingKey.IsExpired() {
					// Clé expirée, supprimer et continuer
					_ = config.IdempotencyRepo.DeleteByKey(r.Context(), idempotencyKey)
					next.ServeHTTP(w, r)
					return
				}

				// Vérifier si même requête (hash match)
				if !existingKey.IsSameRequest(bodyBytes) {
					// Conflit : même clé mais requête différente
					logger.Warn().
						Str("idempotency_key", idempotencyKey).
						Str("user_id", userID).
						Msg("⚠️ Conflit d'idempotence : même clé, requête différente")
					utils.WriteAppError(w, utils.NewAppError(
						"IDEMPOTENCY_CONFLICT",
						"Idempotency-Key already used with different request body. Use a new key.",
						http.StatusConflict,
					))
					return
				}

				// Même requête = renvoyer la réponse cachée
				logger.Info().
					Str("idempotency_key", idempotencyKey).
					Int("cached_status", existingKey.ResponseStatus).
					Msg("✅ Réponse cachée renvoyée (idempotence)")

				// Restaurer les headers
				for k, v := range existingKey.ResponseHeaders {
					w.Header().Set(k, v)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotent-Replayed", "true")
				w.WriteHeader(existingKey.ResponseStatus)

				// Écrire le body
				responseBody, _ := json.Marshal(existingKey.ResponseBody)
				w.Write(responseBody)
				return
			}

			// 6. Cas : Nouvelle clé, créer l'entrée
			endpoint := r.Method + " " + r.URL.Path
			newKey := entity.NewIdempotencyKey(idempotencyKey, userID, endpoint, bodyBytes, config.TTL)

			if err := config.IdempotencyRepo.Create(r.Context(), newKey); err != nil {
				logger.Error().Err(err).Msg("❌ Erreur création clé d'idempotence")
				// Continuer sans idempotence (dégradation gracieuse)
				next.ServeHTTP(w, r)
				return
			}

			// 7. Capturer la réponse du handler
			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r)

			// 8. Stocker la réponse pour les futures requêtes
			responseHeaders := make(map[string]string)
			for k := range recorder.Header() {
				responseHeaders[k] = recorder.Header().Get(k)
			}

			var responseBody map[string]interface{}
			if err := json.Unmarshal(recorder.body.Bytes(), &responseBody); err != nil {
				// Si pas JSON, stocker vide
				responseBody = make(map[string]interface{})
			}

			if err := config.IdempotencyRepo.UpdateResponse(
				r.Context(),
				idempotencyKey,
				recorder.statusCode,
				responseHeaders,
				responseBody,
			); err != nil {
				logger.Warn().Err(err).Msg("⚠️ Impossible de stocker la réponse (idempotence dégradée)")
			}

			logger.Debug().
				Str("idempotency_key", idempotencyKey).
				Int("status", recorder.statusCode).
				Msg("✅ Clé d'idempotence créée et réponse stockée")
		})
	}
}
