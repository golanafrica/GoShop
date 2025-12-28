// application/usecase/product_uscase/create_product_usecase.go
package productuscase

import (
	"context"
	"database/sql"
	"time"

	dto "Goshop/application/dto/product_dto"
	"Goshop/application/metrics"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"

	"github.com/rs/zerolog"
)

type CreateProductUsecase struct {
	repo      repository.ProductRepository
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewCreateProductUsecase(
	repo repository.ProductRepository,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *CreateProductUsecase {
	return &CreateProductUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("create_product"),
	}
}

func (uc *CreateProductUsecase) Execute(ctx context.Context, input dto.CreateProductRequest) (*dto.ProductResponse, error) {
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	// ✅ Pas de logger local — utilise uc.logger directement
	logger.Info().
		Str("operation", "execute").
		Str("product_name", input.Name).
		Int("price_cents", int(input.PriceCents)).
		Int("stock", input.Stock).
		Msg("Starting product creation")

	// Validation
	if input.Name == "" {
		logger.Warn().
			Str("operation", "validate").
			Str("field", "name").
			Msg("Product name validation failed - empty name")
		return nil, utils.ErrProductInvalidName
	}

	if input.PriceCents <= 0 {
		logger.Warn().
			Str("operation", "validate").
			Str("field", "price_cents").
			Int("value", int(input.PriceCents)).
			Msg("Product price validation failed - invalid price")
		return nil, utils.ErrProductInvalidPrice
	}

	if input.Stock < 0 {
		logger.Warn().
			Str("operation", "validate").
			Str("field", "stock").
			Int("value", input.Stock).
			Msg("Product stock validation failed - negative stock")
		return nil, utils.ErrProductInvalidStock
	}

	logger.Debug().
		Str("operation", "validate").
		Str("product_name", input.Name).
		Msg("All validations passed")

	// Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("product_name", input.Name).
		Msg("Beginning database transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_name", input.Name).
			Msg("Failed to begin transaction")
		return nil, utils.ErrTransactionBegin
	}

	// Rollback sécurisé (seulement si erreur)
	defer func() {
		if err != nil {
			logger.Warn().
				Str("operation", "execute").
				Str("product_name", input.Name).
				Msg("Rolling back transaction due to error")
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("operation", "execute").
					Str("product_name", input.Name).
					Msg("Failed to rollback transaction")
			}
		}
	}()

	// Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("product_name", input.Name).
		Msg("Repository attached to transaction")

	// Création de l'entité produit
	product := &entity.Product{
		Name:        input.Name,
		Description: input.Description,
		PriceCents:  input.PriceCents,
		Stock:       input.Stock,
	}

	// Log des données (corrigé : description_length en int)
	logger.Debug().
		Str("operation", "execute").
		Str("product_name", input.Name).
		Int("description_length", len(input.Description)).
		Msg("Creating product in repository")

	// Création du produit
	if err = repo.Create(ctx, product); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_name", product.Name).
			Msg("Failed to create product in repository")
		return nil, utils.ErrProductCreateFail
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Str("product_name", product.Name).
		Msg("Product created successfully in repository")

	// Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Msg("Committing transaction")

	if err = tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("product_id", product.ID).
			Msg("Failed to commit transaction")
		return nil, utils.ErrTransactionCommit
	}

	logger.Debug().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Msg("Transaction committed successfully")

	// Préparation de la réponse
	response := &dto.ProductResponse{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
		Stock:       product.Stock,
		CreatedAt:   product.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   product.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	// Log de succès (sans métriques de perf)
	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Str("product_id", product.ID).
		Str("product_name", product.Name).
		Dur("duration_ms", duration).
		Msg("Product creation completed successfully")

	metrics.ProductsCreatedTotal.Inc()

	return response, nil
}
