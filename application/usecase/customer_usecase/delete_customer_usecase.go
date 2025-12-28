// application/usecase/customer_usecase/delete_customer_usecase.go
package customerusecase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type DeleteCustomerUsecase struct {
	repo      repository.CustomerRepositoryInterface
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewDeleteCustomerUsecase(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *DeleteCustomerUsecase {
	return &DeleteCustomerUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("delete_customer"),
	}
}

func (uc *DeleteCustomerUsecase) Execute(ctx context.Context, id string) error {
	logger := zerolog.Ctx(ctx)
	if id == "" {

		logger.Warn().
			Str("operation", "execute").
			Msg("Customer ID is empty")
		return errors.New("customer ID is required")
	}

	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Starting customer deletion process")

	// Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Beginning database transaction for deletion")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", id).
			Msg("Failed to begin transaction")
		return errors.New("failed to begin transaction")
	}

	// ✅ CORRECTION CLÉ : utilise une variable d'erreur dédiée pour le defer
	var rollbackErr error
	defer func() {
		if rollbackErr != nil {
			logger.Warn().
				Str("operation", "execute").
				Str("customer_id", id).
				Msg("Rolling back transaction due to error")
			if rErr := tx.Rollback(); rErr != nil && rErr != sql.ErrTxDone {
				logger.Error().
					Err(rErr).
					Str("operation", "execute").
					Str("customer_id", id).
					Msg("Failed to rollback transaction")
			}
		}
	}()

	// Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Repository attached to transaction")

	// Vérifier si le client existe
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Checking if customer exists before deletion")

	customer, findErr := repo.FindByCustomerID(ctx, id)
	if findErr != nil {
		if findErr == sql.ErrNoRows {
			logger.Warn().
				Err(findErr).
				Dur("duration_before_error", time.Since(start)).
				Str("operation", "execute").
				Str("customer_id", id).
				Msg("Customer not found for deletion")
			rollbackErr = errors.New("customer not found") // ✅ Affecte rollbackErr
			return rollbackErr
		}

		logger.Error().
			Err(findErr).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Str("customer_id", id).
			Msg("Failed to check customer existence")
		rollbackErr = findErr // ✅ Affecte rollbackErr
		return rollbackErr
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Str("customer_email", customer.Email).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Msg("Customer found, proceeding with deletion")

	// Suppression du client
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Deleting customer from repository")

	deleteErr := repo.DeleteCustomer(ctx, id)
	if deleteErr != nil {
		if deleteErr == sql.ErrNoRows {
			logger.Warn().
				Err(deleteErr).
				Dur("duration_before_error", time.Since(start)).
				Str("operation", "execute").
				Str("customer_id", id).
				Msg("Customer not found during deletion")
			rollbackErr = errors.New("customer not found") // ✅ Affecte rollbackErr
			return rollbackErr
		}

		logger.Error().
			Err(deleteErr).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Str("customer_id", id).
			Str("customer_exists_before", func() string {
				if customer != nil {
					return "true"
				}
				return "false"
			}()).
			Msg("Failed to delete customer from repository")
		rollbackErr = deleteErr // ✅ Affecte rollbackErr
		return rollbackErr
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Customer deleted successfully from repository")

	// Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Committing deletion transaction")

	if commitErr := tx.Commit(); commitErr != nil {
		logger.Error().
			Err(commitErr).
			Stack().
			Str("operation", "execute").
			Str("customer_id", id).
			Msg("Failed to commit transaction")
		rollbackErr = commitErr // Techniquement, on ne rollback pas après commit échoué, mais cohérent
		return rollbackErr
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Transaction committed successfully")

	// Log de succès
	duration := time.Since(start)
	logger.Info().
		Str("customer_id", id).
		Str("customer_email", func() string {
			if customer != nil {
				return customer.Email
			}
			return "unknown"
		}()).
		Str("customer_name", func() string {
			if customer != nil {
				return customer.FirstName + " " + customer.LastName
			}
			return "unknown"
		}()).
		Dur("total_duration_ms", duration).
		Msg("Customer deletion completed successfully")

	return nil
}
