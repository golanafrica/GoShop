// application/usecase/product_uscase/update_product_usecase.go
package productuscase

import (
	"context"
	"database/sql"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type UpdateProductUsecase struct {
	repo      repository.ProductRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewUpdateProductUsecase(
	repo repository.ProductRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *UpdateProductUsecase {
	return &UpdateProductUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("update_product"),
	}
}

func (uc *UpdateProductUsecase) Execute(ctx context.Context, product *entity.Product) (*entity.Product, error) {
	logger := zerolog.Ctx(ctx)
	if product == nil {
		logger.Warn().
			Str("operation", "execute").
			Msg("Received nil product")
		return nil, utils.ErrProductUpdateFail
	}

	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Str("product_name", product.Name).
		Int("price_cents", int(product.PriceCents)).
		Int("stock", product.Stock).
		Msg("Starting product update")

	// Validation des données
	if err := uc.validateProductData(ctx, product); err != nil {
		return nil, err
	}

	// Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Msg("Beginning database transaction for update")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", product.ID).
			Msg("Failed to begin transaction for product update")
		return nil, utils.ErrTransactionBegin
	}

	// Rollback seulement en cas d'erreur
	defer func() {
		if err != nil {
			logger.Warn().
				Str("operation", "execute").
				Str("product_id", product.ID).
				Msg("Rolling back transaction due to error")
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("operation", "execute").
					Str("product_id", product.ID).
					Msg("Failed to rollback transaction")
			}
		}
	}()

	// Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Msg("Repository attached to transaction")

	// Vérifier si le produit existe (utiliser la même variable err)
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Msg("Checking if product exists")

	existing, err := repo.FindByID(ctx, product.ID)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("operation", "execute").
			Str("product_id", product.ID).
			Msg("Product not found for update")
		return nil, utils.ErrProductNotFound
	}

	// Log des changements
	uc.logChanges(ctx, existing, product)

	// Appliquer les modifications
	existing.Name = product.Name
	existing.Description = product.Description
	existing.PriceCents = product.PriceCents
	existing.Stock = product.Stock

	// Mise à jour du produit
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Msg("Updating product in repository")

	updatedProduct, err := repo.Update(ctx, existing)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", product.ID).
			Int("old_price", int(existing.PriceCents)).
			Int("new_price", int(product.PriceCents)).
			Int("old_stock", existing.Stock).
			Int("new_stock", product.Stock).
			Msg("Failed to update product in repository")
		return nil, utils.ErrProductUpdateFail
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", updatedProduct.ID).
		Str("product_name", updatedProduct.Name).
		Msg("Product updated successfully in repository")

	// Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", updatedProduct.ID).
		Msg("Committing update transaction")

	if err = tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", updatedProduct.ID).
			Msg("Failed to commit update transaction")
		return nil, utils.ErrTransactionCommit
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", updatedProduct.ID).
		Msg("Transaction committed successfully")

	// Log de succès
	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Str("product_id", updatedProduct.ID).
		Str("product_name", updatedProduct.Name).
		Dur("duration_ms", duration).
		Msg("Product update completed successfully")

	return updatedProduct, nil
}

func (uc *UpdateProductUsecase) validateProductData(ctx context.Context, product *entity.Product) error {
	logger := zerolog.Ctx(ctx)
	if product.Name == "" {
		logger.Warn().
			Str("operation", "validate").
			Str("product_id", product.ID).
			Msg("Product name validation failed - empty name")
		return utils.ErrProductInvalidName
	}

	if product.PriceCents <= 0 {
		logger.Warn().
			Str("operation", "validate").
			Str("product_id", product.ID).
			Int("price_cents", int(product.PriceCents)).
			Msg("Product price validation failed - invalid price")
		return utils.ErrProductInvalidPrice
	}

	if product.Stock < 0 {
		logger.Warn().
			Str("operation", "validate").
			Str("product_id", product.ID).
			Int("stock", product.Stock).
			Msg("Product stock validation failed - negative stock")
		return utils.ErrProductInvalidStock
	}

	if product.ID == "" {
		logger.Warn().
			Str("operation", "validate").
			Msg("Product ID validation failed - empty ID")
		return utils.ErrProductNotFound
	}

	logger.Debug().
		Str("operation", "validate").
		Str("product_id", product.ID).
		Msg("All product data validations passed")
	return nil
}

func (uc *UpdateProductUsecase) logChanges(ctx context.Context, existing, new *entity.Product) {
	logger := zerolog.Ctx(ctx)
	changes := make(map[string]interface{})

	if existing.Name != new.Name {
		changes["name"] = map[string]string{
			"old": existing.Name,
			"new": new.Name,
		}
	}

	if existing.Description != new.Description {
		descChanges := map[string]interface{}{
			"old_length": len(existing.Description),
			"new_length": len(new.Description),
		}
		if len(existing.Description) > 0 || len(new.Description) > 0 {
			descChanges["old_preview"] = truncateString(existing.Description, 50)
			descChanges["new_preview"] = truncateString(new.Description, 50)
		}
		changes["description"] = descChanges
	}

	if existing.PriceCents != new.PriceCents {
		changes["price"] = map[string]interface{}{
			"old":    existing.PriceCents,
			"new":    new.PriceCents,
			"change": new.PriceCents - existing.PriceCents,
		}
	}

	if existing.Stock != new.Stock {
		changes["stock"] = map[string]interface{}{
			"old":    existing.Stock,
			"new":    new.Stock,
			"change": new.Stock - existing.Stock,
		}
	}

	if len(changes) > 0 {
		logger.Info().
			Str("operation", "log_changes").
			Str("product_id", existing.ID).
			Interface("changes", changes).
			Int("total_changes", len(changes)).
			Msg("Detected changes for product update")
	} else {
		logger.Warn().
			Str("operation", "log_changes").
			Str("product_id", existing.ID).
			Msg("No changes detected for product update")
	}
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}
