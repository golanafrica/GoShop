// interfaces/middl/secure_headers_swagger.go
package middl

import (
	"net/http"
	"os"
)

// SecureHeadersSwagger applique des headers de sécurité PERMISSIFS pour Swagger UI
//
// 📋 Usage :
// Ce middleware est appliqué UNIQUEMENT sur les routes Swagger (/swagger/*)
// car l'interface nécessite des ressources inline (scripts, styles) que
// la CSP stricte bloquerait.
//
// ⚠️ Ce middleware doit TOUJOURS être combiné avec une restriction d'accès
// en production (Swagger devrait être désactivé ou protégé par authentification)
func SecureHeadersSwagger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ============================================================
		// HEADERS COMMUNS
		// ============================================================

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0") // Aligné avec secure_headers.go
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=()")

		// ============================================================
		// HSTS - Activé en production/staging
		// ============================================================
		env := os.Getenv("APP_ENV")
		if env == "production" || env == "staging" {
			w.Header().Set("Strict-Transport-Security",
				"max-age=63072000; includeSubDomains")
		}

		// ============================================================
		// CSP PERMISSIF pour Swagger UI
		// ============================================================
		// Swagger UI nécessite :
		// - 'unsafe-inline' pour les styles générés dynamiquement
		// - 'unsafe-eval' pour certains scripts de parsing OpenAPI
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self';")

		next.ServeHTTP(w, r)
	})
}
