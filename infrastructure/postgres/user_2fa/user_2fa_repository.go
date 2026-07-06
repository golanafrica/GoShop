package user2fa

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
	"github.com/lib/pq"
)

// ============================================================
// 🆕 v4.4.0 : USER 2FA REPOSITORY - POSTGRES IMPLEMENTATION
// 🆕 v4.4.1 : + Protection anti-replay (RecordCodeUsed)
// ============================================================

// User2FARepositoryImpl implémente repository.User2FARepository
type User2FARepositoryImpl struct {
	db *sql.DB
}

// NewUser2FARepository crée une nouvelle instance du repository
func NewUser2FARepository(db *sql.DB) *User2FARepositoryImpl {
	return &User2FARepositoryImpl{
		db: db,
	}
}

// ============================================================
// MÉTHODES CRUD
// ============================================================

// Create crée une nouvelle configuration 2FA
func (r *User2FARepositoryImpl) Create(ctx context.Context, user2fa *entity.User2FA) error {
	if user2fa == nil {
		return repository.ErrUser2FAInvalidData
	}

	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO user_2fa (
			id, user_id, secret_encrypted, is_enabled,
			recovery_codes_encrypted, recovery_codes_used,
			failed_attempts, locked_until, last_verified_at,
			last_used_code, last_used_at,
			setup_at, enabled_at, disabled_at,
			last_ip_address, last_user_agent,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		id,
		user2fa.UserID,
		user2fa.SecretEncrypted,
		user2fa.IsEnabled,
		user2fa.RecoveryCodesEncrypted,
		pq.Array(user2fa.RecoveryCodesUsed),
		user2fa.FailedAttempts,
		user2fa.LockedUntil,
		user2fa.LastVerifiedAt,
		user2fa.LastUsedCode, // 🆕 v4.4.1
		user2fa.LastUsedAt,   // 🆕 v4.4.1
		user2fa.SetupAt,
		user2fa.EnabledAt,
		user2fa.DisabledAt,
		user2fa.LastIPAddress,
		user2fa.LastUserAgent,
		now,
		now,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return repository.ErrUser2FAAlreadyExists
		}
		return fmt.Errorf("create user_2fa: %w", err)
	}

	user2fa.ID = id
	user2fa.CreatedAt = now
	user2fa.UpdatedAt = now

	return nil
}

// FindByUserID récupère la configuration 2FA d'un utilisateur
func (r *User2FARepositoryImpl) FindByUserID(ctx context.Context, userID string) (*entity.User2FA, error) {
	if userID == "" {
		return nil, repository.ErrUser2FAInvalidData
	}

	query := `
		SELECT 
			id, user_id, secret_encrypted, is_enabled,
			recovery_codes_encrypted, recovery_codes_used,
			failed_attempts, locked_until, last_verified_at,
			last_used_code, last_used_at,
			setup_at, enabled_at, disabled_at,
			last_ip_address, last_user_agent,
			created_at, updated_at
		FROM user_2fa
		WHERE user_id = $1
	`

	row := r.db.QueryRowContext(ctx, query, userID)

	var user2fa entity.User2FA
	var recoveryCodesUsed []int // 🆕 v4.4.1 : Déclaration correcte
	var lockedUntil, lastVerifiedAt, lastUsedAt, setupAt, enabledAt, disabledAt sql.NullTime
	var lastUsedCode sql.NullString

	err := row.Scan(
		&user2fa.ID,
		&user2fa.UserID,
		&user2fa.SecretEncrypted,
		&user2fa.IsEnabled,
		&user2fa.RecoveryCodesEncrypted,
		pq.Array(&recoveryCodesUsed), // 🆕 v4.4.1 : Scan dans la variable déclarée
		&user2fa.FailedAttempts,
		&lockedUntil,
		&lastVerifiedAt,
		&lastUsedCode,
		&lastUsedAt,
		&setupAt,
		&enabledAt,
		&disabledAt,
		&user2fa.LastIPAddress,
		&user2fa.LastUserAgent,
		&user2fa.CreatedAt,
		&user2fa.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrUser2FANotFound
		}
		return nil, fmt.Errorf("find user_2fa by user_id: %w", err)
	}

	// 🆕 v4.4.1 : Assignation correcte
	user2fa.RecoveryCodesUsed = recoveryCodesUsed
	if lockedUntil.Valid {
		user2fa.LockedUntil = &lockedUntil.Time
	}
	if lastVerifiedAt.Valid {
		user2fa.LastVerifiedAt = &lastVerifiedAt.Time
	}
	if lastUsedCode.Valid {
		user2fa.LastUsedCode = lastUsedCode.String
	}
	if lastUsedAt.Valid {
		user2fa.LastUsedAt = &lastUsedAt.Time
	}
	if setupAt.Valid {
		user2fa.SetupAt = &setupAt.Time
	}
	if enabledAt.Valid {
		user2fa.EnabledAt = &enabledAt.Time
	}
	if disabledAt.Valid {
		user2fa.DisabledAt = &disabledAt.Time
	}

	return &user2fa, nil
}

