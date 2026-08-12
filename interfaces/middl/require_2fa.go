package middl

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/infrastructure/crypto"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

// ============================================================
// REQUIRE 2FA MIDDLEWARE
// ============================================================
//
// Protège les routes sensibles : si 2FA activée, exige un code TOTP
// valide (header X-2FA-Token ou cookie 2fa_token).
//
// Flux :
//  1. user_id depuis AuthMiddleware
//  2. FindByUserID — pas de config → laisser passer (2FA optionnelle)
//  3. is_enabled → exiger + valider TOTP (secret déchiffré)
//  4. Persister failed attempts / anti-replay / last_verified
// ============================================================

const (
	TwoFATokenKey contextKey = "2fa_token"
	TwoFAValidKey contextKey = "2fa_valid"
)

func Require2FA(user2faRepo repository.User2FARepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())

			userID, ok := utils.UserIDFromContext(r.Context())
			if !ok || userID == "" {
				logger.Warn().Msg("❌ Require2FA: user_id not found in context")
				utils.WriteAppError(w, utils.ErrUnauthorized)
				return
			}

			user2fa, err := user2faRepo.FindByUserID(r.Context(), userID)
			if err != nil {
				// Pas de config 2FA → optionnel, on laisse passer
				if errors.Is(err, repository.ErrUser2FANotFound) {
					logger.Debug().Str("user_id", userID).Msg("ℹ️ Require2FA: no 2FA config, allowing access")
					next.ServeHTTP(w, r)
					return
				}
				logger.Error().Err(err).Str("user_id", userID).Msg("❌ Require2FA: FindByUserID failed")
				utils.WriteAppError(w, utils.NewAppError("2FA_LOOKUP_FAILED", "failed to load 2FA configuration", http.StatusInternalServerError))
				return
			}

			if user2fa == nil || !user2fa.IsEnabled {
				next.ServeHTTP(w, r)
				return
			}

			if user2fa.IsLocked() {
				logger.Warn().Str("user_id", userID).Msg("❌ Require2FA: account locked")
				utils.WriteAppError(w, utils.NewAppError(
					"2FA_ACCOUNT_LOCKED",
					"2FA account is locked due to too many failed attempts",
					http.StatusForbidden,
				))
				return
			}

			token2FA := extract2FAToken(r)
			if token2FA == "" {
				logger.Warn().Str("user_id", userID).Msg("❌ Require2FA: 2FA enabled but no token provided")
				utils.WriteAppError(w, utils.NewAppError(
					"2FA_TOKEN_REQUIRED",
					"2FA token is required for this operation",
					http.StatusUnauthorized,
				))
				return
			}

			secret, err := crypto.Decrypt(user2fa.SecretEncrypted)
			if err != nil || secret == "" {
				logger.Error().Err(err).Str("user_id", userID).Msg("❌ Require2FA: decrypt secret failed")
				utils.WriteAppError(w, utils.NewAppError(
					"2FA_CONFIG_ERROR",
					"2FA configuration error",
					http.StatusInternalServerError,
				))
				return
			}

			// Validation TOTP (met à jour failed_attempts / last_used en mémoire)
			if err := user2fa.ValidateTOTPCode(token2FA, secret); err != nil {
				_ = user2faRepo.Update(r.Context(), user2fa)

				code, msg, status := map2FAError(err)
				logger.Warn().Err(err).Str("user_id", userID).Str("code", code).Msg("❌ Require2FA: invalid token")
				utils.WriteAppError(w, utils.NewAppError(code, msg, status))
				return
			}

			// Succès : persister anti-replay + last_verified
			ip := utils.GetClientIP(r)
			ua := r.UserAgent()
			user2fa.SetLastActivity(ip, ua)
			if err := user2faRepo.Update(r.Context(), user2fa); err != nil {
				logger.Error().Err(err).Str("user_id", userID).Msg("Require2FA: failed to persist success state")
			}
			_ = user2faRepo.RecordCodeUsed(r.Context(), userID, token2FA)
			_ = user2faRepo.RecordSuccessfulVerification(r.Context(), userID, ip, ua)

			ctx := context.WithValue(r.Context(), TwoFATokenKey, token2FA)
			ctx = context.WithValue(ctx, TwoFAValidKey, true)
			r = r.WithContext(ctx)

			logger.Debug().Str("user_id", userID).Msg("✅ Require2FA: TOTP validated")
			next.ServeHTTP(w, r)
		})
	}
}

func extract2FAToken(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-2FA-Token")); token != "" {
		return token
	}
	if cookie, err := r.Cookie("2fa_token"); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func map2FAError(err error) (code, msg string, status int) {
	switch {
	case errors.Is(err, entity.Err2FAAccountLocked):
		return "2FA_ACCOUNT_LOCKED", err.Error(), http.StatusForbidden
	case errors.Is(err, entity.Err2FACodeAlreadyUsed):
		return "2FA_CODE_REPLAY", err.Error(), http.StatusUnauthorized
	case errors.Is(err, entity.Err2FAInvalidCode):
		return "2FA_INVALID_CODE", err.Error(), http.StatusUnauthorized
	default:
		return "2FA_INVALID_CODE", "invalid 2FA code", http.StatusUnauthorized
	}
}

func Get2FATokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(TwoFATokenKey).(string)
	return token, ok
}

func Is2FAValidFromContext(ctx context.Context) bool {
	valid, ok := ctx.Value(TwoFAValidKey).(bool)
	return ok && valid
}
