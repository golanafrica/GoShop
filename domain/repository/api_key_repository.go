package repository

//go:generate mockgen -destination=../../mocks/repository/mock_api_key_repository.go -package=repository . APIKeyRepository

import (
	"context"
	"errors"
	"time"

	"Goshop/domain/entity"
)

// ============================================================
// 🆕 v4.4.3 : API KEY REPOSITORY INTERFACE
// ============================================================
//
// 🎯 Objectif :
//   Définir l'interface APIKeyRepository pour la persistance
//   des clés API avec scopes granulaires et audit trail.
//
// 📋 Principe DDD :
//   - Interface dans le domaine (domain/repository)
//   - Implémentation dans l'infrastructure (infrastructure/postgres)
//   - Injection de dépendance via le constructeur
//
// 🔐 Sécurité :
//   - Toutes les opérations nécessitent un contexte
//   - Clés jamais stockées en clair (hash SHA-256)
//   - Audit trail complet (usage logs)
//
// ============================================================

// ============================================================
// ERREURS
// ============================================================

var (
	ErrAPIKeyNotFound        = errors.New("api key not found")
	ErrAPIKeyAlreadyExists   = errors.New("api key already exists")
	ErrAPIKeyInvalidData     = errors.New("invalid api key data")
	ErrAPIKeyHashMismatch    = errors.New("api key hash does not match")
	ErrAPIKeyCannotRevokeOwn = errors.New("cannot revoke current api key via this method")
)

// ============================================================
// INTERFACE PRINCIPALE
// ============================================================

