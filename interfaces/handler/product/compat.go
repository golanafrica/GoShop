// interfaces/handler/product/compat.go
package producthandler

import (
	"Goshop/domain/repository"
)

// Ancien constructeur pour compatibilité avec les tests existants
func NewProductHandlerOld(
	repo repository.ProductRepository,
	txManager repository.TxManager,
) *ProductHandler {
	return NewProductHandler(repo, txManager)
}
