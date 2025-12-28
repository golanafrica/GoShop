// application/usecase/customer_usecase/get_customer_by_id_usecase.go
// application/usecase/customer_usecase/get_customer_by_id_usecase.go
package customerusecase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type GetCustomerByIdUsecase struct {
	repo      repository.CustomerRepositoryInterface
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewGetCustomerByIdUsecase(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *GetCustomerByIdUsecase {
	return &GetCustomerByIdUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("get_customer_by_id"),
	}
}

func (uc *GetCustomerByIdUsecase) Execute(ctx context.Context, id string) (*entity.Customer, error) {
	logger := zerolog.Ctx(ctx)
	if id == "" {
		logger.Warn().
			Str("operation", "execute").
			Msg("Customer ID is empty")
		return nil, errors.New("customer ID is required")
	}

	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Starting customer retrieval by ID")

	// ✅ AJOUT : utilise une transaction (lecture cohérente)
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Beginning read-only transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", id).
			Msg("Failed to begin transaction")
		return nil, err
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error().
				Err(rollbackErr).
				Str("operation", "execute").
				Str("customer_id", id).
				Msg("Failed to rollback transaction")
		}
	}()

	// ✅ Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", id).
		Msg("Repository attached to transaction")

	// ✅ Lecture via le repository transactionnel
	customer, err := repo.FindByCustomerID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn().
				Err(err).
				Dur("duration_before_error", time.Since(start)).
				Str("operation", "execute").
				Str("customer_id", id).
				Msg("Customer not found in repository")
			return nil, errors.New("customer not found")
		}

		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Str("customer_id", id).
			Msg("Failed to retrieve customer from repository")
		return nil, err
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Str("customer_email", customer.Email).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Msg("Customer retrieved from repository")

	uc.analyzeCustomer(ctx, customer)

	// ✅ Commit (optionnel en lecture seule, mais cohérent)
	if err := tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", customer.ID).
			Msg("Failed to commit transaction")
		return nil, err
	}

	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Str("customer_email", customer.Email).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Dur("total_duration_ms", duration).
		Msg("Customer retrieved successfully")

	return customer, nil
}

// analyzeCustomer — inchangé
func (uc *GetCustomerByIdUsecase) analyzeCustomer(ctx context.Context, customer *entity.Customer) {
	logger := zerolog.Ctx(ctx)
	analysis := map[string]interface{}{
		"customer_id": customer.ID,
		"email":       customer.Email,
		"first_name":  customer.FirstName,
		"last_name":   customer.LastName,
		"full_name":   customer.FirstName + " " + customer.LastName,
	}

	logger.Debug().
		Str("operation", "analyze_customer").
		Str("customer_id", customer.ID).
		Interface("customer_analysis", analysis).
		Msg("Customer details analysis")
}
