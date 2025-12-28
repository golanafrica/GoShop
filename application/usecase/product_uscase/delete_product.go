// application/usecase/product_uscase/delete_product_usecase.go
package productuscase

import (
	"context"
	"database/sql"
	"time"

	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type DeleteProductUsecase struct {
	repo      repository.ProductRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewDeleteProductUsecase(
	repo repository.ProductRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *DeleteProductUsecase {
	return &DeleteProductUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("delete_product"),
	}
}

func (uc *DeleteProductUsecase) Execute(ctx context.Context, id string) error {
	logger := zerolog.Ctx(ctx)
	if id == "" {
		logger.Warn().
			Str("operation", "execute").
			Msg("Product ID is empty")
		return utils.ErrProductNotFound
	}

	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Starting product deletion")

	// Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Beginning database transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", id).
			Msg("Failed to begin transaction for product deletion")
		return utils.ErrTransactionBegin
	}

	// Rollback seulement en cas d'erreur
	defer func() {
		if err != nil {
			logger.Warn().
				Str("operation", "execute").
				Str("product_id", id).
				Msg("Rolling back transaction due to error")
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("operation", "execute").
					Str("product_id", id).
					Msg("Failed to rollback transaction after error")
			}
		}
	}()

	// Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Repository attached to transaction")

	// Vérifier si le produit existe
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Checking if product exists before deletion")

	// Utiliser la même variable err pour que le defer fonctionne
	product, err := repo.FindByID(ctx, id)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("operation", "execute").
			Str("product_id", id).
			Msg("Product not found for deletion")
		return utils.ErrProductNotFound
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Str("product_name", product.Name).
		Msg("Product found, proceeding with deletion")

	// Suppression du produit
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Deleting product from repository")

	if err = repo.Delete(ctx, id); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", id).
			Str("product_name", product.Name).
			Msg("Failed to delete product from repository")
		return utils.ErrProductDeleteFail
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Str("product_name", product.Name).
		Msg("Product deleted successfully from repository")

	// Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Msg("Committing deletion transaction")

	if err = tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", id).
			Str("product_name", product.Name).
			Msg("Failed to commit deletion transaction")
		return utils.ErrTransactionCommit
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", id).
		Str("product_name", product.Name).
		Msg("Transaction committed successfully")

	// Log de succès
	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Str("product_id", id).
		Str("product_name", product.Name).
		Dur("duration_ms", duration).
		Msg("Product deletion completed successfully")

	return nil
}
