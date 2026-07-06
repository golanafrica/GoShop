package usersession

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/google/uuid"
)

// ============================================================
// 🆕 v4.4.2 : USER SESSION REPOSITORY - POSTGRES IMPLEMENTATION
// ============================================================

// UserSessionRepositoryImpl implémente repository.UserSessionRepository
type UserSessionRepositoryImpl struct {
	db *sql.DB
}

// NewUserSessionRepository crée une nouvelle instance du repository
func NewUserSessionRepository(db *sql.DB) *UserSessionRepositoryImpl {
	return &UserSessionRepositoryImpl{
		db: db,
	}
}

// ============================================================
// MÉTHODES CRUD
// ============================================================

// Create crée une nouvelle session
func (r *UserSessionRepositoryImpl) Create(ctx context.Context, session *entity.UserSession) error {
	if session == nil {
		return repository.ErrSessionInvalidData
	}

	id := uuid.New().String()
	now := time.Now()

	deviceInfoJSON, err := json.Marshal(session.DeviceInfo)
	if err != nil {
		return fmt.Errorf("marshal device_info: %w", err)
	}

	query := `
		INSERT INTO user_sessions (
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err = r.db.ExecContext(ctx, query,
		id,
		session.UserID,
		session.SessionTokenHash,
		session.SessionID,
		string(deviceInfoJSON),
		session.IPAddress,
		session.LastActivity,
		now,
		session.ExpiresAt,
		session.IsActive,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return repository.ErrSessionAlreadyExists
		}
		return fmt.Errorf("create user_session: %w", err)
	}

	session.ID = id
	session.CreatedAt = now
	return nil
}

// FindByID récupère une session par son ID
func (r *UserSessionRepositoryImpl) FindByID(ctx context.Context, id string) (*entity.UserSession, error) {
	if id == "" {
		return nil, repository.ErrSessionInvalidData
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE id = $1
	`

	return r.scanSession(r.db.QueryRowContext(ctx, query, id))
}

// FindBySessionID récupère une session par son session_id (jti JWT)
func (r *UserSessionRepositoryImpl) FindBySessionID(ctx context.Context, sessionID string) (*entity.UserSession, error) {
	if sessionID == "" {
		return nil, repository.ErrSessionInvalidData
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE session_id = $1
	`

	return r.scanSession(r.db.QueryRowContext(ctx, query, sessionID))
}

// Update met à jour une session
func (r *UserSessionRepositoryImpl) Update(ctx context.Context, session *entity.UserSession) error {
	if session == nil {
		return repository.ErrSessionInvalidData
	}

	deviceInfoJSON, err := json.Marshal(session.DeviceInfo)
	if err != nil {
		return fmt.Errorf("marshal device_info: %w", err)
	}

	query := `
		UPDATE user_sessions SET
			session_token_hash = $1,
			device_info = $2,
			ip_address = $3,
			last_activity = $4,
			expires_at = $5,
			is_active = $6,
			revoked_at = $7,
			revoked_by = $8
		WHERE id = $9
	`

	result, err := r.db.ExecContext(ctx, query,
		session.SessionTokenHash,
		string(deviceInfoJSON),
		session.IPAddress,
		session.LastActivity,
		session.ExpiresAt,
		session.IsActive,
		session.RevokedAt,
		session.RevokedBy,
		session.ID,
	)

	if err != nil {
		return fmt.Errorf("update user_session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrSessionNotFound
	}

	return nil
}

// Delete supprime complètement une session
func (r *UserSessionRepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return repository.ErrSessionInvalidData
	}

	query := `DELETE FROM user_sessions WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user_session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrSessionNotFound
	}

	return nil
}

// Exists vérifie si une session existe
func (r *UserSessionRepositoryImpl) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, repository.ErrSessionInvalidData
	}

	query := `SELECT EXISTS(SELECT 1 FROM user_sessions WHERE id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user_session exists: %w", err)
	}

	return exists, nil
}

// ============================================================
// MÉTHODES SPÉCIFIQUES SESSIONS
// ============================================================

// FindByUserID récupère toutes les sessions d'un utilisateur
func (r *UserSessionRepositoryImpl) FindByUserID(ctx context.Context, userID string) ([]*entity.UserSession, error) {
	if userID == "" {
		return nil, repository.ErrSessionInvalidData
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE user_id = $1
		ORDER BY last_activity DESC
	`

	return r.scanSessions(r.db.QueryContext(ctx, query, userID))
}

