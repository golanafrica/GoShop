package sessionusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// ============================================================
// 🆕 v4.4.2 : SESSION MANAGEMENT USECASES
// ============================================================
//
// 🎯 Objectif :
//   Gérer tous les cas d'utilisation de la gestion des sessions :
//   - ListSessions : Lister les sessions d'un utilisateur
//   - RevokeSession : Révoquer une session spécifique
//   - RevokeAllSessions : Révoquer toutes les sessions (sauf courante)
//   - GetSessionStats : Statistiques globales (admin)
//
// 🔐 Sécurité :
//   - Un user ne peut révoquer que SES propres sessions
//   - super_admin peut révoquer n'importe quelle session
//   - Audit trail complet (IP, User Agent, revoked_by)
//   - Protection contre révocation de la session courante
//
// ============================================================

// ============================================================
// STRUCTURES COMMUNES
// ============================================================

// AdminContext représente le contexte de l'admin authentifié
type AdminContext struct {
	AdminID          string
	AdminEmail       string
	AdminRole        string
	IPAddress        string
	UserAgent        string
	CurrentSessionID string // Session courante (jti JWT)
}

// ============================================================
// USECASE 1 : LIST SESSIONS
// ============================================================

// ListSessionsUsecase liste les sessions d'un utilisateur
type ListSessionsUsecase struct {
	sessionRepo repository.UserSessionRepository
}

// NewListSessionsUsecase crée une nouvelle instance
func NewListSessionsUsecase(
	sessionRepo repository.UserSessionRepository,
) *ListSessionsUsecase {
	return &ListSessionsUsecase{
		sessionRepo: sessionRepo,
	}
}

// ListSessionsRequest représente la requête
type ListSessionsRequest struct {
	UserID     string `json:"user_id"`     // Optionnel (admin peut voir d'autres users)
	ActiveOnly bool   `json:"active_only"` // true = seulement sessions actives
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

// ListSessionsResponse représente la réponse
type ListSessionsResponse struct {
	Success    bool                         `json:"success"`
	Message    string                       `json:"message"`
	Sessions   []*entity.UserSessionSummary `json:"sessions"`
	Total      int                          `json:"total"`
	CurrentID  string                       `json:"current_session_id"`
	Statistics *SessionStats                `json:"statistics,omitempty"`
}

// SessionStats contient les statistiques rapides
type SessionStats struct {
	TotalActive  int `json:"total_active"`
	TotalRevoked int `json:"total_revoked"`
	TotalExpired int `json:"total_expired"`
}

// Execute liste les sessions
func (uc *ListSessionsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *ListSessionsRequest,
) (*ListSessionsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation : par défaut, l'user voit ses propres sessions
	targetUserID := req.UserID
	if targetUserID == "" {
		targetUserID = admin.AdminID
	}

	// 2. Vérifier permissions : un user normal ne peut voir que ses sessions
	if targetUserID != admin.AdminID {
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("target_user_id", targetUserID).
				Msg("❌ Permission refusée : user ne peut voir que ses propres sessions")
			return nil, errors.New("insufficient permissions: can only view own sessions")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Str("target_user_id", targetUserID).
			Msg("ℹ️ Admin consulte les sessions d'un autre user")
	}

	logger.Debug().
		Str("target_user_id", targetUserID).
		Bool("active_only", req.ActiveOnly).
		Msg("📋 List sessions")

	// 3. Récupérer les sessions
	var sessions []*entity.UserSession
	var err error

	if req.ActiveOnly {
		sessions, err = uc.sessionRepo.FindActiveByUserID(ctx, targetUserID)
	} else {
		sessions, err = uc.sessionRepo.FindByUserID(ctx, targetUserID)
	}

	if err != nil {
		return nil, fmt.Errorf("find sessions: %w", err)
	}

	// 4. Convertir en summaries
	summaries := make([]*entity.UserSessionSummary, 0, len(sessions))
	for _, s := range sessions {
		summaries = append(summaries, s.ToSummary(admin.CurrentSessionID))
	}

	// 5. Récupérer les statistiques
	stats, err := uc.sessionRepo.GetStatisticsByUser(ctx, targetUserID)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur récupération statistiques")
	}

	var sessionStats *SessionStats
	if stats != nil {
		sessionStats = &SessionStats{
			TotalActive:  stats.TotalActive,
			TotalRevoked: stats.TotalRevoked,
			TotalExpired: stats.TotalExpired,
		}
	}

	return &ListSessionsResponse{
		Success:    true,
		Message:    fmt.Sprintf("%d session(s) trouvée(s)", len(summaries)),
		Sessions:   summaries,
		Total:      len(summaries),
		CurrentID:  admin.CurrentSessionID,
		Statistics: sessionStats,
	}, nil
}

