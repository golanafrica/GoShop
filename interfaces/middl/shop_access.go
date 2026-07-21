package middl

import (
	"net/http"

	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"
)

// RequireShopAccess vérifie que l'utilisateur authentifié est propriétaire ou collaborateur de la boutique
func RequireShopAccess(collabRepo repository.ShopCollaboratorRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. Récupérer la boutique du contexte (injectée par TenantResolver)
			shop, err := tenant.FromContext(ctx)
			if err != nil || shop == nil {
				utils.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "Accès refusé : boutique non trouvée"})
				return
			}

			// 2. Récupérer l'ID de l'utilisateur authentifié
			userID, ok := utils.GetUserID(ctx)
			if !ok || userID == "" {
				utils.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Non authentifié"})
				return
			}

			// 3. Vérifier si l'utilisateur est le propriétaire de la boutique
			// shop.OwnerID est déjà de type string
			if shop.OwnerID == userID {
				next.ServeHTTP(w, r)
				return
			}

			// 4. Sinon, vérifier si c'est un collaborateur actif de cette boutique
			_, err = collabRepo.FindByShopIDAndUserID(ctx, shop.ID, userID)
			if err != nil {
				// L'utilisateur n'est ni propriétaire ni collaborateur
				utils.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "Accès refusé : vous n'avez pas les droits sur cette boutique"})
				return
			}

			// Accès autorisé
			next.ServeHTTP(w, r)
		})
	}
}
