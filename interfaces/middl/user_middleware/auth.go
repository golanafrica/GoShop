package middleware

import (
	"Goshop/interfaces/utils"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddlewareConfig permet d'injecter un faux validateur en tests.
type AuthMiddlewareConfig struct {
	JWTValidator utils.JWTValidator
}

// NewAuthMiddleware crée un middleware propre et testable.
func NewAuthMiddleware(config ...AuthMiddlewareConfig) func(http.Handler) http.Handler {
	var validator utils.JWTValidator

	// Choix entre validateur custom (tests) ou celui de utils
	if len(config) > 0 && config[0].JWTValidator != nil {
		validator = config[0].JWTValidator
	} else {
		validator = defaultJWTValidator{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			auth := r.Header.Get("Authorization")

			// 1. Header manquant
			if auth == "" {
				utils.WriteAppError(w, utils.ErrTokenMissing) // "missing Authorization header"
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
				utils.WriteAppError(w, utils.ErrTokenMalformed) // "invalid or corrupted token"
				return
			}

			// 4. Validation via le validateur
			claims, err := validator.ValidateToken(tokenString)
			if err != nil {
				// CORRECTION ICI : Utiliser ErrTokenMalformed au lieu de ErrUnauthorized
				utils.WriteAppError(w, utils.ErrTokenMalformed) // "invalid or corrupted token"
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

			// 8. Injection dans le context
			ctx := utils.WithUserID(r.Context(), userID)

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