// ============================================================
// USECASE 2 : REVOKE SESSION
// ============================================================

// RevokeSessionUsecase révoque une session spécifique
type RevokeSessionUsecase struct {
	sessionRepo repository.UserSessionRepository
}

// NewRevokeSessionUsecase crée une nouvelle instance
func NewRevokeSessionUsecase(
	sessionRepo repository.UserSessionRepository,
) *RevokeSessionUsecase {
	return &RevokeSessionUsecase{
		sessionRepo: sessionRepo,
	}
}

// RevokeSessionRequest représente la requête
type RevokeSessionRequest struct {
	TargetSessionID string `json:"target_session_id"` // Session à révoquer
	UserID          string `json:"user_id"`           // Propriétaire de la session (optionnel)
}

// RevokeSessionResponse représente la réponse
type RevokeSessionResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	RevokedSessionID string `json:"revoked_session_id"`
}

// Execute révoque une session spécifique
func (uc *RevokeSessionUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *RevokeSessionRequest,
) (*RevokeSessionResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation
	if req.TargetSessionID == "" {
		return nil, errors.New("target_session_id is required")
	}

	// 2. Protection : impossible de révoquer sa propre session courante
	if req.TargetSessionID == admin.CurrentSessionID {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("session_id", req.TargetSessionID).
			Msg("❌ Tentative de révocation de la session courante")
		return nil, errors.New("cannot revoke current active session")
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("admin_role", admin.AdminRole).
		Str("target_session_id", req.TargetSessionID).
		Msg("🔐 Revoke session")

	// 3. Récupérer la session cible
	session, err := uc.sessionRepo.FindBySessionID(ctx, req.TargetSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, errors.New("session not found")
		}
		return nil, fmt.Errorf("find session: %w", err)
	}

	// 4. Vérifier permissions
	if session.UserID != admin.AdminID {
		// L'admin essaie de révoquer la session d'un autre user
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("session_owner", session.UserID).
				Msg("❌ Permission refusée : user ne peut révoquer que ses propres sessions")
			return nil, errors.New("insufficient permissions: can only revoke own sessions")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Str("session_owner", session.UserID).
			Msg("ℹ️ Admin révoque la session d'un autre user")
	}

	// 5. Vérifier si la session est déjà révoquée
	if !session.IsActive {
		return nil, errors.New("session is already revoked")
	}

	// 6. Révoquer la session
	if err := uc.sessionRepo.RevokeSession(ctx, req.TargetSessionID, admin.AdminID); err != nil {
		return nil, fmt.Errorf("revoke session: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_session_id", req.TargetSessionID).
		Str("session_owner", session.UserID).
		Msg("✅ Session révoquée avec succès")

	return &RevokeSessionResponse{
		Success:          true,
		Message:          "Session révoquée avec succès",
		RevokedSessionID: req.TargetSessionID,
	}, nil
}

// ============================================================
// USECASE 3 : REVOKE ALL SESSIONS
// ============================================================

// RevokeAllSessionsUsecase révoque toutes les sessions d'un user (sauf courante)
type RevokeAllSessionsUsecase struct {
	sessionRepo repository.UserSessionRepository
}

// NewRevokeAllSessionsUsecase crée une nouvelle instance
func NewRevokeAllSessionsUsecase(
	sessionRepo repository.UserSessionRepository,
) *RevokeAllSessionsUsecase {
	return &RevokeAllSessionsUsecase{
		sessionRepo: sessionRepo,
	}
}

// RevokeAllSessionsRequest représente la requête
type RevokeAllSessionsRequest struct {
	UserID string `json:"user_id"` // Optionnel (admin peut révoquer pour un autre user)
}

// RevokeAllSessionsResponse représente la réponse
type RevokeAllSessionsResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	RevokedCount    int    `json:"revoked_count"`
	ExcludedSession string `json:"excluded_session"` // Session courante non révoquée
}

