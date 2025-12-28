// application/usecase/customer_usecase/update_customer_usecase.go
package customerusecase

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type UpdateCustomerUsecase struct {
	repo      repository.CustomerRepositoryInterface
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewUpdateCustomerUsecase(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *UpdateCustomerUsecase {
	return &UpdateCustomerUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("update_customer"),
	}
}

func (uc *UpdateCustomerUsecase) Execute(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	logger := zerolog.Ctx(ctx)
	if customer == nil {
		logger.Warn().
			Str("operation", "execute").
			Msg("Received nil customer")
		return nil, errors.New("customer cannot be nil")
	}

	if customer.ID == "" {
		logger.Warn().
			Str("operation", "execute").
			Msg("Customer ID is empty")
		return nil, errors.New("customer ID is required")
	}

	start := time.Now()

	logger.Info().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Msg("Starting customer update process")

	// Validation des données de mise à jour
	if err := uc.validateUpdateData(ctx, customer); err != nil {
		// Logging déjà fait dans validateUpdateData
		return nil, err
	}

	// Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Msg("Beginning database transaction for update")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", customer.ID).
			Msg("Failed to begin transaction")
		return nil, errors.New("failed to begin transaction")
	}

	// Gestion du rollback en cas d'erreur
	defer func() {
		if err != nil {
			logger.Warn().
				Str("operation", "execute").
				Str("customer_id", customer.ID).
				Msg("Rolling back transaction due to error")
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("operation", "execute").
					Str("customer_id", customer.ID).
					Msg("Failed to rollback transaction")
			}
		}
	}()

	// Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Msg("Repository attached to transaction")

	// Vérifier si le client existe
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Msg("Checking if customer exists")

	existingCustomer, err := repo.FindByCustomerID(ctx, customer.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn().
				Err(err).
				Dur("duration_before_error", time.Since(start)).
				Str("operation", "execute").
				Str("customer_id", customer.ID).
				Msg("Customer not found for update")
			return nil, errors.New("customer not found")
		}

		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Str("operation", "execute").
			Str("customer_id", customer.ID).
			Msg("Failed to retrieve existing customer")
		return nil, err
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Str("customer_email", existingCustomer.Email).
		Str("customer_name", existingCustomer.FirstName+" "+existingCustomer.LastName).
		Msg("Customer found, proceeding with update")

	// Normaliser les nouvelles données
	uc.normalizeCustomerData(ctx, customer)

	// Log des changements
	changes := uc.logChanges(ctx, existingCustomer, customer)

	// Appliquer les mises à jour
	updatedCustomer := uc.applyUpdates(ctx, existingCustomer, customer, changes)

	// Mise à jour du client
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", customer.ID).
		Msg("Updating customer in repository")

	updated, err := repo.UpdateCustomer(ctx, updatedCustomer)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", customer.ID).
			Int("changes_applied", len(changes)).
			Msg("Failed to update customer in repository")
		return nil, err
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", updated.ID).
		Msg("Customer updated successfully in repository")

	// Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", updated.ID).
		Msg("Committing update transaction")

	if err := tx.Commit(); err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_id", updated.ID).
			Msg("Failed to commit transaction")
		return nil, errors.New("failed to commit transaction")
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", updated.ID).
		Msg("Transaction committed successfully")

	// Log de succès (sans métriques de perf)
	duration := time.Since(start)
	logger.Info().
		Str("customer_id", updated.ID).
		Str("customer_email", updated.Email).
		Str("customer_name", updated.FirstName+" "+updated.LastName).
		Dur("total_duration_ms", duration).
		Int("changes_applied", len(changes)).
		Msg("Customer update completed successfully")

	return updated, nil
}

// validateUpdateData — utilise uc.logger directement
func (uc *UpdateCustomerUsecase) validateUpdateData(ctx context.Context, customer *entity.Customer) error {
	logger := zerolog.Ctx(ctx)
	if customer.FirstName != "" && len(strings.TrimSpace(customer.FirstName)) < 2 {
		logger.Warn().
			Str("operation", "validate").
			Str("field", "first_name").
			Str("value", customer.FirstName).
			Int("length", len(customer.FirstName)).
			Msg("Update validation failed: first name too short")
		return errors.New("first name must be at least 2 characters")
	}

	if customer.LastName != "" && len(strings.TrimSpace(customer.LastName)) < 2 {
		logger.Warn().
			Str("operation", "validate").
			Str("field", "last_name").
			Str("value", customer.LastName).
			Int("length", len(customer.LastName)).
			Msg("Update validation failed: last name too short")
		return errors.New("last name must be at least 2 characters")
	}

	if customer.Email != "" {
		email := strings.TrimSpace(customer.Email)
		if !isValidEmails(email) {
			logger.Warn().
				Str("operation", "validate").
				Str("field", "email").
				Str("value", maskEmail(email)).
				Msg("Update validation failed: invalid email format")
			return errors.New("email must be a valid email address")
		}
	}

	logger.Debug().
		Str("operation", "validate").
		Str("customer_id", customer.ID).
		Msg("Update data validation passed")
	return nil
}

