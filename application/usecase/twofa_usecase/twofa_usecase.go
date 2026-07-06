package twofausecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"
	"Goshop/infrastructure/crypto"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.0 : 2FA USECASES
// 🆕 v4.4.1 : + Protection anti-replay (RFC 6238 Section 5.2)
// ============================================================
//
// 🎯 Objectif :
//   Gérer tous les cas d'utilisation de la 2FA :
//   - Setup : Générer secret TOTP + QR code
//   - Verify & Enable : Vérifier code et activer 2FA
//   - Disable : Désactiver 2FA
//   - Recovery : Utiliser code de récupération
//   - Regenerate : Régénérer codes de récupération
//   - Status : Récupérer statut 2FA
//
// 🔐 Sécurité :
//   - Secret TOTP chiffré avec AES-256-GCM (via crypto.Encrypt)
//   - Codes de récupération chiffrés
//   - Verrouillage après 5 tentatives échouées
//   - 🆕 v4.4.1 : Protection anti-replay (code utilisé rejeté dans 30s)
//   - Audit trail complet
//
// ============================================================

// ============================================================
// STRUCTURES COMMUNES
// ============================================================

// AdminContext représente le contexte de l'admin authentifié
type AdminContext struct {
	AdminID    string
	AdminEmail string
	AdminRole  string
	IPAddress  string
	UserAgent  string
}

// ============================================================
// USECASE 1 : SETUP 2FA
// ============================================================

// Setup2FAUsecase gère l'initialisation de la 2FA
type Setup2FAUsecase struct {
	user2faRepo repository.User2FARepository
	userRepo    userrepository.UserRepository
}

// NewSetup2FAUsecase crée une nouvelle instance
func NewSetup2FAUsecase(
	user2faRepo repository.User2FARepository,
	userRepo userrepository.UserRepository,
) *Setup2FAUsecase {
	return &Setup2FAUsecase{
		user2faRepo: user2faRepo,
		userRepo:    userRepo,
	}
}

// Setup2FARequest représente la requête
type Setup2FARequest struct {
	UserID string `json:"user_id"`
}

// Setup2FAResponse représente la réponse
type Setup2FAResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Secret      string `json:"secret"`       // Secret en clair (pour app d'auth)
	QRCodeURL   string `json:"qr_code_url"`  // URL pour QR code
	ManualEntry string `json:"manual_entry"` // Entrée manuelle
	ExpiresIn   int    `json:"expires_in"`   // Secondes avant expiration du setup
	Warning     string `json:"warning"`      // Avertissement de sécurité
}

