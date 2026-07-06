package repository

import (
	"context"
	"errors"

	"Goshop/domain/entity"
)

// ============================================================
// 🆕 v4.4.0 : USER 2FA REPOSITORY INTERFACE
// ============================================================
//
// 🎯 Objectif :
//   Définir l'interface User2FARepository pour la persistance
//   de la configuration 2FA des utilisateurs.
//
// 📋 Principe DDD :
//   - Interface dans le domaine (domain/repository)
//   - Implémentation dans l'infrastructure (infrastructure/postgres)
//   - Injection de dépendance via le constructeur
//
// 🔐 Sécurité :
//   - Toutes les opérations nécessitent un contexte
//   - Données sensibles chiffrées en base
//   - Audit trail complet
//
// ============================================================

// ============================================================
// ERREURS
// ============================================================

var (
	ErrUser2FANotFound      = errors.New("user 2FA configuration not found")
	ErrUser2FAAlreadyExists = errors.New("user 2FA configuration already exists")
	ErrUser2FAInvalidData   = errors.New("invalid user 2FA data")
)

// ============================================================
// INTERFACE PRINCIPALE
// ============================================================

// User2FARepository définit les opérations de persistance pour la 2FA
type User2FARepository interface {
	// ============================================================
	// MÉTHODES CRUD
	// ============================================================

	// Create crée une nouvelle configuration 2FA (setup initial)
	// Retourne ErrUser2FAAlreadyExists si une config existe déjà
	Create(ctx context.Context, user2fa *entity.User2FA) error

	// FindByUserID récupère la configuration 2FA d'un utilisateur
	// Retourne ErrUser2FANotFound si aucune config n'existe
	FindByUserID(ctx context.Context, userID string) (*entity.User2FA, error)

	// Update met à jour la configuration 2FA
	// Utilisé pour : activation, désactivation, MAJ codes, reset failed attempts
	Update(ctx context.Context, user2fa *entity.User2FA) error

	// Delete supprime complètement la configuration 2FA
	// Utilisé lors de la désactivation définitive
	Delete(ctx context.Context, userID string) error

	// Exists vérifie si une configuration 2FA existe pour un user
	Exists(ctx context.Context, userID string) (bool, error)

	// ============================================================
	// MÉTHODES SPÉCIFIQUES 2FA
	// ============================================================

	// UpdateSecret met à jour le secret TOTP chiffré
	// Utilisé lors du setup initial
	UpdateSecret(ctx context.Context, userID string, secretEncrypted string) error

	// Enable2FA active la 2FA pour un utilisateur
	// Met à jour : is_enabled=true, enabled_at, updated_at
	Enable2FA(ctx context.Context, userID string) error

	// Disable2FA désactive la 2FA pour un utilisateur
	// Met à jour : is_enabled=false, disabled_at, updated_at
	// Supprime : secret, recovery codes
	Disable2FA(ctx context.Context, userID string) error

	// UpdateRecoveryCodes met à jour les codes de récupération chiffrés
	UpdateRecoveryCodes(ctx context.Context, userID string, codesEncrypted string) error

	// MarkRecoveryCodeUsed marque un code de récupération comme utilisé
	// Ajoute l'index au tableau recovery_codes_used
	MarkRecoveryCodeUsed(ctx context.Context, userID string, codeIndex int) error

	// ResetRecoveryCodes réinitialise tous les codes de récupération
	// Utilisé pour générer de nouveaux codes
	ResetRecoveryCodes(ctx context.Context, userID string) error

	// ============================================================
	// MÉTHODES DE SÉCURITÉ
	// ============================================================

	// IncrementFailedAttempts incrémente le compteur de tentatives échouées
	// Si MaxFailedAttempts atteint, verrouille le compte
	IncrementFailedAttempts(ctx context.Context, userID string) error

	// ResetFailedAttempts réinitialise le compteur de tentatives
	// Utilisé après une vérification réussie
	ResetFailedAttempts(ctx context.Context, userID string) error

	// LockAccount verrouille le compte pour une durée spécifiée
	LockAccount(ctx context.Context, userID string, durationMinutes int) error

	// UnlockAccount déverrouille manuellement le compte (par un admin)
	UnlockAccount(ctx context.Context, userID string) error

	// IsAccountLocked vérifie si le compte est verrouillé
	IsAccountLocked(ctx context.Context, userID string) (bool, error)

	// ============================================================
	// MÉTHODES D'AUDIT
	// ============================================================

	// RecordSuccessfulVerification enregistre une vérification réussie
	// Met à jour : last_verified_at, reset failed_attempts
	RecordSuccessfulVerification(ctx context.Context, userID string, ipAddress, userAgent string) error

	// UpdateLastActivity met à jour la dernière activité
	UpdateLastActivity(ctx context.Context, userID string, ipAddress, userAgent string) error

	// ============================================================
	// MÉTHODES DE STATISTIQUES
	// ============================================================

	// CountEnabled compte le nombre d'utilisateurs avec 2FA activée
	CountEnabled(ctx context.Context) (int, error)

	// CountLocked compte le nombre de comptes verrouillés
	CountLocked(ctx context.Context) (int, error)

	// GetStatistics retourne les statistiques globales 2FA
	GetStatistics(ctx context.Context) (*User2FAStatistics, error)

	// ListEnabledUsers liste les utilisateurs avec 2FA activée (pour audit)
	ListEnabledUsers(ctx context.Context, limit, offset int) ([]*entity.User2FA, int, error)

	// 🆕 v4.4.1 : Protection anti-replay
	// RecordCodeUsed enregistre un code TOTP comme utilisé
	RecordCodeUsed(ctx context.Context, userID string, code string) error
}

// ============================================================
// STRUCTURES DE DONNÉES
// ============================================================

// User2FAStatistics contient les statistiques globales 2FA
type User2FAStatistics struct {
	TotalEnabled      int     `json:"total_enabled"`
	TotalDisabled     int     `json:"total_disabled"`
	TotalLocked       int     `json:"total_locked"`
	TotalUsersWith2FA int     `json:"total_users_with_2fa"`
	AvgFailedAttempts float64 `json:"avg_failed_attempts"`
	MaxFailedAttempts int     `json:"max_failed_attempts"`
	AdoptionRate      float64 `json:"adoption_rate"` // Pourcentage d'admins avec 2FA
}

// User2FAStatus représente le statut 2FA d'un utilisateur (pour dashboard)
type User2FAStatus struct {
	UserID                 string `json:"user_id"`
	Email                  string `json:"email"`
	Role                   string `json:"role"`
	Is2FAEnabled           bool   `json:"is_2fa_enabled"`
	IsSetup                bool   `json:"is_setup"`
	IsLocked               bool   `json:"is_locked"`
	FailedAttempts         int    `json:"failed_attempts"`
	RemainingRecoveryCodes int    `json:"remaining_recovery_codes"`
	Status                 string `json:"status"`
}
