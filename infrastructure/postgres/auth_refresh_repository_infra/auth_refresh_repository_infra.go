package authrefreshrepositoryinfra

import (
	authentity "Goshop/domain/auth_entity"
	authrepository "Goshop/domain/repository/auth_repository"
	"database/sql"
	"time"
)

type RefreshSessionPostgres struct {
	db *sql.DB
}

func NewRefreshSessionPostgres(db *sql.DB) authrepository.RefreshSessionRepository {
	return &RefreshSessionPostgres{db: db}
}

func (r *RefreshSessionPostgres) Create(session *authentity.RefreshSession) error {
	query := `
	INSERT INTO refresh_sessions (id, user_id, expires_at, revoked, created_at)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (id) DO UPDATE SET revoked = EXCLUDED.revoked, expires_at = EXCLUDED.expires_at
	`
	_, err := r.db.Exec(query, session.ID, session.UserID, session.ExpiresAt.UTC(), session.Revoked, session.CreatedAt.UTC())
	return err
}

func (r *RefreshSessionPostgres) FindByID(id string) (*authentity.RefreshSession, error) {
	query := `
	SELECT id, user_id, expires_at, revoked, created_at
	FROM refresh_sessions
	WHERE id = $1
	`
	row := r.db.QueryRow(query, id)
	var s authentity.RefreshSession
	var expiresAt time.Time
	var createdAt time.Time
	if err := row.Scan(&s.ID, &s.UserID, &expiresAt, &s.Revoked, &createdAt); err != nil {
		return nil, err
	}
	s.ExpiresAt = expiresAt
	s.CreatedAt = createdAt
	return &s, nil
}

func (r *RefreshSessionPostgres) Revoke(id string) error {
	query := `
	UPDATE refresh_sessions SET revoked = true WHERE id = $1
	`
	_, err := r.db.Exec(query, id)
	return err
}