// FindActiveByUserID récupère uniquement les sessions actives
func (r *UserSessionRepositoryImpl) FindActiveByUserID(ctx context.Context, userID string) ([]*entity.UserSession, error) {
	if userID == "" {
		return nil, repository.ErrSessionInvalidData
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE user_id = $1 AND is_active = true AND expires_at > NOW()
		ORDER BY last_activity DESC
	`

	return r.scanSessions(r.db.QueryContext(ctx, query, userID))
}

// CountByUserID compte le nombre de sessions
func (r *UserSessionRepositoryImpl) CountByUserID(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, repository.ErrSessionInvalidData
	}

	query := `SELECT COUNT(*) FROM user_sessions WHERE user_id = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}

	return count, nil
}

// CountActiveByUserID compte les sessions actives
func (r *UserSessionRepositoryImpl) CountActiveByUserID(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, repository.ErrSessionInvalidData
	}

	query := `
		SELECT COUNT(*) FROM user_sessions 
		WHERE user_id = $1 AND is_active = true AND expires_at > NOW()
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active sessions: %w", err)
	}

	return count, nil
}

// ============================================================
// MÉTHODES DE SÉCURITÉ
// ============================================================

// RevokeSession révoque une session spécifique
func (r *UserSessionRepositoryImpl) RevokeSession(ctx context.Context, sessionID string, revokedBy string) error {
	if sessionID == "" {
		return repository.ErrSessionInvalidData
	}

	query := `
		UPDATE user_sessions 
		SET 
			is_active = false,
			revoked_at = NOW(),
			revoked_by = $2
		WHERE session_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, sessionID, revokedBy)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrSessionNotFound
	}

	return nil
}

// RevokeAllUserSessions révoque TOUTES les sessions d'un utilisateur
func (r *UserSessionRepositoryImpl) RevokeAllUserSessions(ctx context.Context, userID string, excludeSessionID string) error {
	if userID == "" {
		return repository.ErrSessionInvalidData
	}

	query := `
		UPDATE user_sessions 
		SET 
			is_active = false,
			revoked_at = NOW(),
			revoked_by = $1
		WHERE user_id = $2 
		AND is_active = true
		AND session_id != $3
	`

	_, err := r.db.ExecContext(ctx, query, userID, userID, excludeSessionID)
	if err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}

	return nil
}

// ValidateToken vérifie si un token correspond à une session active
func (r *UserSessionRepositoryImpl) ValidateToken(ctx context.Context, sessionID string, token string) (*entity.UserSession, error) {
	if sessionID == "" || token == "" {
		return nil, repository.ErrSessionInvalidData
	}

	session, err := r.FindBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Vérifier si la session est active
	if !session.IsActiveSession() {
		if session.IsRevoked() {
			return nil, entity.ErrSessionRevoked
		}
		return nil, entity.ErrSessionExpired
	}

	// Vérifier le token
	if !session.ValidateToken(token) {
		return nil, repository.ErrSessionTokenMismatch
	}

	return session, nil
}

// UpdateLastActivity met à jour la dernière activité
func (r *UserSessionRepositoryImpl) UpdateLastActivity(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return repository.ErrSessionInvalidData
	}

	query := `
		UPDATE user_sessions 
		SET last_activity = NOW()
		WHERE session_id = $1 AND is_active = true AND expires_at > NOW()
	`

	result, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("update last activity: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrSessionNotFound
	}

	return nil
}

