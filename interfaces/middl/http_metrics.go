// interfaces/middl/http_metrics.go
package middl

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"Goshop/application/metrics"
)

// HTTPMetricsResponseWriter capture le status code
type HTTPMetricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *HTTPMetricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// HTTPMetricsMiddleware enregistre les métriques HTTP
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// ✅ Utilise le bon type
		rw := &HTTPMetricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)
		path := getNormalizedPath(r.URL.Path)

		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path, status).Observe(duration)
	})
}

// getNormalizedPath normalise les URLs pour les métriques
func getNormalizedPath(path string) string {
	if path == "/login" || path == "/register" || path == "/health/live" {
		return path
	}

	if strings.HasPrefix(path, "/api/products/") && len(strings.Split(path, "/")) == 4 {
		return "/api/products/:id"
	}
	if strings.HasPrefix(path, "/api/customers/") && len(strings.Split(path, "/")) == 4 {
		return "/api/customers/:id"
	}
	if strings.HasPrefix(path, "/api/orders/") && len(strings.Split(path, "/")) == 4 {
		return "/api/orders/:id"
	}

	return path
}