// Execute initialise la 2FA pour un utilisateur
func (uc *Setup2FAUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *Setup2FARequest,
) (*Setup2FAResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation
	if req.UserID == "" {
		req.UserID = admin.AdminID
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Msg("🔐 Setup 2FA")

	// 2. Vérifier que l'user existe
	user, err := uc.userRepo.FindUserByID(req.UserID)
	if err != nil {
		if errors.Is(err, userrepository.ErrUserNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// 3. Vérifier si une config 2FA existe déjà
	exists, err := uc.user2faRepo.Exists(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("check 2fa exists: %w", err)
	}
	if exists {
		return nil, entity.Err2FAAlreadyEnabled
	}

	// 4. Générer le secret TOTP
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      entity.TOTPIssuer,
		AccountName: user.Email,
		Period:      entity.TOTPPeriod,
		Digits:      otp.Digits(entity.TOTPDigits),
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP key: %w", err)
	}

	// 5. Chiffrer le secret avec AES-256-GCM
	secretEncrypted, err := crypto.Encrypt(key.Secret())
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}

	// 6. Créer la configuration 2FA
	now := time.Now()
	user2fa := &entity.User2FA{
		UserID:            req.UserID,
		SecretEncrypted:   secretEncrypted,
		IsEnabled:         false, // Pas encore activé
		RecoveryCodesUsed: []int{},
		FailedAttempts:    0,
		SetupAt:           &now,
		LastIPAddress:     admin.IPAddress,
		LastUserAgent:     admin.UserAgent,
	}

	// 7. Sauvegarder en base
	if err := uc.user2faRepo.Create(ctx, user2fa); err != nil {
		return nil, fmt.Errorf("create 2fa config: %w", err)
	}

	logger.Info().
		Str("user_id", req.UserID).
		Str("email", user.Email).
		Msg("✅ 2FA setup initialized")

	return &Setup2FAResponse{
		Success:     true,
		Message:     "Configuration 2FA initialisée. Scannez le QR code avec votre application d'authentification.",
		Secret:      key.Secret(),
		QRCodeURL:   key.URL(),
		ManualEntry: fmt.Sprintf("%s: %s", entity.TOTPIssuer, key.Secret()),
		ExpiresIn:   300, // 5 minutes pour compléter le setup
		Warning:     "⚠️ Ne partagez JAMAIS ce secret ou QR code. Conservez vos codes de récupération en lieu sûr.",
	}, nil
}

// ============================================================
// USECASE 2 : VERIFY & ENABLE 2FA
// ============================================================

// VerifyAndEnable2FAUsecase gère la vérification et l'activation
type VerifyAndEnable2FAUsecase struct {
	user2faRepo repository.User2FARepository
}

// NewVerifyAndEnable2FAUsecase crée une nouvelle instance
func NewVerifyAndEnable2FAUsecase(
	user2faRepo repository.User2FARepository,
) *VerifyAndEnable2FAUsecase {
	return &VerifyAndEnable2FAUsecase{
		user2faRepo: user2faRepo,
	}
}

// VerifyAndEnable2FARequest représente la requête
type VerifyAndEnable2FARequest struct {
	UserID string `json:"user_id"`
	Code   string `json:"code"` // Code TOTP à 6 chiffres
}

// VerifyAndEnable2FAResponse représente la réponse
type VerifyAndEnable2FAResponse struct {
	Success              bool     `json:"success"`
	Message              string   `json:"message"`
	IsEnabled            bool     `json:"is_enabled"`
	RecoveryCodes        []string `json:"recovery_codes"` // Affichés UNE SEULE FOIS
	RecoveryCodesWarning string   `json:"recovery_codes_warning"`
}

// Execute vérifie le code et active la 2FA
func (uc *VerifyAndEnable2FAUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *VerifyAndEnable2FARequest,
) (*VerifyAndEnable2FAResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation
	if req.UserID == "" {
		req.UserID = admin.AdminID
	}
	if len(req.Code) != entity.TOTPDigits {
		return nil, entity.Err2FAInvalidCode
	}

	logger.Info().
		Str("user_id", req.UserID).
		Msg("🔐 Verify & Enable 2FA")

	// 2. Récupérer la config 2FA
	user2fa, err := uc.user2faRepo.FindByUserID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUser2FANotFound) {
			return nil, entity.Err2FANotSetup
		}
		return nil, fmt.Errorf("find 2fa config: %w", err)
	}

	// 3. Vérifier si le compte est verrouillé
	if user2fa.IsLocked() {
		return nil, entity.Err2FAAccountLocked
	}

	// 🆕 v4.4.1 : Protection anti-replay (RFC 6238 Section 5.2)
	if user2fa.IsCodeReplay(req.Code) {
		logger.Warn().
			Str("user_id", req.UserID).
			Str("code", req.Code).
			Msg("⚠️ Replay attack detected - code already used")
		if err := uc.user2faRepo.IncrementFailedAttempts(ctx, req.UserID); err != nil {
			logger.Error().Err(err).Msg("❌ Erreur increment failed attempts")
		}
		return nil, entity.Err2FACodeAlreadyUsed
	}

	// 4. Déchiffrer le secret avec AES-256-GCM
	secret, err := crypto.Decrypt(user2fa.SecretEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}

	// 5. Valider le code TOTP
	valid := totp.Validate(req.Code, secret)
	if !valid {
		// Incrémenter le compteur d'échecs
		if err := uc.user2faRepo.IncrementFailedAttempts(ctx, req.UserID); err != nil {
			logger.Error().Err(err).Msg("❌ Erreur increment failed attempts")
		}
		return nil, entity.Err2FAInvalidCode
	}

	// 🆕 v4.4.1 : Enregistrer le code comme utilisé (anti-replay)
	if err := uc.user2faRepo.RecordCodeUsed(ctx, req.UserID, req.Code); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur record code used")
	}

	// 6. Générer les codes de récupération
	recoveryCodes, err := entity.GenerateRecoveryCodes()
	if err != nil {
		return nil, fmt.Errorf("generate recovery codes: %w", err)
	}

	// 7. Chiffrer les codes de récupération avec AES-256-GCM
	recoveryCodesJSON, err := json.Marshal(recoveryCodes)
	if err != nil {
		return nil, fmt.Errorf("marshal recovery codes: %w", err)
	}

	recoveryCodesEncrypted, err := crypto.Encrypt(string(recoveryCodesJSON))
	if err != nil {
		return nil, fmt.Errorf("encrypt recovery codes: %w", err)
	}

	// 8. Mettre à jour les codes de récupération en base
	if err := uc.user2faRepo.UpdateRecoveryCodes(ctx, req.UserID, recoveryCodesEncrypted); err != nil {
		return nil, fmt.Errorf("update recovery codes: %w", err)
	}

	// 9. Activer la 2FA
	if err := uc.user2faRepo.Enable2FA(ctx, req.UserID); err != nil {
		return nil, fmt.Errorf("enable 2fa: %w", err)
	}

	// 10. Enregistrer le succès dans l'audit
	if err := uc.user2faRepo.RecordSuccessfulVerification(ctx, req.UserID, admin.IPAddress, admin.UserAgent); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur record successful verification")
	}

	logger.Info().
		Str("user_id", req.UserID).
		Msg("✅ 2FA enabled successfully")

	return &VerifyAndEnable2FAResponse{
		Success:              true,
		Message:              "2FA activée avec succès. Conservez vos codes de récupération en lieu sûr.",
		IsEnabled:            true,
		RecoveryCodes:        recoveryCodes, // 🆕 v4.4.1 : Variable maintenant déclarée
		RecoveryCodesWarning: "⚠️ Ces codes ne seront affichés QU'UNE SEULE FOIS. Copiez-les et stockez-les en lieu sûr.",
	}, nil
}

