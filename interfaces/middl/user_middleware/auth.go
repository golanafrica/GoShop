package middleware

import (
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.2 : AuthMiddleware avec vérification de session
// ============================================================

// AuthMiddlewareConfig permet d'injecter un faux validateur en tests.
type AuthMiddlewareConfig struct {
	JWTValidator utils.JWTValidator
	SessionRepo  repository.UserSessionRepository // 🆕 v4.4.2
}

// NewAuthMiddleware crée un middleware propre et testable.
func NewAuthMiddleware(config ...AuthMiddlewareConfig) func(http.Handler) http.Handler {
	var validator utils.JWTValidator
	var sessionRepo repository.UserSessionRepository // 🆕 v4.4.2

	// Choix entre validateur custom (tests) ou celui de utils
	if len(config) > 0 {
		if config[0].JWTValidator != nil {
			validator = config[0].JWTValidator
		}
		if config[0].SessionRepo != nil {
			sessionRepo = config[0].SessionRepo
		}
	}

	if validator == nil {
		validator = defaultJWTValidator{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())

			auth := r.Header.Get("Authorization")

			// 1. Header manquant
			if auth == "" {
				utils.WriteAppError(w, utils.ErrTokenMissing)
				return
			}

			// 2. Format Bearer obligatoire
			if !strings.HasPrefix(auth, "Bearer ") {
				utils.WriteAppError(w, utils.ErrTokenFormatInvalid)
				return
			}

			// 3. Extraction du token
			tokenString := strings.TrimPrefix(auth, "Bearer ")
			if tokenString == "" {
				utils.WriteAppError(w, utils.ErrTokenMalformed)
				return
			}

			// 4. Validation via le validateur
			claims, err := validator.ValidateToken(tokenString)
			if err != nil {
				utils.WriteAppError(w, utils.ErrTokenMalformed)
				return
			}

			// 5. Vérification du type access
			tType, ok := claims["type"].(string)
			if !ok || tType != "access" {
				utils.WriteAppError(w, utils.ErrTokenTypeInvalid)
				return
			}

			// 6. Vérification expiration
			exp, ok := claims["exp"].(float64)
			if !ok {
				utils.WriteAppError(w, utils.ErrTokenMalformed)
				return
			}
			if int64(exp) < time.Now().Unix() {
				utils.WriteAppError(w, utils.ErrAccessTokenExpired)
				return
			}

			// 7. Extraction du user ID (sub)
			userID, ok := claims["sub"].(string)
			if !ok || userID == "" {
				utils.WriteAppError(w, utils.ErrTokenSubjectInvalid)
				return
			}

			// ============================================================
			// 🆕 v4.0.0 : Extraction du rôle
			// ============================================================
			role, ok := claims["role"].(string)
			if !ok || role == "" {
				role = "merchant"
			}

			// ============================================================
			// 🆕 v4.4.2 : Extraction du session_id (jti)
			// ============================================================
			sessionID, _ := claims["jti"].(string)

			// ============================================================
			// 🆕 v4.4.2 : Extraction de l'email
			// ============================================================
			email, _ := claims["email"].(string)

			// ============================================================
			// 🆕 v4.4.2 : VÉRIFICATION DE LA SESSION ACTIVE
			// ============================================================
			if sessionRepo != nil && sessionID != "" {
				sessionCheckStart := time.Now()

				session, err := sessionRepo.FindBySessionID(r.Context(), sessionID)
				if err != nil {
					logger.Warn().
						Err(err).
						Str("user_id", userID).
						Str("session_id", sessionID).
						Msg("❌ Session non trouvée - accès refusé")
					utils.WriteAppError(w, utils.ErrUnauthorized)
					return
				}

				// Vérifier si la session est active
				if !session.IsActiveSession() {
					logger.Warn().
						Str("user_id", userID).
						Str("session_id", sessionID).
						Str("status", session.GetStatus()).
						Msg("❌ Session inactive - accès refusé")
					utils.WriteAppError(w, utils.ErrUnauthorized)
					return
				}

				// Mettre à jour last_activity (non bloquant)
				go func() {
					bgCtx := context.Background()
					if err := sessionRepo.UpdateLastActivity(bgCtx, sessionID); err != nil {
						logger.Debug().
							Err(err).
							Str("session_id", sessionID).
							Msg("⚠️ Erreur MAJ last_activity")
					}
				}()

				logger.Debug().
					Str("user_id", userID).
					Str("session_id", sessionID).
					Dur("session_check_ms", time.Since(sessionCheckStart)).
					Msg("✅ Session vérifiée")
			}

			// ============================================================
			// 🆕 v4.4.2 : Injection complète dans le contexte
			// ============================================================
			ctx := utils.WithFullUser(r.Context(), userID, role, email, sessionID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Version par défaut (production)
type defaultJWTValidator struct{}

func (d defaultJWTValidator) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	return utils.ValidateToken(tokenString)
}

// Version courte compatible (ancienne API)
func AuthMiddleware(next http.Handler) http.Handler {
	return NewAuthMiddleware()(next)
}
