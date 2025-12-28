// interfaces/handler/customer_handler/compat.go
package customerhandler // IMPORTANT: même nom que le package principal

import (
	"Goshop/domain/repository"
)

// Ancien constructeur pour compatibilité avec les tests existants
func NewCustomerHandlerOld(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
) *CustomerHandler { // NOTE: CustomerHandler avec majuscule
	return NewCustomerHandler(repo, txManager)
}
