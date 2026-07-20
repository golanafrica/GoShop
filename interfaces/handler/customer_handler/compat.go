// interfaces/handler/customer_handler/compat.go
package customerhandler

import (
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository"
)

// Ancien constructeur pour compatibilité avec les tests existants
// NOTE: Il est recommandé de migrer vers NewCustomerHandler directement avec le userRepo.
func NewCustomerHandlerOld(
	repo repository.CustomerRepositoryInterface,
	userRepo userrepository.UserRepository,
	txManager repository.TxManager,
) *CustomerHandler {
	return NewCustomerHandler(repo, userRepo, txManager)
}