// normalizeCustomerData — utilise uc.logger directement
func (uc *UpdateCustomerUsecase) normalizeCustomerData(ctx context.Context, customer *entity.Customer) {
	logger := zerolog.Ctx(ctx)
	normalized := false

	if customer.FirstName != "" {
		old := customer.FirstName
		customer.FirstName = strings.TrimSpace(customer.FirstName)
		if old != customer.FirstName {
			normalized = true
		}
	}

	if customer.LastName != "" {
		old := customer.LastName
		customer.LastName = strings.TrimSpace(customer.LastName)
		if old != customer.LastName {
			normalized = true
		}
	}

	if customer.Email != "" {
		old := customer.Email
		customer.Email = strings.ToLower(strings.TrimSpace(customer.Email))
		if old != customer.Email {
			normalized = true
		}
	}

	if normalized {
		logger.Debug().
			Str("operation", "normalize").
			Str("customer_id", customer.ID).
			Msg("Customer data normalized before update")
	}
}

// logChanges — utilise uc.logger directement
func (uc *UpdateCustomerUsecase) logChanges(ctx context.Context, existing, new *entity.Customer) []string {
	logger := zerolog.Ctx(ctx)
	var changes []string

	if new.FirstName != "" && new.FirstName != existing.FirstName {
		changes = append(changes, "first_name")
		logger.Info().
			Str("operation", "log_changes").
			Str("customer_id", existing.ID).
			Str("field", "first_name").
			Str("old_value", existing.FirstName).
			Str("new_value", new.FirstName).
			Msg("First name will be updated")
	}

	if new.LastName != "" && new.LastName != existing.LastName {
		changes = append(changes, "last_name")
		logger.Info().
			Str("operation", "log_changes").
			Str("customer_id", existing.ID).
			Str("field", "last_name").
			Str("old_value", existing.LastName).
			Str("new_value", new.LastName).
			Msg("Last name will be updated")
	}

	if new.Email != "" && new.Email != existing.Email {
		changes = append(changes, "email")
		logger.Info().
			Str("operation", "log_changes").
			Str("customer_id", existing.ID).
			Str("field", "email").
			Str("old_value", maskEmail(existing.Email)).
			Str("new_value", maskEmail(new.Email)).
			Msg("Email will be updated")
	}

	if len(changes) > 0 {
		logger.Info().
			Str("operation", "log_changes").
			Str("customer_id", existing.ID).
			Strs("fields_to_update", changes).
			Int("total_changes", len(changes)).
			Msg("Customer changes detected")
	} else {
		logger.Warn().
			Str("operation", "log_changes").
			Str("customer_id", existing.ID).
			Msg("No changes detected for customer update")
	}

	return changes
}

// applyUpdates — utilise uc.logger directement
func (uc *UpdateCustomerUsecase) applyUpdates(ctx context.Context, existing, new *entity.Customer, changes []string) *entity.Customer {
	logger := zerolog.Ctx(ctx)
	if new.FirstName != "" && new.FirstName != existing.FirstName {
		existing.FirstName = new.FirstName
	}
	if new.LastName != "" && new.LastName != existing.LastName {
		existing.LastName = new.LastName
	}
	if new.Email != "" && new.Email != existing.Email {
		existing.Email = new.Email
	}

	logger.Debug().
		Str("operation", "apply_updates").
		Str("customer_id", existing.ID).
		Int("changes_applied", len(changes)).
		Msg("Changes applied to customer entity")

	return existing
}

// Helpers (inchangés)
func isValidEmails(email string) bool {
	if email == "" {
		return false
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return false
	}
	parts := strings.Split(email, "@")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	username := parts[0]
	if len(username) <= 2 {
		username = "***"
	} else {
		username = username[:2] + "***"
	}
	return username + "@" + parts[1]
}