// Execute révoque toutes les sessions
func (uc *RevokeAllSessionsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *RevokeAllSessionsRequest,
) (*RevokeAllSessionsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation : par défaut, l'user révoque ses propres sessions
	targetUserID := req.UserID
	if targetUserID == "" {
		targetUserID = admin.AdminID
	}

	// 2. Vérifier permissions
	if targetUserID != admin.AdminID {
		if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
			logger.Warn().
				Str("admin_id", admin.AdminID).
				Str("admin_role", admin.AdminRole).
				Str("target_user_id", targetUserID).
				Msg("❌ Permission refusée : user ne peut révoquer que ses propres sessions")
			return nil, errors.New("insufficient permissions: can only revoke own sessions")
		}
		logger.Info().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Str("target_user_id", targetUserID).
			Msg("ℹ️ Admin révoque toutes les sessions d'un autre user")
	}

	// 3. Compter les sessions actives avant révocation
	activeCount, err := uc.sessionRepo.CountActiveByUserID(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("count active sessions: %w", err)
	}

	// 4. Révoquer toutes les sessions (sauf la courante)
	if err := uc.sessionRepo.RevokeAllUserSessions(ctx, targetUserID, admin.CurrentSessionID); err != nil {
		return nil, fmt.Errorf("revoke all sessions: %w", err)
	}

	// 5. Recalculer le nombre révoqué
	newActiveCount, err := uc.sessionRepo.CountActiveByUserID(ctx, targetUserID)
	if err != nil {
		logger.Warn().Err(err).Msg("⚠️ Erreur recalcul sessions actives")
		newActiveCount = 1 // Au moins la session courante
	}

	revokedCount := activeCount - newActiveCount

	logger.Info().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", targetUserID).
		Int("revoked_count", revokedCount).
		Msg("✅ Toutes les sessions révoquées (sauf courante)")

	return &RevokeAllSessionsResponse{
		Success:         true,
		Message:         fmt.Sprintf("%d session(s) révoquée(s). Votre session actuelle reste active.", revokedCount),
		RevokedCount:    revokedCount,
		ExcludedSession: admin.CurrentSessionID,
	}, nil
}

// ============================================================
// USECASE 4 : GET SESSION STATISTICS
// ============================================================

// GetSessionStatsUsecase récupère les statistiques globales
type GetSessionStatsUsecase struct {
	sessionRepo repository.UserSessionRepository
}

// NewGetSessionStatsUsecase crée une nouvelle instance
func NewGetSessionStatsUsecase(
	sessionRepo repository.UserSessionRepository,
) *GetSessionStatsUsecase {
	return &GetSessionStatsUsecase{
		sessionRepo: sessionRepo,
	}
}

// GetSessionStatsRequest représente la requête
type GetSessionStatsRequest struct {
	UserID string `json:"user_id"` // Optionnel (admin peut voir stats globales)
}

