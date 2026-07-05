package middl

import "net/http"

// ============================================================
// 🆕 v4.3.2 : CHARSET UTF-8 MIDDLEWARE
// ============================================================
//
// 🎯 Objectif :
//   Forcer l'encodage UTF-8 pour toutes les réponses JSON.
//   Résout les problèmes d'encodage des caractères spéciaux
//   (é, à, ô, etc.) dans les réponses API.
//
// 📋 Usage :
//   Appliquer une seule fois dans app.go sur le router global.
//
// ============================================================

// CharsetUTF8 ajoute le header Content-Type avec charset=utf-8
func CharsetUTF8(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Forcer UTF-8 pour toutes les réponses
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}
