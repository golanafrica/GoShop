// interfaces/middl/secure_headers.go
package middl

import (
	"net/http"
	"os"
)

func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Headers de sécurité communs à tous les environnements
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")

		// CSP adaptative selon l'environnement
		env := os.Getenv("APP_ENV")
		if env == "development" && r.URL.Path == "/swagger/index.html" {
			// CSP PERMISSIF pour Swagger UI en développement
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"font-src 'self'; "+
					"connect-src 'self';")
		} else {
			// CSP STRICT pour tout le reste (production + API)
			w.Header().Set("Content-Security-Policy",
				"default-src 'none'; "+
					"frame-ancestors 'none'; "+
					"sandbox allow-same-origin allow-forms;")
		}

		next.ServeHTTP(w, r)
	})
}
