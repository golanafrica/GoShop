package middl

import (
	"net/http"
	"strings"

	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// TenantResolver résout le tenant (boutique) depuis le host HTTP
func TenantResolver(shopRepo repository.ShopRepository, logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := extractHost(r.Host)

			// 1. Essayer par custom_domain
			shop, err := shopRepo.FindByCustomDomain(r.Context(), host)
			if err != nil {
				logger.Error().Err(err).Str("host", host).Msg("Error finding shop by custom domain")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// 2. Sinon, essayer par slug (sous-domaine)
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

			// 3. Si aucun tenant trouvé → 404
			if shop == nil {
				logger.Warn().Str("host", host).Msg("Shop not found")
				http.Error(w, "Shop not found", http.StatusNotFound)
				return
			}

			// 4. Vérifier que la boutique est active
			if !shop.IsActive {
				logger.Warn().Str("shop_id", shop.ID.String()).Msg("Shop is inactive")
				http.Error(w, "Shop is inactive", http.StatusForbidden)
				return
			}

			// 5. Ajouter le tenant au contexte
			ctx := tenant.WithTenant(r.Context(), shop)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractHost extrait le host sans le port
func extractHost(host string) string {
	// Retirer le port si présent (ex: localhost:8080 → localhost)
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
	// Cas spécial : localhost ou single-word → pas de slug
	return ""
}
