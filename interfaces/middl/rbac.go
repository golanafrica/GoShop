package middl

import (
	"Goshop/interfaces/utils"
	"net/http"
)

// RequireRoles : middleware RBAC.
// Exemple : r.Use(RequireRoles("admin", "manager"))
//
// Il nécessite que AuthMiddleware ait déjà injecté
// les claims (UserID + Role).

func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 1. Extraction du rôle utilisateur
			role, ok := utils.UserRoleFromContext(r.Context())
			if !ok {
				utils.WriteAppError(w, utils.ErrUnauthorized)
				return
			}

			// 2. Vérification rôle autorisé
			for _, allowed := range allowedRoles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 3. Sinon refus
			utils.WriteAppError(w, utils.ErrForbidden)
		})
	}
}