// ExtendSession étend la durée d'une session
func (r *UserSessionRepositoryImpl) ExtendSession(ctx context.Context, sessionID string, duration time.Duration) error {
	if sessionID == "" {
		return repository.ErrSessionInvalidData
	}

	if duration <= 0 {
		duration = entity.DefaultSessionDuration
	}

	newExpires := time.Now().Add(duration)

	query := `
		UPDATE user_sessions 
		SET 
			expires_at = $1,
			last_activity = NOW()
		WHERE session_id = $2 AND is_active = true
	`

	result, err := r.db.ExecContext(ctx, query, newExpires, sessionID)
	if err != nil {
		return fmt.Errorf("extend session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrSessionNotFound
	}

	return nil
}

// ============================================================
// MÉTHODES DE NETTOYAGE
// ============================================================

// CleanupExpiredSessions supprime les sessions expirées
func (r *UserSessionRepositoryImpl) CleanupExpiredSessions(ctx context.Context) (int, error) {
	query := `
		DELETE FROM user_sessions 
		WHERE expires_at <= NOW()
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// CleanupOldRevokedSessions supprime les sessions révoquées depuis > 30 jours
func (r *UserSessionRepositoryImpl) CleanupOldRevokedSessions(ctx context.Context) (int, error) {
	query := `
		DELETE FROM user_sessions 
		WHERE is_active = false 
		AND revoked_at < NOW() - INTERVAL '30 days'
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup old revoked sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// CleanupAllInactive supprime toutes les sessions inactives
func (r *UserSessionRepositoryImpl) CleanupAllInactive(ctx context.Context) (int, error) {
	query := `
		DELETE FROM user_sessions 
		WHERE expires_at <= NOW() 
		OR (is_active = false AND revoked_at < NOW() - INTERVAL '30 days')
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup all inactive: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// ============================================================
// MÉTHODES DE STATISTIQUES
// ============================================================

// GetStatistics retourne les statistiques globales
func (r *UserSessionRepositoryImpl) GetStatistics(ctx context.Context) (*entity.SessionStatistics, error) {
	query := `
		SELECT 
			COUNT(*) FILTER (WHERE is_active = true AND expires_at > NOW()) AS total_active,
			COUNT(*) FILTER (WHERE is_active = false) AS total_revoked,
			COUNT(*) FILTER (WHERE expires_at <= NOW()) AS total_expired,
			COUNT(DISTINCT user_id) FILTER (WHERE is_active = true AND expires_at > NOW()) AS unique_users_active,
			COALESCE(AVG(EXTRACT(EPOCH FROM (NOW() - last_activity)) / 60) FILTER (WHERE is_active = true), 0) AS avg_idle_minutes
		FROM user_sessions
	`

	var stats entity.SessionStatistics
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalActive,
		&stats.TotalRevoked,
		&stats.TotalExpired,
		&stats.UniqueUsersActive,
		&stats.AvgIdleMinutes,
	)

	if err != nil {
		return nil, fmt.Errorf("get session statistics: %w", err)
	}

	return &stats, nil
}

// GetStatisticsByUser retourne les statistiques par utilisateur
func (r *UserSessionRepositoryImpl) GetStatisticsByUser(ctx context.Context, userID string) (*entity.SessionStatistics, error) {
	if userID == "" {
		return nil, repository.ErrSessionInvalidData
	}

	query := `
		SELECT 
			COUNT(*) FILTER (WHERE is_active = true AND expires_at > NOW()) AS total_active,
			COUNT(*) FILTER (WHERE is_active = false) AS total_revoked,
			COUNT(*) FILTER (WHERE expires_at <= NOW()) AS total_expired,
			COUNT(DISTINCT user_id) AS unique_users_active,
			COALESCE(AVG(EXTRACT(EPOCH FROM (NOW() - last_activity)) / 60) FILTER (WHERE is_active = true), 0) AS avg_idle_minutes
		FROM user_sessions
		WHERE user_id = $1
	`

	var stats entity.SessionStatistics
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.TotalActive,
		&stats.TotalRevoked,
		&stats.TotalExpired,
		&stats.UniqueUsersActive,
		&stats.AvgIdleMinutes,
	)

	if err != nil {
		return nil, fmt.Errorf("get session statistics by user: %w", err)
	}

	return &stats, nil
}

// ListActiveSessions liste les sessions actives avec pagination
func (r *UserSessionRepositoryImpl) ListActiveSessions(ctx context.Context, limit, offset int) ([]*entity.UserSession, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `
		SELECT COUNT(*) FROM user_sessions 
		WHERE is_active = true AND expires_at > NOW()
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count active sessions: %w", err)
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE is_active = true AND expires_at > NOW()
		ORDER BY last_activity DESC
		LIMIT $1 OFFSET $2
	`

	sessions, err := r.scanSessions(r.db.QueryContext(ctx, query, limit, offset))
	if err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

// ListActiveSessionsByUser liste les sessions actives d'un user
func (r *UserSessionRepositoryImpl) ListActiveSessionsByUser(ctx context.Context, userID string, limit, offset int) ([]*entity.UserSession, int, error) {
	if userID == "" {
		return nil, 0, repository.ErrSessionInvalidData
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `
		SELECT COUNT(*) FROM user_sessions 
		WHERE user_id = $1 AND is_active = true AND expires_at > NOW()
	`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count active sessions by user: %w", err)
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE user_id = $1 AND is_active = true AND expires_at > NOW()
		ORDER BY last_activity DESC
		LIMIT $2 OFFSET $3
	`

	sessions, err := r.scanSessions(r.db.QueryContext(ctx, query, userID, limit, offset))
	if err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

// ============================================================
// MÉTHODES D'ANALYSE
// ============================================================

// FindSessionsByIP trouve toutes les sessions depuis une IP
func (r *UserSessionRepositoryImpl) FindSessionsByIP(ctx context.Context, ipAddress string) ([]*entity.UserSession, error) {
	if ipAddress == "" {
		return nil, repository.ErrSessionInvalidData
	}

	query := `
		SELECT 
			id, user_id, session_token_hash, session_id,
			device_info, ip_address, last_activity,
			created_at, expires_at, is_active,
			revoked_at, revoked_by
		FROM user_sessions
		WHERE ip_address = $1
		ORDER BY last_activity DESC
	`

	return r.scanSessions(r.db.QueryContext(ctx, query, ipAddress))
}

// FindDuplicateSessions trouve les users avec plusieurs sessions actives
func (r *UserSessionRepositoryImpl) FindDuplicateSessions(ctx context.Context, maxSessionsPerUser int) ([]*entity.UserSession, error) {
	if maxSessionsPerUser <= 0 {
		maxSessionsPerUser = 3
	}

	query := `
		SELECT 
			us.id, us.user_id, us.session_token_hash, us.session_id,
			us.device_info, us.ip_address, us.last_activity,
			us.created_at, us.expires_at, us.is_active,
			us.revoked_at, us.revoked_by
		FROM user_sessions us
		INNER JOIN (
			SELECT user_id 
			FROM user_sessions 
			WHERE is_active = true AND expires_at > NOW()
			GROUP BY user_id 
			HAVING COUNT(*) > $1
		) dup ON us.user_id = dup.user_id
		WHERE us.is_active = true AND us.expires_at > NOW()
		ORDER BY us.user_id, us.last_activity DESC
	`

	return r.scanSessions(r.db.QueryContext(ctx, query, maxSessionsPerUser))
}

// GetMostActiveUsers retourne les users avec le plus de sessions
func (r *UserSessionRepositoryImpl) GetMostActiveUsers(ctx context.Context, limit int) ([]repository.UserSessionCount, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			us.user_id,
			u.email,
			u.role,
			COUNT(*) AS session_count,
			MAX(us.last_activity) AS last_activity
		FROM user_sessions us
		JOIN users u ON us.user_id = u.id
		WHERE us.is_active = true AND us.expires_at > NOW()
		GROUP BY us.user_id, u.email, u.role
		ORDER BY session_count DESC, last_activity DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get most active users: %w", err)
	}
	defer rows.Close()

	var counts []repository.UserSessionCount
	for rows.Next() {
		var c repository.UserSessionCount
		if err := rows.Scan(
			&c.UserID,
			&c.Email,
			&c.Role,
			&c.SessionCount,
			&c.LastActivity,
		); err != nil {
			return nil, fmt.Errorf("scan user session count: %w", err)
		}
		counts = append(counts, c)
	}

	return counts, nil
}

// ============================================================
// HELPERS
// ============================================================

// scanSession scanne une seule session
func (r *UserSessionRepositoryImpl) scanSession(row *sql.Row) (*entity.UserSession, error) {
	var session entity.UserSession
	var deviceInfoJSON string
	var revokedAt sql.NullTime
	var revokedBy sql.NullString

	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.SessionTokenHash,
		&session.SessionID,
		&deviceInfoJSON,
		&session.IPAddress,
		&session.LastActivity,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.IsActive,
		&revokedAt,
		&revokedBy,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrSessionNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}

	// Parser device_info JSON
	if deviceInfoJSON != "" {
		if err := json.Unmarshal([]byte(deviceInfoJSON), &session.DeviceInfo); err != nil {
			session.DeviceInfo = entity.DeviceInfo{}
		}
	}

	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}
	if revokedBy.Valid {
		session.RevokedBy = revokedBy.String
	}

	return &session, nil
}

// scanSessions scanne plusieurs sessions
func (r *UserSessionRepositoryImpl) scanSessions(rows *sql.Rows, err error) ([]*entity.UserSession, error) {
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*entity.UserSession
	for rows.Next() {
		var session entity.UserSession
		var deviceInfoJSON string
		var revokedAt sql.NullTime
		var revokedBy sql.NullString

		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.SessionTokenHash,
			&session.SessionID,
			&deviceInfoJSON,
			&session.IPAddress,
			&session.LastActivity,
			&session.CreatedAt,
			&session.ExpiresAt,
			&session.IsActive,
			&revokedAt,
			&revokedBy,
		); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}

		if deviceInfoJSON != "" {
			if err := json.Unmarshal([]byte(deviceInfoJSON), &session.DeviceInfo); err != nil {
				session.DeviceInfo = entity.DeviceInfo{}
			}
		}

		if revokedAt.Valid {
			session.RevokedAt = &revokedAt.Time
		}
		if revokedBy.Valid {
			session.RevokedBy = revokedBy.String
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// isUniqueViolation vérifie si l'erreur est une violation d'unicité
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "unique constraint") || contains(errMsg, "duplicate key")
}

// contains vérifie si une chaîne contient une sous-chaîne
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