// Update met à jour la configuration 2FA
func (r *User2FARepositoryImpl) Update(ctx context.Context, user2fa *entity.User2FA) error {
	if user2fa == nil {
		return repository.ErrUser2FAInvalidData
	}

	query := `
		UPDATE user_2fa SET
			secret_encrypted = $1,
			is_enabled = $2,
			recovery_codes_encrypted = $3,
			recovery_codes_used = $4,
			failed_attempts = $5,
			locked_until = $6,
			last_verified_at = $7,
			last_used_code = $8,
			last_used_at = $9,
			enabled_at = $10,
			disabled_at = $11,
			last_ip_address = $12,
			last_user_agent = $13,
			updated_at = $14
		WHERE user_id = $15
	`

	result, err := r.db.ExecContext(ctx, query,
		user2fa.SecretEncrypted,
		user2fa.IsEnabled,
		user2fa.RecoveryCodesEncrypted,
		pq.Array(user2fa.RecoveryCodesUsed),
		user2fa.FailedAttempts,
		user2fa.LockedUntil,
		user2fa.LastVerifiedAt,
		user2fa.LastUsedCode,
		user2fa.LastUsedAt,
		user2fa.EnabledAt,
		user2fa.DisabledAt,
		user2fa.LastIPAddress,
		user2fa.LastUserAgent,
		time.Now(),
		user2fa.UserID,
	)

	if err != nil {
		return fmt.Errorf("update user_2fa: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// Delete supprime complètement la configuration 2FA
func (r *User2FARepositoryImpl) Delete(ctx context.Context, userID string) error {
	if userID == "" {
		return repository.ErrUser2FAInvalidData
	}

	query := `DELETE FROM user_2fa WHERE user_id = $1`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user_2fa: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// Exists vérifie si une configuration 2FA existe
func (r *User2FARepositoryImpl) Exists(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, repository.ErrUser2FAInvalidData
	}

	query := `SELECT EXISTS(SELECT 1 FROM user_2fa WHERE user_id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user_2fa exists: %w", err)
	}

	return exists, nil
}

// ============================================================
// MÉTHODES SPÉCIFIQUES 2FA
// ============================================================

// UpdateSecret met à jour le secret TOTP chiffré
func (r *User2FARepositoryImpl) UpdateSecret(ctx context.Context, userID string, secretEncrypted string) error {
	query := `
		UPDATE user_2fa 
		SET secret_encrypted = $1, updated_at = NOW()
		WHERE user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, secretEncrypted, userID)
	if err != nil {
		return fmt.Errorf("update user_2fa secret: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// Enable2FA active la 2FA
func (r *User2FARepositoryImpl) Enable2FA(ctx context.Context, userID string) error {
	query := `
		UPDATE user_2fa 
		SET is_enabled = true, enabled_at = NOW(), updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("enable user_2fa: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// Disable2FA désactive la 2FA
func (r *User2FARepositoryImpl) Disable2FA(ctx context.Context, userID string) error {
	query := `
		UPDATE user_2fa 
		SET 
			is_enabled = false,
			disabled_at = NOW(),
			secret_encrypted = '',
			recovery_codes_encrypted = '',
			recovery_codes_used = '{}',
			failed_attempts = 0,
			locked_until = NULL,
			last_used_code = NULL,
			last_used_at = NULL,
			updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("disable user_2fa: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// UpdateRecoveryCodes met à jour les codes de récupération
func (r *User2FARepositoryImpl) UpdateRecoveryCodes(ctx context.Context, userID string, codesEncrypted string) error {
	query := `
		UPDATE user_2fa 
		SET recovery_codes_encrypted = $1, recovery_codes_used = '{}', updated_at = NOW()
		WHERE user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, codesEncrypted, userID)
	if err != nil {
		return fmt.Errorf("update recovery codes: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// MarkRecoveryCodeUsed marque un code comme utilisé
func (r *User2FARepositoryImpl) MarkRecoveryCodeUsed(ctx context.Context, userID string, codeIndex int) error {
	query := `
		UPDATE user_2fa 
		SET recovery_codes_used = array_append(recovery_codes_used, $1), updated_at = NOW()
		WHERE user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, codeIndex, userID)
	if err != nil {
		return fmt.Errorf("mark recovery code used: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// ResetRecoveryCodes réinitialise tous les codes
func (r *User2FARepositoryImpl) ResetRecoveryCodes(ctx context.Context, userID string) error {
	query := `
		UPDATE user_2fa 
		SET recovery_codes_used = '{}', updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("reset recovery codes: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// ============================================================
// MÉTHODES DE SÉCURITÉ
// ============================================================

// IncrementFailedAttempts incrémente le compteur d'échecs
func (r *User2FARepositoryImpl) IncrementFailedAttempts(ctx context.Context, userID string) error {
	query := `
		UPDATE user_2fa 
		SET 
			failed_attempts = failed_attempts + 1,
			locked_until = CASE 
				WHEN failed_attempts + 1 >= 5 THEN NOW() + INTERVAL '15 minutes'
				ELSE locked_until
			END,
			updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("increment failed attempts: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// ResetFailedAttempts réinitialise le compteur
func (r *User2FARepositoryImpl) ResetFailedAttempts(ctx context.Context, userID string) error {
	query := `
		UPDATE user_2fa 
		SET failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("reset failed attempts: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// LockAccount verrouille le compte
func (r *User2FARepositoryImpl) LockAccount(ctx context.Context, userID string, durationMinutes int) error {
	query := `
		UPDATE user_2fa 
		SET locked_until = NOW() + make_interval(mins => $1), updated_at = NOW()
		WHERE user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, durationMinutes, userID)
	if err != nil {
		return fmt.Errorf("lock account: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// UnlockAccount déverrouille le compte
func (r *User2FARepositoryImpl) UnlockAccount(ctx context.Context, userID string) error {
	query := `
		UPDATE user_2fa 
		SET failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("unlock account: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// IsAccountLocked vérifie si le compte est verrouillé
func (r *User2FARepositoryImpl) IsAccountLocked(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_2fa 
			WHERE user_id = $1 AND locked_until > NOW()
		)
	`

	var locked bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("check account locked: %w", err)
	}

	return locked, nil
}

// ============================================================
// MÉTHODES D'AUDIT
// ============================================================

// RecordSuccessfulVerification enregistre une vérification réussie
func (r *User2FARepositoryImpl) RecordSuccessfulVerification(ctx context.Context, userID string, ipAddress, userAgent string) error {
	query := `
		UPDATE user_2fa 
		SET 
			failed_attempts = 0,
			locked_until = NULL,
			last_verified_at = NOW(),
			last_ip_address = $2,
			last_user_agent = $3,
			updated_at = NOW()
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID, ipAddress, userAgent)
	if err != nil {
		return fmt.Errorf("record successful verification: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// UpdateLastActivity met à jour la dernière activité
func (r *User2FARepositoryImpl) UpdateLastActivity(ctx context.Context, userID string, ipAddress, userAgent string) error {
	query := `
		UPDATE user_2fa 
		SET last_ip_address = $1, last_user_agent = $2, updated_at = NOW()
		WHERE user_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, ipAddress, userAgent, userID)
	if err != nil {
		return fmt.Errorf("update last activity: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// ============================================================
// MÉTHODES DE STATISTIQUES
// ============================================================

// CountEnabled compte les 2FA activées
func (r *User2FARepositoryImpl) CountEnabled(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM user_2fa WHERE is_enabled = true`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count enabled 2fa: %w", err)
	}

	return count, nil
}

// CountLocked compte les comptes verrouillés
func (r *User2FARepositoryImpl) CountLocked(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM user_2fa WHERE locked_until > NOW()`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count locked accounts: %w", err)
	}

	return count, nil
}

// GetStatistics retourne les statistiques globales
func (r *User2FARepositoryImpl) GetStatistics(ctx context.Context) (*repository.User2FAStatistics, error) {
	query := `
		SELECT 
			COUNT(*) FILTER (WHERE is_enabled = true) AS total_enabled,
			COUNT(*) FILTER (WHERE is_enabled = false) AS total_disabled,
			COUNT(*) FILTER (WHERE locked_until > NOW()) AS total_locked,
			COUNT(*) AS total_users_with_2fa,
			COALESCE(AVG(failed_attempts), 0) AS avg_failed_attempts,
			COALESCE(MAX(failed_attempts), 0) AS max_failed_attempts
		FROM user_2fa
	`

	var stats repository.User2FAStatistics
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalEnabled,
		&stats.TotalDisabled,
		&stats.TotalLocked,
		&stats.TotalUsersWith2FA,
		&stats.AvgFailedAttempts,
		&stats.MaxFailedAttempts,
	)

	if err != nil {
		return nil, fmt.Errorf("get 2fa statistics: %w", err)
	}

	if stats.TotalUsersWith2FA > 0 {
		stats.AdoptionRate = float64(stats.TotalEnabled) / float64(stats.TotalUsersWith2FA) * 100
	}

	return &stats, nil
}

// ListEnabledUsers liste les utilisateurs avec 2FA activée
func (r *User2FARepositoryImpl) ListEnabledUsers(ctx context.Context, limit, offset int) ([]*entity.User2FA, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM user_2fa WHERE is_enabled = true`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count enabled users: %w", err)
	}

	query := `
		SELECT 
			id, user_id, secret_encrypted, is_enabled,
			recovery_codes_encrypted, recovery_codes_used,
			failed_attempts, locked_until, last_verified_at,
			last_used_code, last_used_at,
			setup_at, enabled_at, disabled_at,
			last_ip_address, last_user_agent,
			created_at, updated_at
		FROM user_2fa
		WHERE is_enabled = true
		ORDER BY enabled_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list enabled users: %w", err)
	}
	defer rows.Close()

	var users []*entity.User2FA
	for rows.Next() {
		var user2fa entity.User2FA
		var recoveryCodesUsed []int
		var lockedUntil, lastVerifiedAt, lastUsedAt, setupAt, enabledAt, disabledAt sql.NullTime
		var lastUsedCode sql.NullString

		err := rows.Scan(
			&user2fa.ID,
			&user2fa.UserID,
			&user2fa.SecretEncrypted,
			&user2fa.IsEnabled,
			&user2fa.RecoveryCodesEncrypted,
			pq.Array(&recoveryCodesUsed),
			&user2fa.FailedAttempts,
			&lockedUntil,
			&lastVerifiedAt,
			&lastUsedCode,
			&lastUsedAt,
			&setupAt,
			&enabledAt,
			&disabledAt,
			&user2fa.LastIPAddress,
			&user2fa.LastUserAgent,
			&user2fa.CreatedAt,
			&user2fa.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user_2fa: %w", err)
		}

		user2fa.RecoveryCodesUsed = recoveryCodesUsed
		if lockedUntil.Valid {
			user2fa.LockedUntil = &lockedUntil.Time
		}
		if lastVerifiedAt.Valid {
			user2fa.LastVerifiedAt = &lastVerifiedAt.Time
		}
		if lastUsedCode.Valid {
			user2fa.LastUsedCode = lastUsedCode.String
		}
		if lastUsedAt.Valid {
			user2fa.LastUsedAt = &lastUsedAt.Time
		}
		if setupAt.Valid {
			user2fa.SetupAt = &setupAt.Time
		}
		if enabledAt.Valid {
			user2fa.EnabledAt = &enabledAt.Time
		}
		if disabledAt.Valid {
			user2fa.DisabledAt = &disabledAt.Time
		}

		users = append(users, &user2fa)
	}

	return users, total, nil
}

// ============================================================
// 🆕 v4.4.1 : PROTECTION ANTI-REPLAY
// ============================================================

// RecordCodeUsed enregistre un code TOTP comme utilisé
func (r *User2FARepositoryImpl) RecordCodeUsed(ctx context.Context, userID string, code string) error {
	query := `
		UPDATE user_2fa 
		SET 
			last_used_code = $1,
			last_used_at = NOW(),
			updated_at = NOW()
		WHERE user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, code, userID)
	if err != nil {
		return fmt.Errorf("record code used: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return repository.ErrUser2FANotFound
	}

	return nil
}

// ============================================================
// HELPERS
// ============================================================

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "unique constraint") || contains(errMsg, "duplicate key")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Assurer que json est utilisé
var _ = json.Marshal
