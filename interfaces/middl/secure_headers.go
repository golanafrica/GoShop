// interfaces/middl/secure_headers.go
// interfaces/middl/secure_headers.go
package middl

import (
	"net/http"
	"os"
)

// SecureHeaders applique les headers de sécurité HTTP à toutes les réponses
//
// 🛡️ Headers appliqués :
// - X-Content-Type-Options: nosniff (anti-MIME sniffing)
// - X-Frame-Options: DENY (anti-clickjacking)
// - X-XSS-Protection: 0 (XSS Auditor déprécié, désactivé)
// - Referrer-Policy: no-referrer (contrôle du header Referer)
// - Cache-Control: no-store (pas de cache pour données sensibles)
// - Permissions-Policy: restriction APIs navigateur
// - Strict-Transport-Security: HSTS (uniquement en prod/staging)
// - Content-Security-Policy: adaptative selon environnement
//
// ⚠️ Note importante sur HSTS :
// En développement, HSTS est DÉSACTIVÉ pour permettre l'accès à localhost en HTTP.
// En production/staging, HSTS force HTTPS pour 2 ans sur tous les sous-domaines.
// Une fois HSTS activé avec max-age élevé, il est difficile de revenir en arrière
// (les navigateurs "se souviennent" de l'exigence HTTPS).
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ============================================================
		// HEADERS COMMUNS À TOUS LES ENVIRONNEMENTS
		// ============================================================

		// Anti-MIME sniffing : empêche le navigateur de deviner le type MIME
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Anti-clickjacking : empêche l'embedding du site dans une iframe
		w.Header().Set("X-Frame-Options", "DENY")

		// XSS Auditor déprécié : value "0" pour désactiver explicitement
		// (retiré de Chrome 78+, non supporté par Firefox/Safari)
		w.Header().Set("X-XSS-Protection", "0")

		// Ne pas envoyer le header Referer (protection privacy)
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Pas de cache pour l'API (données sensibles, données temps réel)
		w.Header().Set("Cache-Control", "no-store")

		// Restriction des APIs navigateur non nécessaires
		// Désactive : caméra, micro, géolocalisation, payment API
		w.Header().Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=()")

		// ============================================================
		// HSTS - HTTP STRICT TRANSPORT SECURITY
		// ============================================================
		// Activé UNIQUEMENT en production/staging (pas en dev)
		// Raison : en dev, localhost est souvent en HTTP, HSTS bloquerait l'accès
		env := os.Getenv("APP_ENV")
		if env == "production" || env == "staging" {
			// max-age=63072000 = 2 ans (63072000 secondes)
			// includeSubDomains = protège api.goshop.com, admin.goshop.com, etc.
			// Note : pas de "preload" car irréversible une fois soumis à hstspreload.org
			w.Header().Set("Strict-Transport-Security",
				"max-age=63072000; includeSubDomains")
		}

		// ============================================================
		// CSP - CONTENT SECURITY POLICY (ADAPTATIVE)
		// ============================================================
		if env == "development" && r.URL.Path == "/swagger/index.html" {
			// CSP PERMISSIF pour Swagger UI en développement
			// Swagger nécessite unsafe-inline pour ses scripts et styles inline
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"font-src 'self'; "+
					"connect-src 'self';")
		} else {
			// CSP STRICT pour tout le reste (production + API)
			// default-src 'none' = rien n'est autorisé par défaut
			// frame-ancestors 'none' = pas d'embedding (équivalent X-Frame-Options)
			// sandbox = isolation du contexte d'exécution
			w.Header().Set("Content-Security-Policy",
				"default-src 'none'; "+
					"frame-ancestors 'none'; "+
					"sandbox allow-same-origin allow-forms;")
		}

		next.ServeHTTP(w, r)
	})
}
