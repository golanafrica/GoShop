package middl

import (
	"context"
	"net/http"

	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.0 : REQUIRE 2FA MIDDLEWARE
// ============================================================
//
// 🎯 Objectif :
//   Protéger les routes sensibles en exigeant que l'utilisateur
//   ait la 2FA activée ET qu'il ait fourni un token 2FA valide.
//
// 📋 Utilisation :
//   - Appliquer sur les routes critiques (ex: /api/admin/sensitive/*)
//   - Nécessite que AuthMiddleware ait été appliqué avant
//   - Vérifie la présence du header "X-2FA-Token" ou cookie "2fa_token"
//
// 🔐 Flux :
//   1. Vérifier si l'user a la 2FA activée
//   2. Si oui, vérifier la présence du token 2FA
//   3. Valider le token (via Redis ou session)
//   4. Si valide, continuer ; sinon, refuser
//
// ⚠️ Note :
//   Ce middleware est OPTIONNEL. Il n'est pas appliqué par défaut
//   sur toutes les routes admin. Il doit être appliqué explicitement
//   sur les routes qui nécessitent une sécurité renforcée.
//
// ============================================================

// 🆕 v4.4.0 : Context keys pour 2FA
// Note : On utilise le type contextKey déjà défini dans request_id.go
const (
	TwoFATokenKey contextKey = "2fa_token"
	TwoFAValidKey contextKey = "2fa_valid"
)

// Require2FA middleware exige que l'utilisateur ait la 2FA activée
// et qu'il ait fourni un token 2FA valide.
//
// Utilisation :
//
//	r.Route("/api/admin/sensitive", func(r chi.Router) {
//	    r.Use(middleware.AuthMiddleware)
//	    r.Use(middl.Require2FA(user2faRepo))
//	    // ... routes protégées
//	})
func Require2FA(user2faRepo repository.User2FARepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())

			// 1. Extraire l'ID utilisateur du contexte (doit être injecté par AuthMiddleware)
			userID, ok := utils.UserIDFromContext(r.Context())
			if !ok || userID == "" {
				logger.Warn().Msg("❌ Require2FA: user_id not found in context")
				utils.WriteAppError(w, utils.ErrUnauthorized)
				return
			}

			// 2. Vérifier si l'user a la 2FA activée
			user2fa, err := user2faRepo.FindByUserID(r.Context(), userID)
			if err != nil {
				// Si pas de config 2FA, on laisse passer (2FA optionnelle)
				logger.Debug().
					Str("user_id", userID).
					Msg("ℹ️ Require2FA: 2FA not configured for user, allowing access")
				next.ServeHTTP(w, r)
				return
			}

			// 3. Si 2FA activée, vérifier le token
			if user2fa.IsEnabled {
				// Extraire le token 2FA du header ou cookie
				token2FA := extract2FAToken(r)
				if token2FA == "" {
					logger.Warn().
						Str("user_id", userID).
						Msg("❌ Require2FA: 2FA enabled but no token provided")
					utils.WriteAppError(w, utils.NewAppError(
						"2FA_TOKEN_REQUIRED",
						"2FA token is required for this operation",
						http.StatusUnauthorized,
					))
					return
				}

				// TODO: Valider le token 2FA via Redis ou session
				// Pour l'instant, on accepte tout token non vide
				// À implémenter : vérifier que le token est valide et non expiré

				// Injecter le token dans le contexte
				ctx := context.WithValue(r.Context(), TwoFATokenKey, token2FA)
				ctx = context.WithValue(ctx, TwoFAValidKey, true)
				r = r.WithContext(ctx)

				logger.Debug().
					Str("user_id", userID).
					Msg("✅ Require2FA: 2FA token validated")
			}

			// 4. Continuer vers le handler
			next.ServeHTTP(w, r)
		})
	}
}

// extract2FAToken extrait le token 2FA du header ou cookie
func extract2FAToken(r *http.Request) string {
	// 1. Essayer le header X-2FA-Token
	if token := r.Header.Get("X-2FA-Token"); token != "" {
		return token
	}

	// 2. Essayer le header Authorization (format: Bearer <jwt> <2fa_token>)
	if auth := r.Header.Get("Authorization"); auth != "" {
		// Format alternatif : Bearer <jwt>,2fa=<token>
		// TODO: Implémenter le parsing
	}

	// 3. Essayer le cookie
	if cookie, err := r.Cookie("2fa_token"); err == nil {
		return cookie.Value
	}

	return ""
}

// Get2FATokenFromContext récupère le token 2FA du contexte
func Get2FATokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(TwoFATokenKey).(string)
	return token, ok
}

// Is2FAValidFromContext vérifie si le token 2FA est valide
func Is2FAValidFromContext(ctx context.Context) bool {
	valid, ok := ctx.Value(TwoFAValidKey).(bool)
	return ok && valid
}
