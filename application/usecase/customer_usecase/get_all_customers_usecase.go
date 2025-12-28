// application/usecase/customer_usecase/get_all_customers_usecase.go
// application/usecase/customer_usecase/get_all_customers_usecase.go
package customerusecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type GetAllCustomersUsecase struct {
	repo      repository.CustomerRepositoryInterface
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewGetAllCustomersUsecase(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *GetAllCustomersUsecase {
	return &GetAllCustomersUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("get_all_customers"),
	}
}

func (uc *GetAllCustomersUsecase) Execute(ctx context.Context) ([]*entity.Customer, error) {
	start := time.Now()
	logger := zerolog.Ctx(ctx)
	// ✅ Pas de logger local — utilise uc.logger directement
	logger.Info().
		Str("operation", "execute").
		Msg("Starting retrieval of all customers")

	// Début de la transaction (lecture seule)
	logger.Debug().
		Str("operation", "execute").
		Msg("Beginning read-only transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback sécurisé
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logger.Error().
				Err(rollbackErr).
				Str("operation", "execute").
				Msg("Failed to rollback transaction")
		}
	}()

	// Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Msg("Repository attached to transaction")

	// Récupérer tous les clients
	logger.Debug().
		Str("operation", "execute").
		Msg("Fetching all customers from repository")

	customers, err := repo.FindAllCustomers(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Msg("Failed to retrieve customers from repository")
		return nil, fmt.Errorf("failed to retrieve customers: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Int("customers_count", len(customers)).
		Msg("Customers retrieved from repository")

	// Analyser les statistiques des clients (optionnel)
	if len(customers) > 0 {
		uc.analyzeCustomers(ctx, customers)
	} else {
		logger.Info().
			Str("operation", "execute").
			Msg("No customers found in the system")
	}

	// CORRECTION : Commit DOIT être appelé dans TOUS les cas
	logger.Debug().
		Str("operation", "execute").
		Msg("Committing read-only transaction")

	if err := tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug().
		Str("operation", "execute").
		Msg("Transaction committed successfully")

	// Log de succès (sans métriques de perf)
	duration := time.Since(start)
	logger.Info().
		Str("operation", "execute").
		Int("customers_returned", len(customers)).
		Dur("total_duration_ms", duration).
		Msg("All customers retrieved successfully")

	// CORRECTION : Retourner après le commit
	return customers, nil
}

// analyzeCustomers — utilise uc.logger, pas de paramètre logger
func (uc *GetAllCustomersUsecase) analyzeCustomers(ctx context.Context, customers []*entity.Customer) {
	logger := zerolog.Ctx(ctx)
	if len(customers) == 0 {
		return
	}

	var oldestCustomer, newestCustomer time.Time
	for i, customer := range customers {
		if i == 0 || customer.CreatedAt.Before(oldestCustomer) {
			oldestCustomer = customer.CreatedAt
		}
		if i == 0 || customer.CreatedAt.After(newestCustomer) {
			newestCustomer = customer.CreatedAt
		}
	}

	logger.Debug().
		Str("operation", "analyze_customers").
		Time("oldest_customer_created", oldestCustomer).
		Time("newest_customer_created", newestCustomer).
		Msg("Customer statistics analysis")
}