// ============================================================
// USECASE 3 : DISABLE 2FA
// ============================================================

// Disable2FAUsecase gère la désactivation de la 2FA
type Disable2FAUsecase struct {
	user2faRepo repository.User2FARepository
}

// NewDisable2FAUsecase crée une nouvelle instance
func NewDisable2FAUsecase(
	user2faRepo repository.User2FARepository,
) *Disable2FAUsecase {
	return &Disable2FAUsecase{
		user2faRepo: user2faRepo,
	}
}

// Disable2FARequest représente la requête
type Disable2FARequest struct {
	UserID     string `json:"user_id"`
	Code       string `json:"code"`        // Code TOTP OU code de récupération
	IsRecovery bool   `json:"is_recovery"` // true si c'est un code de récupération
}

// Disable2FAResponse représente la réponse
type Disable2FAResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Execute désactive la 2FA
func (uc *Disable2FAUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *Disable2FARequest,
) (*Disable2FAResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation
	if req.UserID == "" {
		req.UserID = admin.AdminID
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Bool("is_recovery", req.IsRecovery).
		Msg("🔐 Disable 2FA")

	// 2. Récupérer la config 2FA
	user2fa, err := uc.user2faRepo.FindByUserID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUser2FANotFound) {
			return nil, entity.Err2FANotSetup
		}
		return nil, fmt.Errorf("find 2fa config: %w", err)
	}

	if !user2fa.IsEnabled {
		return nil, entity.Err2FANotEnabled
	}

	// 3. Vérifier le code
	if req.IsRecovery {
		// Code de récupération
		if err := uc.verifyRecoveryCode(ctx, user2fa, req.Code); err != nil {
			return nil, err
		}
	} else {
		// Code TOTP
		if len(req.Code) != entity.TOTPDigits {
			return nil, entity.Err2FAInvalidCode
		}

		// 🆕 v4.4.1 : Protection anti-replay
		if user2fa.IsCodeReplay(req.Code) {
			logger.Warn().
				Str("user_id", req.UserID).
				Str("code", req.Code).
				Msg("⚠️ Replay attack detected - code already used")
			if err := uc.user2faRepo.IncrementFailedAttempts(ctx, req.UserID); err != nil {
				logger.Error().Err(err).Msg("❌ Erreur increment failed attempts")
			}
			return nil, entity.Err2FACodeAlreadyUsed
		}

		secret, err := crypto.Decrypt(user2fa.SecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt secret: %w", err)
		}

		valid := totp.Validate(req.Code, secret)
		if !valid {
			if err := uc.user2faRepo.IncrementFailedAttempts(ctx, req.UserID); err != nil {
				logger.Error().Err(err).Msg("❌ Erreur increment failed attempts")
			}
			return nil, entity.Err2FAInvalidCode
		}

		// 🆕 v4.4.1 : Enregistrer le code comme utilisé
		if err := uc.user2faRepo.RecordCodeUsed(ctx, req.UserID, req.Code); err != nil {
			logger.Error().Err(err).Msg("❌ Erreur record code used")
		}
	}

	// 4. Désactiver la 2FA
	if err := uc.user2faRepo.Disable2FA(ctx, req.UserID); err != nil {
		return nil, fmt.Errorf("disable 2fa: %w", err)
	}

	logger.Info().
		Str("user_id", req.UserID).
		Msg("✅ 2FA disabled successfully")

	return &Disable2FAResponse{
		Success: true,
		Message: "2FA désactivée avec succès. Votre compte est maintenant protégé uniquement par mot de passe.",
	}, nil
}

