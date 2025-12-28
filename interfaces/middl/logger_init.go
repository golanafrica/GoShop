// interfaces/middl/logger_init.go
package middl

import (
	"net/http"

	"Goshop/config/setupLogging"
)

// LoggerInitMiddleware initialise le logger de base dans le contexte
func LoggerInitMiddleware(appLogger *setupLogging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Crée le logger de base avec les champs globaux
			baseLogger := appLogger.Zerolog()
			ctx := baseLogger.WithContext(r.Context())
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
