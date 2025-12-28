package authrepository

import authentity "Goshop/domain/auth_entity"

type RefreshSessionRepository interface {
	Create(session *authentity.RefreshSession) error
	FindByID(id string) (*authentity.RefreshSession, error)
	Revoke(id string) error
}