// verifyRecoveryCode vérifie un code de récupération
func (uc *Disable2FAUsecase) verifyRecoveryCode(ctx context.Context, user2fa *entity.User2FA, code string) error {
	if user2fa.RecoveryCodesEncrypted == "" {
		return entity.Err2FARecoveryCodeNotFound
	}

	// Déchiffrer les codes
	recoveryCodesJSON, err := crypto.Decrypt(user2fa.RecoveryCodesEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt recovery codes: %w", err)
	}

	var recoveryCodes []string
	if err := json.Unmarshal([]byte(recoveryCodesJSON), &recoveryCodes); err != nil {
		return fmt.Errorf("unmarshal recovery codes: %w", err)
	}

	// Valider le code
	_, err = user2fa.ValidateRecoveryCode(code, recoveryCodes)
	if err != nil {
		if err := uc.user2faRepo.IncrementFailedAttempts(ctx, user2fa.UserID); err != nil {
			return fmt.Errorf("increment failed attempts: %w", err)
		}
		return err
	}

	return nil
}

// ============================================================
// USECASE 4 : GET 2FA STATUS
// ============================================================

// Get2FAStatusUsecase récupère le statut 2FA
type Get2FAStatusUsecase struct {
	user2faRepo repository.User2FARepository
}

// NewGet2FAStatusUsecase crée une nouvelle instance
func NewGet2FAStatusUsecase(
	user2faRepo repository.User2FARepository,
) *Get2FAStatusUsecase {
	return &Get2FAStatusUsecase{
		user2faRepo: user2faRepo,
	}
}

// Get2FAStatusRequest représente la requête
type Get2FAStatusRequest struct {
	UserID string `json:"user_id"`
}

// Get2FAStatusResponse représente la réponse
type Get2FAStatusResponse struct {
	Success                bool       `json:"success"`
	UserID                 string     `json:"user_id"`
	IsEnabled              bool       `json:"is_enabled"`
	IsSetup                bool       `json:"is_setup"`
	IsLocked               bool       `json:"is_locked"`
	LockedUntil            *time.Time `json:"locked_until,omitempty"`
	FailedAttempts         int        `json:"failed_attempts"`
	LastVerifiedAt         *time.Time `json:"last_verified_at,omitempty"`
	RemainingRecoveryCodes int        `json:"remaining_recovery_codes"`
	Status                 string     `json:"status"`
}

// Execute récupère le statut 2FA
func (uc *Get2FAStatusUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *Get2FAStatusRequest,
) (*Get2FAStatusResponse, error) {
	logger := zerolog.Ctx(ctx)

	if req.UserID == "" {
		req.UserID = admin.AdminID
	}

	logger.Debug().
		Str("user_id", req.UserID).
		Msg("🔐 Get 2FA status")

	// Récupérer la config 2FA
	user2fa, err := uc.user2faRepo.FindByUserID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUser2FANotFound) {
			// Pas de config 2FA
			return &Get2FAStatusResponse{
				Success:   true,
				UserID:    req.UserID,
				IsEnabled: false,
				IsSetup:   false,
				IsLocked:  false,
				Status:    "not_setup",
			}, nil
		}
		return nil, fmt.Errorf("find 2fa config: %w", err)
	}

	return &Get2FAStatusResponse{
		Success:                true,
		UserID:                 user2fa.UserID,
		IsEnabled:              user2fa.IsEnabled,
		IsSetup:                user2fa.IsSetupComplete(),
		IsLocked:               user2fa.IsLocked(),
		LockedUntil:            user2fa.LockedUntil,
		FailedAttempts:         user2fa.FailedAttempts,
		LastVerifiedAt:         user2fa.LastVerifiedAt,
		RemainingRecoveryCodes: user2fa.GetRemainingRecoveryCodes(),
		Status:                 user2fa.GetStatus(),
	}, nil
}

// ============================================================
// USECASE 5 : REGENERATE RECOVERY CODES
// ============================================================

