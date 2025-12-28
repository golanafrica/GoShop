// interfaces/middl/request_id.go
package middl

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type contextKey string

const (
	requestIDKey    contextKey = "request_id"
	requestIDHeader string     = "X-Request-ID"
)

// RequestIDMiddleware injecte un ID unique par requête dans le contexte, les headers, et le logger.
// interfaces/middl/request_id.go
// RequestIDMiddleware injecte un ID unique par requête dans le contexte, les headers, et le logger.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}

		// Enrichit TOUJOURS le logger avec request_id
		logger := zerolog.Ctx(r.Context())
		newLogger := logger.With().Str("request_id", requestID).Logger()
		ctx := newLogger.WithContext(r.Context())
		ctx = context.WithValue(ctx, requestIDKey, requestID)
		r = r.WithContext(ctx)

		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r)
	})
}

// GetRequestID récupère le request_id depuis le contexte.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}
