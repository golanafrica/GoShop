package entity

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// ============================================================
// 🆕 v4.4.0 : USER 2FA ENTITY
// ============================================================
//
// 🎯 Objectif :
//   Représenter la configuration 2FA d'un utilisateur avec :
//   - Secret TOTP (chiffré en base)
//   - Codes de récupération (chiffrés en base)
//   - Gestion du verrouillage après échecs
//   - Validation TOTP conforme RFC 6238
//
// 📋 Règles métier :
//   - 1 seule configuration 2FA par user (UNIQUE)
//   - Verrouillage après 5 tentatives échouées
//   - Durée de verrouillage : 15 minutes
//   - Codes de récupération : 10 codes uniques
//   - Longueur code TOTP : 6 chiffres
//   - Validité code TOTP : 30 secondes
//
// 🔐 Sécurité :
//   - Secret jamais loggé en clair
//   - Codes de récupération à usage unique
//   - Chiffrement AES-256 en base de données
//
// ============================================================

// ============================================================
// CONSTANTES
// ============================================================

const (
	// TOTP Configuration
	TOTPIssuer    = "GoShop"
	TOTPPeriod    = 30 // secondes
	TOTPDigits    = 6
	TOTPAlgorithm = "SHA1"

	// Codes de récupération
	RecoveryCodesCount = 10
	RecoveryCodeLength = 8 // caractères par code

	// Sécurité
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

// ============================================================
// ERREURS
// ============================================================

var (
	Err2FANotEnabled           = errors.New("2FA is not enabled for this user")
	Err2FAAlreadyEnabled       = errors.New("2FA is already enabled for this user")
	Err2FANotSetup             = errors.New("2FA is not set up for this user")
	Err2FAInvalidCode          = errors.New("invalid 2FA code")
	Err2FAAccountLocked        = errors.New("2FA account is locked due to too many failed attempts")
	Err2FARecoveryCodeUsed     = errors.New("recovery code has already been used")
	Err2FARecoveryCodeNotFound = errors.New("recovery code not found")
	Err2FASecretRequired       = errors.New("2FA secret is required")
	Err2FACodeAlreadyUsed      = errors.New("2FA code has already been used (replay protection)")
)

// ============================================================
// STRUCTURE PRINCIPALE
// ============================================================

// User2FA représente la configuration 2FA d'un utilisateur
type User2FA struct {
	// Identification
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Configuration TOTP (chiffré en base)
	SecretEncrypted string `json:"-"` // Jamais sérialisé
	IsEnabled       bool   `json:"is_enabled"`

	// Codes de récupération (chiffrés en base)
	RecoveryCodesEncrypted string `json:"-"` // Jamais sérialisé
	RecoveryCodesUsed      []int  `json:"recovery_codes_used"`

	// Sécurité
	FailedAttempts int        `json:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until"`
	LastVerifiedAt *time.Time `json:"last_verified_at"`

	// 🆕 v4.4.1 : Protection anti-replay (RFC 6238 Section 5.2)
	LastUsedCode string     `json:"-"` // Dernier code TOTP utilisé
	LastUsedAt   *time.Time `json:"-"` // Timestamp de dernière utilisation

	// Audit
	SetupAt       *time.Time `json:"setup_at"`
	EnabledAt     *time.Time `json:"enabled_at"`
	DisabledAt    *time.Time `json:"disabled_at"`
	LastIPAddress string     `json:"last_ip_address"`
	LastUserAgent string     `json:"last_user_agent"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Champ temporaire (non persisté) : secret en clair pour setup initial
	TemporarySecret string `json:"temporary_secret,omitempty"`
}

// ============================================================
// CONSTRUCTEURS
// ============================================================

// NewUser2FA crée une nouvelle configuration 2FA (non activée)
func NewUser2FA(userID string) (*User2FA, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	now := time.Now()

	return &User2FA{
		UserID:            userID,
		IsEnabled:         false,
		RecoveryCodesUsed: []int{},
		FailedAttempts:    0,
		SetupAt:           &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// ============================================================
// MÉTHODES TOTP
// ============================================================

// GenerateTOTPSecret génère un nouveau secret TOTP
// Retourne le secret en clair (pour QR code) et la clé TOTP
func GenerateTOTPSecret(email string) (string, *otp.Key, error) {
	if email == "" {
		return "", nil, errors.New("email is required")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: email,
		Period:      TOTPPeriod,
		Digits:      otp.Digits(TOTPDigits),
	})
	if err != nil {
		return "", nil, fmt.Errorf("generate TOTP key: %w", err)
	}

	return key.Secret(), key, nil
}

// ValidateTOTPCode valide un code TOTP avec protection anti-replay
func (u *User2FA) ValidateTOTPCode(code string, secret string) error {
	// Vérifier si le compte est verrouillé
	if u.IsLocked() {
		return Err2FAAccountLocked
	}

	// Valider le format du code
	if len(code) != TOTPDigits {
		u.RecordFailedAttempt()
		return Err2FAInvalidCode
	}

	// 🆕 v4.4.1 : Protection anti-replay (RFC 6238 Section 5.2)
	if u.IsCodeReplay(code) {
		u.RecordFailedAttempt()
		return Err2FACodeAlreadyUsed
	}

	// Valider le code TOTP
	valid := totp.Validate(code, secret)
	if !valid {
		u.RecordFailedAttempt()
		return Err2FAInvalidCode
	}

	// Succès : enregistrer le code comme utilisé et reset des tentatives
	u.RecordCodeUsed(code)
	u.RecordSuccessfulVerification()
	return nil
}

// 🆕 v4.4.1 : IsCodeReplay vérifie si le code est une réutilisation
func (u *User2FA) IsCodeReplay(code string) bool {
	if u.LastUsedCode == "" || u.LastUsedAt == nil {
		return false // Première utilisation
	}

	// Si même code et dans la fenêtre de 30 secondes → replay
	if u.LastUsedCode == code {
		timeSinceLastUse := time.Since(*u.LastUsedAt)
		if timeSinceLastUse < time.Duration(TOTPPeriod)*time.Second {
			return true
		}
	}

	return false
}

// 🆕 v4.4.1 : RecordCodeUsed enregistre un code comme utilisé
func (u *User2FA) RecordCodeUsed(code string) {
	now := time.Now()
	u.LastUsedCode = code
	u.LastUsedAt = &now
	u.UpdatedAt = now
}

// GetTOTPURL retourne l'URL pour le QR code
func GetTOTPURL(key *otp.Key) string {
	if key == nil {
		return ""
	}
	return key.URL()
}

// ============================================================
// MÉTHODES CODES DE RÉCUPÉRATION
// ============================================================

// GenerateRecoveryCodes génère 10 codes de récupération uniques
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodesCount)

	for i := 0; i < RecoveryCodesCount; i++ {
		code, err := generateRandomCode(RecoveryCodeLength)
		if err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		codes[i] = code
	}

	return codes, nil
}

// generateRandomCode génère un code aléatoire de la longueur spécifiée
func generateRandomCode(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	code := hex.EncodeToString(bytes)
	// Format : XXXX-XXXX pour lisibilité
	if len(code) >= length {
		code = code[:length]
	}

	// Ajouter un tiret au milieu pour lisibilité
	if len(code) == 8 {
		code = code[:4] + "-" + code[4:]
	}

	return strings.ToUpper(code), nil
}

// ValidateRecoveryCode valide un code de récupération
func (u *User2FA) ValidateRecoveryCode(code string, storedCodes []string) (int, error) {
	// Normaliser le code
	code = strings.ToUpper(strings.TrimSpace(code))

	// Chercher le code dans la liste
	for i, storedCode := range storedCodes {
		storedCode = strings.ToUpper(strings.TrimSpace(storedCode))

		if code == storedCode {
			// Vérifier si le code a déjà été utilisé
			if u.IsRecoveryCodeUsed(i) {
				return i, Err2FARecoveryCodeUsed
			}

			// Marquer comme utilisé
			u.MarkRecoveryCodeUsed(i)
			u.RecordSuccessfulVerification()
			return i, nil
		}
	}

	// Code non trouvé
	u.RecordFailedAttempt()
	return -1, Err2FARecoveryCodeNotFound
}

// IsRecoveryCodeUsed vérifie si un code a déjà été utilisé
func (u *User2FA) IsRecoveryCodeUsed(index int) bool {
	for _, usedIndex := range u.RecoveryCodesUsed {
		if usedIndex == index {
			return true
		}
	}
	return false
}

// MarkRecoveryCodeUsed marque un code comme utilisé
func (u *User2FA) MarkRecoveryCodeUsed(index int) {
	if !u.IsRecoveryCodeUsed(index) {
		u.RecoveryCodesUsed = append(u.RecoveryCodesUsed, index)
		u.UpdatedAt = time.Now()
	}
}

// GetRemainingRecoveryCodes retourne le nombre de codes restants
func (u *User2FA) GetRemainingRecoveryCodes() int {
	return RecoveryCodesCount - len(u.RecoveryCodesUsed)
}

// ============================================================
// MÉTHODES DE SÉCURITÉ
// ============================================================

// IsLocked vérifie si le compte est verrouillé
func (u *User2FA) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// RecordFailedAttempt enregistre une tentative échouée
func (u *User2FA) RecordFailedAttempt() {
	u.FailedAttempts++
	u.UpdatedAt = time.Now()

	// Verrouiller si trop de tentatives
	if u.FailedAttempts >= MaxFailedAttempts {
		lockUntil := time.Now().Add(LockoutDuration)
		u.LockedUntil = &lockUntil
	}
}

// RecordSuccessfulVerification enregistre une vérification réussie
func (u *User2FA) RecordSuccessfulVerification() {
	now := time.Now()
	u.FailedAttempts = 0
	u.LockedUntil = nil
	u.LastVerifiedAt = &now
	u.UpdatedAt = now
}

// Unlock déverrouille manuellement le compte (par un admin)
func (u *User2FA) Unlock() {
	u.FailedAttempts = 0
	u.LockedUntil = nil
	u.UpdatedAt = time.Now()
}

// ============================================================
// MÉTHODES D'ACTIVATION/DÉSACTIVATION
// ============================================================

// Enable active la 2FA
func (u *User2FA) Enable() error {
	if u.SecretEncrypted == "" {
		return Err2FASecretRequired
	}

	if u.IsEnabled {
		return Err2FAAlreadyEnabled
	}

	now := time.Now()
	u.IsEnabled = true
	u.EnabledAt = &now
	u.UpdatedAt = now

	return nil
}

// Disable désactive la 2FA
func (u *User2FA) Disable() {
	if !u.IsEnabled {
		return
	}

	now := time.Now()
	u.IsEnabled = false
	u.DisabledAt = &now
	u.SecretEncrypted = ""
	u.RecoveryCodesEncrypted = ""
	u.RecoveryCodesUsed = []int{}
	u.FailedAttempts = 0
	u.LockedUntil = nil
	u.UpdatedAt = now
}

// ============================================================
// MÉTHODES D'AUDIT
// ============================================================

// SetLastActivity enregistre la dernière activité
func (u *User2FA) SetLastActivity(ipAddress, userAgent string) {
	u.LastIPAddress = ipAddress
	u.LastUserAgent = userAgent
	u.UpdatedAt = time.Now()
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// IsSetupComplete vérifie si la configuration est complète
func (u *User2FA) IsSetupComplete() bool {
	return u.SecretEncrypted != ""
}

// GetStatus retourne le statut 2FA sous forme de string
func (u *User2FA) GetStatus() string {
	switch {
	case !u.IsSetupComplete():
		return "not_setup"
	case u.IsLocked():
		return "locked"
	case u.IsEnabled:
		return "enabled"
	default:
		return "disabled"
	}
}

// ToStatusResponse retourne un statut pour l'API (sans données sensibles)
type User2FAStatus struct {
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

// ToStatusResponse convertit l'entité en réponse API
func (u *User2FA) ToStatusResponse() *User2FAStatus {
	return &User2FAStatus{
		UserID:                 u.UserID,
		IsEnabled:              u.IsEnabled,
		IsSetup:                u.IsSetupComplete(),
		IsLocked:               u.IsLocked(),
		LockedUntil:            u.LockedUntil,
		FailedAttempts:         u.FailedAttempts,
		LastVerifiedAt:         u.LastVerifiedAt,
		RemainingRecoveryCodes: u.GetRemainingRecoveryCodes(),
		Status:                 u.GetStatus(),
	}
}
