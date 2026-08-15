package middl

import (
	"net/http"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/interfaces/utils"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TenantResolver résout le tenant (boutique) depuis le host HTTP
// 🛡️ v4.11.0 : Vérifie que l'utilisateur a accès au shop (owner ou collaborateur)
// Ordre de résolution :
// 1. Header X-Shop-Slug (pour tests/dev)
// 2. Custom domain (ex: mamadou-boutique.com)
// 3. Sous-domaine (ex: demo.golanafrica.com)
func TenantResolver(
	shopRepo repository.ShopRepository,
	shopCollabRepo repository.ShopCollaboratorRepository,
	logger zerolog.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var shop *entity.Shop
			var err error

			// 1. Priorité au header X-Shop-Slug (pour tests/dev)
			if slug := r.Header.Get("X-Shop-Slug"); slug != "" {
				shop, err = shopRepo.FindBySlug(r.Context(), slug)
				if err != nil {
					logger.Error().Err(err).Str("slug", slug).Msg("Error finding shop by header")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if shop == nil {
					logger.Warn().Str("slug", slug).Msg("Shop not found via header")
					http.Error(w, "Shop not found", http.StatusNotFound)
					return
				}
			} else {
				// 2. Résolution depuis le Host
				host := extractHost(r.Host)

				// 2a. Essayer par custom_domain
				shop, err = shopRepo.FindByCustomDomain(r.Context(), host)
				if err != nil {
					logger.Error().Err(err).Str("host", host).Msg("Error finding shop by custom domain")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				// 2b. Sinon, essayer par slug (sous-domaine)
				if shop == nil {
					slug := extractSlugFromHost(host)
					if slug != "" {
						shop, err = shopRepo.FindBySlug(r.Context(), slug)
						if err != nil {
							logger.Error().Err(err).Str("slug", slug).Msg("Error finding shop by slug")
							http.Error(w, "Internal Server Error", http.StatusInternalServerError)
							return
						}
					}
				}

				// 2c. Si aucun tenant trouvé → 404
				if shop == nil {
					logger.Warn().Str("host", host).Msg("Shop not found")
					http.Error(w, "Shop not found - Use X-Shop-Slug header or subdomain", http.StatusNotFound)
					return
				}
			}

			// 3. Vérifier que la boutique est active
			if !shop.IsActive {
				logger.Warn().Str("shop_id", shop.ID.String()).Msg("Shop is inactive")
				http.Error(w, "Shop is inactive", http.StatusForbidden)
				return
			}

			// ============================================================
			// 🛡️ 4. VÉRIFIER L'ACCÈS DE L'UTILISATEUR AU SHOP
			// ============================================================
			userID, ok := utils.UserIDFromContext(r.Context())
			if !ok || userID == "" {
				logger.Warn().Msg("User ID not found in context")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// 4a. Parser le userID en UUID
			userUUID, err := uuid.Parse(userID)
			if err != nil {
				logger.Error().Err(err).
					Str("user_id", userID).
					Msg("Invalid user ID format")
				http.Error(w, "Invalid user ID", http.StatusBadRequest)
				return
			}

			// 4b. Vérifier si l'utilisateur est le propriétaire
			isOwner, err := shopRepo.IsOwner(r.Context(), shop.ID, userUUID)
			if err != nil {
				logger.Error().Err(err).
					Str("shop_id", shop.ID.String()).
					Str("user_id", userID).
					Msg("Error checking ownership")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// 4c. Si pas owner, vérifier si collaborateur
			if !isOwner {
				isCollaborator, err := shopCollabRepo.IsShopCollaborator(r.Context(), userID, shop.ID)
				if err != nil {
					logger.Error().Err(err).
						Str("shop_id", shop.ID.String()).
						Str("user_id", userID).
						Msg("Error checking collaboration")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				if !isCollaborator {
					logger.Warn().
						Str("shop_id", shop.ID.String()).
						Str("shop_slug", shop.Slug).
						Str("user_id", userID).
						Msg("🚨 Tentative d'accès non autorisé (tenant spoofing)")
					http.Error(w, "Access denied: you are not authorized to access this shop", http.StatusForbidden)
					return
				}
			}

			// 5. Ajouter le tenant au contexte
			ctx := tenant.WithTenant(r.Context(), shop)

			// 6. Log enrichi avec le shop_id
			logger.Debug().
				Str("shop_id", shop.ID.String()).
				Str("shop_slug", shop.Slug).
				Str("user_id", userID).
				Bool("is_owner", isOwner).
				Msg("✅ Tenant resolved and access verified")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractHost extrait le host sans le port
func extractHost(host string) string {
	if idx := strings.Index(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// extractSlugFromHost extrait le slug depuis un sous-domaine
// Ex: demo.golanafrica.com → demo
func extractSlugFromHost(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) >= 3 {
		return parts[0]
	}
	return ""
}
