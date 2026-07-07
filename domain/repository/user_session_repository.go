package repository
//go:generate mockgen -destination=../../mocks/repository/mock_user_session_repository.go -package=repository . UserSessionRepository


import (
	"context"
	"errors"
	"time"

	"Goshop/domain/entity"
)

// ============================================================
// 🆕 v4.4.2 : USER SESSION REPOSITORY INTERFACE
// ============================================================
//
// 🎯 Objectif :
//   Définir l'interface UserSessionRepository pour la persistance
//   des sessions utilisateurs actives.
//
// 📋 Principe DDD :
//   - Interface dans le domaine (domain/repository)
//   - Implémentation dans l'infrastructure (infrastructure/postgres)
//   - Injection de dépendance via le constructeur
//
// 🔐 Sécurité :
//   - Toutes les opérations nécessitent un contexte
//   - Tokens jamais stockés en clair (hash SHA-256)
//   - Audit trail complet (revocation)
//
// ============================================================

// ============================================================
// ERREURS
// ============================================================

var (
	ErrSessionNotFound        = errors.New("session not found")
	ErrSessionAlreadyExists   = errors.New("session already exists")
	ErrSessionInvalidData     = errors.New("invalid session data")
	ErrSessionTokenMismatch   = errors.New("session token does not match hash")
	ErrSessionCannotRevokeOwn = errors.New("cannot revoke current session via this method")
)

// ============================================================
// INTERFACE PRINCIPALE
// ============================================================

// UserSessionRepository définit les opérations de persistance pour les sessions
type UserSessionRepository interface {
	// ============================================================
	// MÉTHODES CRUD
	// ============================================================

	// Create crée une nouvelle session
	// Retourne ErrSessionAlreadyExists si session_id existe déjà
	Create(ctx context.Context, session *entity.UserSession) error

	// FindByID récupère une session par son ID
	// Retourne ErrSessionNotFound si non trouvée
	FindByID(ctx context.Context, id string) (*entity.UserSession, error)

	// FindBySessionID récupère une session par son session_id (jti JWT)
	// Utilisé pour valider un token JWT lors de l'authentification
	FindBySessionID(ctx context.Context, sessionID string) (*entity.UserSession, error)

	// Update met à jour une session (last_activity, expires_at, etc.)
	Update(ctx context.Context, session *entity.UserSession) error

	// Delete supprime complètement une session
	Delete(ctx context.Context, id string) error

	// Exists vérifie si une session existe
	Exists(ctx context.Context, id string) (bool, error)

	// ============================================================
	// MÉTHODES SPÉCIFIQUES SESSIONS
	// ============================================================

	// FindByUserID récupère toutes les sessions d'un utilisateur
	// Inclut les sessions actives, révoquées et expirées
	FindByUserID(ctx context.Context, userID string) ([]*entity.UserSession, error)

	// FindActiveByUserID récupère uniquement les sessions actives d'un user
	// Exclut les sessions révoquées et expirées
	FindActiveByUserID(ctx context.Context, userID string) ([]*entity.UserSession, error)

	// CountByUserID compte le nombre de sessions d'un utilisateur
	CountByUserID(ctx context.Context, userID string) (int, error)

	// CountActiveByUserID compte les sessions actives d'un utilisateur
	CountActiveByUserID(ctx context.Context, userID string) (int, error)

	// ============================================================
	// MÉTHODES DE SÉCURITÉ
	// ============================================================

	// RevokeSession révoque une session spécifique
	// revokedBy : user_id qui a effectué la révocation
	RevokeSession(ctx context.Context, sessionID string, revokedBy string) error

	// RevokeAllUserSessions révoque TOUTES les sessions d'un utilisateur
	// Utilisé lors d'une compromission suspectée
	// excludeSessionID : session à ne pas révoquer (session actuelle)
	RevokeAllUserSessions(ctx context.Context, userID string, excludeSessionID string) error

	// ValidateToken vérifie si un token correspond à une session active
	// Retourne la session si valide, sinon une erreur
	ValidateToken(ctx context.Context, sessionID string, token string) (*entity.UserSession, error)

	// UpdateLastActivity met à jour la dernière activité d'une session
	// Utilisé à chaque requête authentifiée
	UpdateLastActivity(ctx context.Context, sessionID string) error

	// ExtendSession étend la durée d'une session
	ExtendSession(ctx context.Context, sessionID string, duration time.Duration) error

	// ============================================================
	// MÉTHODES DE NETTOYAGE
	// ============================================================

	// CleanupExpiredSessions supprime les sessions expirées
	// Retourne le nombre de sessions supprimées
	CleanupExpiredSessions(ctx context.Context) (int, error)

	// CleanupOldRevokedSessions supprime les sessions révoquées depuis > 30 jours
	CleanupOldRevokedSessions(ctx context.Context) (int, error)

	// CleanupAllInactive supprime toutes les sessions inactives
	// (expirées + révoquées anciennes)
	CleanupAllInactive(ctx context.Context) (int, error)

	// ============================================================
	// MÉTHODES DE STATISTIQUES
	// ============================================================

	// GetStatistics retourne les statistiques globales des sessions
	GetStatistics(ctx context.Context) (*entity.SessionStatistics, error)

	// GetStatisticsByUser retourne les statistiques par utilisateur
	GetStatisticsByUser(ctx context.Context, userID string) (*entity.SessionStatistics, error)

	// ListActiveSessions liste les sessions actives avec pagination
	ListActiveSessions(ctx context.Context, limit, offset int) ([]*entity.UserSession, int, error)

	// ListActiveSessionsByUser liste les sessions actives d'un user
	ListActiveSessionsByUser(ctx context.Context, userID string, limit, offset int) ([]*entity.UserSession, int, error)

	// ============================================================
	// MÉTHODES D'ANALYSE
	// ============================================================

	// FindSessionsByIP trouve toutes les sessions depuis une IP
	// Utile pour détecter des activités suspectes
	FindSessionsByIP(ctx context.Context, ipAddress string) ([]*entity.UserSession, error)

	// FindDuplicateSessions trouve les users avec plusieurs sessions actives
	// depuis des IPs différentes (activité suspecte)
	FindDuplicateSessions(ctx context.Context, maxSessionsPerUser int) ([]*entity.UserSession, error)

	// GetMostActiveUsers retourne les users avec le plus de sessions
	GetMostActiveUsers(ctx context.Context, limit int) ([]UserSessionCount, error)
}

// ============================================================
// STRUCTURES DE DONNÉES
// ============================================================

// UserSessionCount représente le nombre de sessions par utilisateur
type UserSessionCount struct {
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	SessionCount int       `json:"session_count"`
	LastActivity time.Time `json:"last_activity"`
}

// SessionActivity représente l'activité d'une session (pour audit)
type SessionActivity struct {
	SessionID    string    `json:"session_id"`
	UserID       string    `json:"user_id"`
	IPAddress    string    `json:"ip_address"`
	LastActivity time.Time `json:"last_activity"`
	Action       string    `json:"action"` // "login", "logout", "revoke", etc.
}
