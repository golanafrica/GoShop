package middl

import (
	"Goshop/interfaces/utils"
	"log"
	"net/http"
)

// HandlerWriteError est le type de tous les handlers "propres" de GoShop.
// Un handler retourne une erreur → le middleware se charge d'écrire la réponse.
//
// Exemple :
//
//	func (h *UserHandler) Register(w, r) error { ... }
type HandlerWriteError func(w http.ResponseWriter, r *http.Request) error

// ErrorHandler enveloppe un handler et capture :
// - les retours d'erreur "propres"
// - les AppError custom
// - les panic (sécurité)
// - les erreurs internes non prévues
//
// Il transforme TOUTES les erreurs en JSON cohérent.
func ErrorHandler(next HandlerWriteError) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// ------------------------
		// 1. Protection contre panic
		// ------------------------
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC] %v", rec)
				utils.WriteAppError(w, utils.ErrInternalServer)
			}
		}()

		// ------------------------
		// 2. Exécution du handler
		// ------------------------
		err := next(w, r)
		if err == nil {
			return // pas d'erreur → réponse envoyée par le handler
		}

		// ------------------------
		// 3. Erreur fonctionnelle connue (AppError)
		// ------------------------
		if appErr, ok := err.(*utils.AppError); ok {
			utils.WriteAppError(w, appErr)
			return
		}

		// ------------------------
		// 4. Erreur inconnue → 500 générique
		// ------------------------
		log.Printf("[ERROR] Unhandled: %v", err)
		utils.WriteAppError(w, utils.ErrInternalServer)
	}
}