// APIKeyRepository définit les opérations de persistance pour les clés API
type APIKeyRepository interface {
	// ============================================================
	// MÉTHODES CRUD
	// ============================================================

	// Create crée une nouvelle clé API
	// Retourne ErrAPIKeyAlreadyExists si le hash existe déjà
	Create(ctx context.Context, apiKey *entity.APIKey) error

	// FindByID récupère une clé API par son ID
	// Retourne ErrAPIKeyNotFound si non trouvée
	FindByID(ctx context.Context, id string) (*entity.APIKey, error)

	// FindByHash récupère une clé API par son hash (authentification)
	// Utilisé par le middleware APIKeyAuth
	FindByHash(ctx context.Context, keyHash string) (*entity.APIKey, error)

	// Update met à jour une clé API (description, scopes, rate limits)
	Update(ctx context.Context, apiKey *entity.APIKey) error

	// Delete supprime complètement une clé API
	Delete(ctx context.Context, id string) error

	// Exists vérifie si une clé API existe
	Exists(ctx context.Context, id string) (bool, error)

	// ============================================================
	// MÉTHODES SPÉCIFIQUES API KEYS
	// ============================================================

	// FindByUserID récupère toutes les clés API d'un utilisateur
	// Inclut les clés actives, révoquées et expirées
	FindByUserID(ctx context.Context, userID string) ([]*entity.APIKey, error)

	// FindActiveByUserID récupère uniquement les clés actives d'un user
	// Exclut les clés révoquées et expirées
	FindActiveByUserID(ctx context.Context, userID string) ([]*entity.APIKey, error)

	// CountByUserID compte le nombre de clés API d'un utilisateur
	CountByUserID(ctx context.Context, userID string) (int, error)

	// CountActiveByUserID compte les clés actives d'un utilisateur
	CountActiveByUserID(ctx context.Context, userID string) (int, error)

	// ============================================================
	// MÉTHODES DE SÉCURITÉ
	// ============================================================

	// RevokeAPIKey révoque une clé API spécifique
	// revokedBy : user_id qui a effectué la révocation
	// reason : raison de la révocation
	RevokeAPIKey(ctx context.Context, id string, revokedBy, reason string) error

	// RevokeAllUserAPIKeys révoque TOUTES les clés API d'un utilisateur
	// Utilisé lors d'une compromission suspectée
	// excludeKeyID : clé à ne pas révoquer (optionnel)
	RevokeAllUserAPIKeys(ctx context.Context, userID string, excludeKeyID string, revokedBy, reason string) error

	// ValidateKey vérifie si une clé est valide (active + non expirée)
	// Retourne la clé si valide, sinon une erreur
	ValidateKey(ctx context.Context, keyHash string) (*entity.APIKey, error)

	// CheckScope vérifie si une clé a le scope requis
	CheckScope(ctx context.Context, keyHash string, scope entity.APIKeyScope) error

	// MarkKeyUsed enregistre l'utilisation d'une clé (MAJ last_used_at)
	MarkKeyUsed(ctx context.Context, keyHash string) error

	// ExtendExpiration étend la date d'expiration d'une clé
	ExtendExpiration(ctx context.Context, id string, duration time.Duration) error

	// ============================================================
	// MÉTHODES D'AUDIT
	// ============================================================

	// LogUsage enregistre l'utilisation d'une clé API
	LogUsage(ctx context.Context, log *entity.APIKeyUsageLog) error

	// GetUsageLogs récupère les logs d'utilisation d'une clé
	// limit : nombre max de logs à retourner
	GetUsageLogs(ctx context.Context, apiKeyID string, limit int) ([]*entity.APIKeyUsageLog, error)

	// GetRecentUsageLogs récupère les logs récents (dernières 24h)
	GetRecentUsageLogs(ctx context.Context, apiKeyID string) ([]*entity.APIKeyUsageLog, error)

	// CountUsageToday compte le nombre d'appels aujourd'hui pour une clé
	CountUsageToday(ctx context.Context, apiKeyID string) (int, error)

	// CountUsageLast7Days compte les appels des 7 derniers jours
	CountUsageLast7Days(ctx context.Context, apiKeyID string) (int, error)

	// ============================================================
	// MÉTHODES DE NETTOYAGE
	// ============================================================

	// CleanupExpiredKeys supprime les clés expirées depuis > 30 jours
	CleanupExpiredKeys(ctx context.Context) (int, error)

	// CleanupOldUsageLogs supprime les logs de plus de N jours
	// retentionDays : nombre de jours à conserver (défaut 90)
	CleanupOldUsageLogs(ctx context.Context, retentionDays int) (int, error)

	// CleanupAllInactive supprime toutes les clés inactives
	// (expirées anciennes + révoquées anciennes + logs anciens)
	CleanupAllInactive(ctx context.Context) (int, error)

	// ============================================================
	// MÉTHODES DE STATISTIQUES
	// ============================================================

	// GetStatistics retourne les statistiques globales des clés API
	GetStatistics(ctx context.Context) (*entity.APIKeyStatistics, error)

	// GetStatisticsByUser retourne les statistiques par utilisateur
	GetStatisticsByUser(ctx context.Context, userID string) (*entity.APIKeyStatistics, error)

	// ListActiveKeys liste les clés actives avec pagination
	ListActiveKeys(ctx context.Context, limit, offset int) ([]*entity.APIKey, int, error)

	// ListActiveKeysByUser liste les clés actives d'un user
	ListActiveKeysByUser(ctx context.Context, userID string, limit, offset int) ([]*entity.APIKey, int, error)

	// GetMostUsedKeys retourne les clés les plus utilisées
	GetMostUsedKeys(ctx context.Context, limit int) ([]APIKeyUsageCount, error)

	// ============================================================
	// MÉTHODES D'ANALYSE
	// ============================================================

	// FindKeysByIP trouve toutes les clés utilisées depuis une IP
	// Utile pour détecter des activités suspectes
	FindKeysByIP(ctx context.Context, ipAddress string) ([]*entity.APIKey, error)

	// FindDuplicateUsers trouve les users avec plusieurs clés actives
	// depuis des IPs différentes (activité suspecte)
	FindDuplicateUsers(ctx context.Context, maxKeysPerUser int) ([]*entity.APIKey, error)

	// GetExpiringKeys retourne les clés qui expirent bientôt
	// withinDays : nombre de jours (défaut 30)
	GetExpiringKeys(ctx context.Context, withinDays int) ([]*entity.APIKey, error)
}

// ============================================================
// STRUCTURES DE DONNÉES
// ============================================================

// APIKeyUsageCount représente le nombre d'utilisations par clé
type APIKeyUsageCount struct {
	APIKeyID   string    `json:"api_key_id"`
	KeyPrefix  string    `json:"key_prefix"`
	Name       string    `json:"name"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	UsageCount int       `json:"usage_count"`
	LastUsedAt time.Time `json:"last_used_at"`
}

// APIKeyUsageSummary représente un résumé d'utilisation
type APIKeyUsageSummary struct {
	APIKeyID       string  `json:"api_key_id"`
	TotalCalls     int     `json:"total_calls"`
	CallsToday     int     `json:"calls_today"`
	CallsLast7Days int     `json:"calls_last_7_days"`
	AvgResponseMs  int     `json:"avg_response_ms"`
	SuccessRate    float64 `json:"success_rate"` // Pourcentage de 2xx
}