// GetSessionStatsResponse représente la réponse
type GetSessionStatsResponse struct {
	Success        bool                          `json:"success"`
	Message        string                        `json:"message"`
	Statistics     *entity.SessionStatistics     `json:"statistics"`
	TopActiveUsers []repository.UserSessionCount `json:"top_active_users,omitempty"`
	GeneratedAt    time.Time                     `json:"generated_at"`
}

// Execute récupère les statistiques
func (uc *GetSessionStatsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *GetSessionStatsRequest,
) (*GetSessionStatsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation des permissions (admin seulement)
	if admin.AdminRole != "super_admin" && admin.AdminRole != "admin" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("❌ Permission refusée : statistiques réservées aux admins")
		return nil, errors.New("insufficient permissions: admin role required")
	}

	logger.Debug().
		Str("admin_id", admin.AdminID).
		Str("target_user_id", req.UserID).
		Msg("📊 Get session statistics")

	// 2. Récupérer les statistiques
	var stats *entity.SessionStatistics
	var err error

	if req.UserID != "" {
		stats, err = uc.sessionRepo.GetStatisticsByUser(ctx, req.UserID)
	} else {
		stats, err = uc.sessionRepo.GetStatistics(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("get statistics: %w", err)
	}

	// 3. Récupérer les top users actifs (stats globales seulement)
	var topUsers []repository.UserSessionCount
	if req.UserID == "" {
		topUsers, err = uc.sessionRepo.GetMostActiveUsers(ctx, 10)
		if err != nil {
			logger.Warn().Err(err).Msg("⚠️ Erreur récupération top users")
		}
	}

	return &GetSessionStatsResponse{
		Success:        true,
		Message:        "Statistiques récupérées avec succès",
		Statistics:     stats,
		TopActiveUsers: topUsers,
		GeneratedAt:    time.Now(),
	}, nil
}

// ============================================================
// USECASE 5 : CLEANUP EXPIRED SESSIONS (Admin)
// ============================================================

// CleanupSessionsUsecase nettoie les sessions expirées
type CleanupSessionsUsecase struct {
	sessionRepo repository.UserSessionRepository
}

// NewCleanupSessionsUsecase crée une nouvelle instance
func NewCleanupSessionsUsecase(
	sessionRepo repository.UserSessionRepository,
) *CleanupSessionsUsecase {
	return &CleanupSessionsUsecase{
		sessionRepo: sessionRepo,
	}
}

// CleanupSessionsRequest représente la requête
type CleanupSessionsRequest struct {
	IncludeRevoked bool `json:"include_revoked"` // true = aussi révoquées anciennes
}

// CleanupSessionsResponse représente la réponse
type CleanupSessionsResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	DeletedCount int    `json:"deleted_count"`
}

// Execute nettoie les sessions
func (uc *CleanupSessionsUsecase) Execute(
	ctx context.Context,
	admin *AdminContext,
	req *CleanupSessionsRequest,
) (*CleanupSessionsResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Validation des permissions (super_admin seulement)
	if admin.AdminRole != "super_admin" {
		logger.Warn().
			Str("admin_id", admin.AdminID).
			Str("admin_role", admin.AdminRole).
			Msg("❌ Permission refusée : cleanup réservé au super_admin")
		return nil, errors.New("insufficient permissions: super_admin role required")
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Bool("include_revoked", req.IncludeRevoked).
		Msg("🧹 Cleanup sessions")

	// 2. Nettoyer
	var deletedCount int
	var err error

	if req.IncludeRevoked {
		deletedCount, err = uc.sessionRepo.CleanupAllInactive(ctx)
	} else {
		deletedCount, err = uc.sessionRepo.CleanupExpiredSessions(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("cleanup sessions: %w", err)
	}

	logger.Info().
		Str("admin_id", admin.AdminID).
		Int("deleted_count", deletedCount).
		Msg("✅ Cleanup terminé")

	return &CleanupSessionsResponse{
		Success:      true,
		Message:      fmt.Sprintf("%d session(s) supprimée(s)", deletedCount),
		DeletedCount: deletedCount,
	}, nil
}
