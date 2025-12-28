// application/usecase/product_uscase/compat.go
// application/usecase/product_uscase/compat.go
package productuscase

import (
	"Goshop/domain/repository"
)

// Anciens constructeurs pour compatibilité avec les tests existants

func NewCreateProductUsecaseOld(repo repository.ProductRepository, tx repository.TxManager) *CreateProductUsecase {
	return NewCreateProductUsecase(repo, tx)
}

func NewDeleteProductUsecaseOld(repo repository.ProductRepository, tx repository.TxManager) *DeleteProductUsecase {
	return NewDeleteProductUsecase(repo, tx)
}

func NewUpdateProductUsecaseOld(repo repository.ProductRepository, tx repository.TxManager) *UpdateProductUsecase {
	return NewUpdateProductUsecase(repo, tx)
}

// CORRECTION: GetProductByIdUsecase a besoin de txManager aussi !
func NewGetProductByIdUsecaseOld(repo repository.ProductRepository, tx repository.TxManager) *GetProductByIdUsecase {
	return NewGetProductByIdUsecase(repo, tx)
}

// CORRECTION: ListProductUsecase a besoin de txManager aussi !
func NewListProductUsecaseOld(repo repository.ProductRepository, tx repository.TxManager) *ListProductUsecase {
	return NewListProductUsecase(repo, tx)
}
