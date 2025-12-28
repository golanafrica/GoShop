// interfaces/middl/request_logger.go
package middl

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// responseWriter wrapper pour capturer le status et la taille
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bodySize   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bodySize += n
	return n, err
}

// RequestLoggerMiddleware logue le début et la fin de chaque requête
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		logger := zerolog.Ctx(r.Context())
		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_ip", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("request_started")

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		logLevel := determineLogLevel(rw.statusCode, duration)
		logger.WithLevel(logLevel).
			Int("status", rw.statusCode).
			Dur("duration_ms", duration).
			Int64("response_size_bytes", int64(rw.bodySize)).
			Str("warning", slowRequestWarning(duration)).
			Msg("request_completed")
	})
}

// determineLogLevel détermine le niveau de log selon le statut et la durée
func determineLogLevel(status int, duration time.Duration) zerolog.Level {
	switch {
	case status >= 500:
		return zerolog.ErrorLevel
	case status >= 400:
		return zerolog.WarnLevel
	case duration > 1*time.Second:
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}

// slowRequestWarning retourne un warning si la requête est lente
func slowRequestWarning(duration time.Duration) string {
	if duration > 2*time.Second {
		return "slow_request"
	}
	return ""
}