// RegenerateRecoveryCodesUsecase régénère les codes de récupération
type RegenerateRecoveryCodesUsecase struct {
	user2faRepo repository.User2FARepository
}

// NewRegenerateRecoveryCodesUsecase crée une nouvelle instance
func NewRegenerateRecoveryCodesUsecase(
	user2faRepo repository.User2FARepository,
) *RegenerateRecoveryCodesUsecase {
	return &RegenerateRecoveryCodesUsecase{
		user2faRepo: user2faRepo,
	}
}

// RegenerateRecoveryCodesRequest représente la requête
type RegenerateRecoveryCodesRequest struct {
	UserID string `json:"user_id"`
	Code   string `json:"code"` // Code TOTP pour confirmation
}

// RegenerateRecoveryCodesResponse représente la réponse
type RegenerateRecoveryCodesResponse struct {
	Success              bool     `json:"success"`
	Message              string   `json:"message"`
	RecoveryCodes        []string `json:"recovery_codes"`
	RecoveryCodesWarning string   `json:"recovery_codes_warning"`
}

// Execute régénère les codes de récupération
func (uc *RegenerateRecoveryCodesUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *RegenerateRecoveryCodesRequest,
) (*RegenerateRecoveryCodesResponse, error) {
	logger := zerolog.Ctx(ctx)

	if req.UserID == "" {
		req.UserID = admin.AdminID
	}

	logger.Info().
		Str("user_id", req.UserID).
		Msg("🔐 Regenerate recovery codes")

	// 1. Récupérer la config 2FA
	user2fa, err := uc.user2faRepo.FindByUserID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUser2FANotFound) {
			return nil, entity.Err2FANotSetup
		}
		return nil, fmt.Errorf("find 2fa config: %w", err)
	}

	if !user2fa.IsEnabled {
		return nil, entity.Err2FANotEnabled
	}

	// 2. Vérifier le code TOTP
	if len(req.Code) != entity.TOTPDigits {
		return nil, entity.Err2FAInvalidCode
	}

	// 🆕 v4.4.1 : Protection anti-replay
	if user2fa.IsCodeReplay(req.Code) {
		logger.Warn().
			Str("user_id", req.UserID).
			Str("code", req.Code).
			Msg("⚠️ Replay attack detected - code already used")
		if err := uc.user2faRepo.IncrementFailedAttempts(ctx, req.UserID); err != nil {
			logger.Error().Err(err).Msg("❌ Erreur increment failed attempts")
		}
		return nil, entity.Err2FACodeAlreadyUsed
	}

	secret, err := crypto.Decrypt(user2fa.SecretEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}

	valid := totp.Validate(req.Code, secret)
	if !valid {
		if err := uc.user2faRepo.IncrementFailedAttempts(ctx, req.UserID); err != nil {
			logger.Error().Err(err).Msg("❌ Erreur increment failed attempts")
		}
		return nil, entity.Err2FAInvalidCode
	}

	// 🆕 v4.4.1 : Enregistrer le code comme utilisé
	if err := uc.user2faRepo.RecordCodeUsed(ctx, req.UserID, req.Code); err != nil {
		logger.Error().Err(err).Msg("❌ Erreur record code used")
	}

	// 3. Générer de nouveaux codes
	recoveryCodes, err := entity.GenerateRecoveryCodes()
	if err != nil {
		return nil, fmt.Errorf("generate recovery codes: %w", err)
	}

	// 4. Chiffrer et sauvegarder avec AES-256-GCM
	recoveryCodesJSON, err := json.Marshal(recoveryCodes)
	if err != nil {
		return nil, fmt.Errorf("marshal recovery codes: %w", err)
	}

	recoveryCodesEncrypted, err := crypto.Encrypt(string(recoveryCodesJSON))
	if err != nil {
		return nil, fmt.Errorf("encrypt recovery codes: %w", err)
	}

	if err := uc.user2faRepo.UpdateRecoveryCodes(ctx, req.UserID, recoveryCodesEncrypted); err != nil {
		return nil, fmt.Errorf("update recovery codes: %w", err)
	}

	logger.Info().
		Str("user_id", req.UserID).
		Msg("✅ Recovery codes regenerated")

	return &RegenerateRecoveryCodesResponse{
		Success:              true,
		Message:              "Codes de récupération régénérés avec succès.",
		RecoveryCodes:        recoveryCodes,
		RecoveryCodesWarning: "⚠️ Ces codes ne seront affichés QU'UNE SEULE FOIS. Les anciens codes sont maintenant invalides.",
	}, nil
}
