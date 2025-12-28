package middl

import "net/http"

// CORS middleware PRO pour API backend modern.
// Compatibles React / Next / Vue / Flutter / mobile.
func CORS(origin string) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Autorise l'origine envoyée en paramètre
			w.Header().Set("Access-Control-Allow-Origin", origin)

			// Indique que des credentials sont acceptés (JWT, cookies)
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Méthodes autorisées
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

			// Headers autorisés
			w.Header().Set("Access-Control-Allow-Headers",
				"Authorization, Content-Type, Accept, X-Requested-With",
			)

			// Expose certains headers au client
			w.Header().Set("Access-Control-Expose-Headers",
				"Authorization, Content-Type",
			)

			// Si c’est une requête preflight → on répond 200
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
