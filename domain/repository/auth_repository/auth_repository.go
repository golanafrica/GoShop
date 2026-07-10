package authrepository

import authentity "Goshop/domain/auth_entity"

//go:generate mockgen -destination=../../../mocks/auth_repository/mock_refresh_session_repository.go -package=auth_repository . RefreshSessionRepository

type RefreshSessionRepository interface {
	Create(session *authentity.RefreshSession) error
	FindByID(id string) (*authentity.RefreshSession, error)
	Revoke(id string) error
}
