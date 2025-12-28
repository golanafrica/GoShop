package middl

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// On crée un histogramme pour mesurer la latence (durée)
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "goshop_http_duration_seconds",
		Help:    "Temps de réponse des endpoints GoShop",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"path", "method", "status"})
)

func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Un wrapper pour capturer le code HTTP (200, 500...)
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.status)

		// On enregistre la donnée
		httpDuration.WithLabelValues(r.URL.Path, r.Method, status).Observe(duration)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
