// interfaces/middl/http_metrics.go
package middl

import (
	"bufio"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Goshop/application/metrics"
)

// HTTPMetricsResponseWriter capture le status code et la taille
type HTTPMetricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bodySize   int
}

func (rw *HTTPMetricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *HTTPMetricsResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bodySize += n
	return n, err
}

// Support WebSocket
func (rw *HTTPMetricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// HTTPMetricsMiddleware enregistre les métriques HTTP
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &HTTPMetricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			bodySize:       0,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)
		path := getNormalizedPath(r.URL.Path)

		// Métriques HTTP de base
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path, status).Observe(duration)
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()

		// Taille de la requête
		if r.ContentLength > 0 {
			metrics.HTTPRequestSizeBytes.WithLabelValues(r.Method, path).Observe(float64(r.ContentLength))
		}

		// Taille de la réponse
		if rw.bodySize > 0 {
			metrics.HTTPResponseSizeBytes.WithLabelValues(r.Method, path, status).Observe(float64(rw.bodySize))
		}
	})
}

// getNormalizedPath normalise les URLs pour éviter l'explosion des labels
func getNormalizedPath(path string) string {
	// Routes statiques
	staticPaths := []string{
		"/login", "/register", "/logout", "/auth/refresh", "/auth/me",
		"/health/live", "/health/ready", "/metrics", "/swagger/",
		"/api/public/products", "/api/public/shops",
		"/ws/notifications",
	}

	for _, p := range staticPaths {
		if path == p || strings.HasPrefix(path, p) {
			return p
		}
	}

	// Routes avec IDs (normaliser pour éviter explosion des labels)
	parts := strings.Split(path, "/")
	if len(parts) >= 3 {
		// /api/products/:id
		if parts[1] == "api" && len(parts) >= 4 {
			resource := parts[2]
			if resource == "products" || resource == "customers" ||
				resource == "orders" || resource == "payments" ||
				resource == "withdrawals" || resource == "shops" {
				return "/api/" + resource + "/:id"
			}
		}

		// /webhooks/:provider
		if parts[1] == "webhooks" && len(parts) >= 3 {
			return "/webhooks/:provider"
		}

		// /admin/*
		if parts[1] == "admin" {
			return "/admin/" + parts[2]
		}
	}

	return path
}
