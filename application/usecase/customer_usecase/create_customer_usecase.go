// application/usecase/customer_usecase/create_customer_usecase.go
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

type CreateCustomerUsecase struct {
	repo      repository.CustomerRepositoryInterface
	txManager repository.TxManager
	//logger    *setupLogging.Logger
}

func NewCreateCustomerUsecase(
	repo repository.CustomerRepositoryInterface,
	txManager repository.TxManager,
	//logger *setupLogging.Logger,
) *CreateCustomerUsecase {
	return &CreateCustomerUsecase{
		repo:      repo,
		txManager: txManager,
		//logger:    logger.WithComponent("create_customer"),
	}
}

func (uc *CreateCustomerUsecase) Execute(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
	if customer == nil {
		zerolog.Ctx(ctx).Warn().Msg("Received nil customer")
		return nil, errors.New("customer cannot be nil")
	}

	start := time.Now()

	logger := zerolog.Ctx(ctx).With().
		Str("operation", "execute").
		Str("component", "create_customer").
		Logger()

	logger.Info().
		Str("customer_email", customer.Email).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Msg("Starting customer creation process")

		// 1. Validation avant transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_email", customer.Email).
		Msg("Validating customer data before transaction")

	if err := uc.validateCustomerData(ctx, customer); err != nil {
		return nil, err
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_email", customer.Email).
		Msg("All customer validations passed")

	// 2. Début de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_email", customer.Email).
		Msg("Beginning database transaction")

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("operation", "execute").
			Str("customer_email", customer.Email).
			Msg("Failed to begin transaction")
		return nil, errors.New("failed to begin transaction")
	}

	// Définir err ici pour le defer
	err = nil
	defer func() {
		if err != nil {
			logger.Warn().
				Str("operation", "execute").
				Str("customer_email", customer.Email).
				Msg("Rolling back transaction due to error")
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				logger.Error().
					Err(rollbackErr).
					Str("operation", "execute").
					Str("customer_email", customer.Email).
					Msg("Failed to rollback transaction")
			}
		}
	}()

	// 3. Attacher le repository à la transaction
	repo := uc.repo.WithTX(tx)
	logger.Debug().
		Str("operation", "execute").
		Str("customer_email", customer.Email).
		Msg("Repository attached to transaction")

		// 4. Vérifier si l'email existe déjà - TEMPORAIREMENT COMMENTÉ POUR LES TESTS
		// 4. Vérifier si l'email existe déjà - AVEC LA NOUVELLE MÉTHODE
	logger.Debug().
		Str("operation", "execute").
		Str("email", customer.Email).
		Msg("Checking if email already exists using FindByEmail")

	existingCustomer, findErr := repo.FindByEmail(ctx, customer.Email)
	if findErr == nil && existingCustomer != nil {
		logger.Warn().
			Str("operation", "execute").
			Str("email", customer.Email).
			Str("existing_customer_id", existingCustomer.ID).
			Msg("Customer with this email already exists")
		return nil, errors.New("customer with this email already exists")
	}

	if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
		logger.Error().
			Err(findErr).
			Stack().
			Str("operation", "execute").
			Str("email", customer.Email).
			Msg("Failed to check email existence")
		// On continue malgré l'erreur (décision de design)
	} else if errors.Is(findErr, sql.ErrNoRows) {
		logger.Debug().
			Str("operation", "execute").
			Str("email", customer.Email).
			Msg("Email is unique, can proceed with creation")
	}

	// 5. Normaliser les données
	uc.normalizeCustomerData(ctx, customer)

	// 6. Création du client
	logger.Debug().
		Str("operation", "execute").
		Str("customer_email", customer.Email).
		Msg("Creating customer in repository")

	createdCustomer, createErr := repo.Create(ctx, customer)
	if createErr != nil {
		logger.Error().
			Err(createErr).
			Stack().
			Str("operation", "execute").
			Str("customer_email", customer.Email).
			Msg("Failed to create customer in repository")
		err = createErr
		return nil, createErr
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", createdCustomer.ID).
		Str("customer_email", createdCustomer.Email).
		Msg("Customer created successfully in repository")

	// 7. Commit de la transaction
	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", createdCustomer.ID).
		Msg("Committing transaction")

	if commitErr := tx.Commit(); commitErr != nil {
		logger.Error().
			Err(commitErr).
			Stack().
			Str("operation", "execute").
			Str("customer_id", createdCustomer.ID).
			Msg("Failed to commit transaction")
		err = commitErr
		return nil, errors.New("failed to commit transaction")
	}

	logger.Debug().
		Str("operation", "execute").
		Str("customer_id", createdCustomer.ID).
		Msg("Transaction committed successfully")

	// 8. Log de succès
	duration := time.Since(start)
	logger.Info().
		Str("customer_id", createdCustomer.ID).
		Str("customer_email", createdCustomer.Email).
		Str("customer_name", createdCustomer.FirstName+" "+createdCustomer.LastName).
		Dur("total_duration_ms", duration).
		Msg("Customer creation completed successfully")

	return createdCustomer, nil
}

// validateCustomerData — validation des données
func (uc *CreateCustomerUsecase) validateCustomerData(ctx context.Context, customer *entity.Customer) error {
	if customer.FirstName == "" {
		zerolog.Ctx(ctx).Warn().
			Str("operation", "validate").
			Str("field", "first_name").
			Msg("Customer validation failed: first name is required")
		return errors.New("customer first name is required")
	}

	if len(strings.TrimSpace(customer.FirstName)) < 2 {
		zerolog.Ctx(ctx).Warn().
			Str("operation", "validate").
			Str("field", "first_name").
			Str("value", customer.FirstName).
			Int("length", len(customer.FirstName)).
			Msg("Customer validation failed: first name must be at least 2 characters")
		return errors.New("customer first name must be at least 2 characters")
	}

	if customer.LastName == "" {
		zerolog.Ctx(ctx).Warn().
			Str("operation", "validate").
			Str("field", "last_name").
			Msg("Customer validation failed: last name is required")
		return errors.New("customer last name is required")
	}

	if len(strings.TrimSpace(customer.LastName)) < 2 {
		zerolog.Ctx(ctx).Warn().
			Str("operation", "validate").
			Str("field", "last_name").
			Str("value", customer.LastName).
			Int("length", len(customer.LastName)).
			Msg("Customer validation failed: last name must be at least 2 characters")
		return errors.New("customer last name must be at least 2 characters")
	}

	if customer.Email == "" {
		zerolog.Ctx(ctx).Warn().
			Str("operation", "validate").
			Str("field", "email").
			Msg("Customer validation failed: email is required")
		return errors.New("customer email is required")
	}

	if !isValidEmail(customer.Email) {
		zerolog.Ctx(ctx).Warn().
			Str("operation", "validate").
			Str("field", "email").
			Str("value", customer.Email).
			Msg("Customer validation failed: invalid email format")
		return errors.New("customer email must be a valid email address")
	}

	return nil
}

// normalizeCustomerData — normalisation des données
func (uc *CreateCustomerUsecase) normalizeCustomerData(ctx context.Context, customer *entity.Customer) {
	oldFirstName := customer.FirstName
	oldLastName := customer.LastName
	oldEmail := customer.Email

	customer.FirstName = strings.TrimSpace(customer.FirstName)
	customer.LastName = strings.TrimSpace(customer.LastName)
	customer.Email = strings.ToLower(strings.TrimSpace(customer.Email))

	if oldFirstName != customer.FirstName || oldLastName != customer.LastName || oldEmail != customer.Email {
		zerolog.Ctx(ctx).Debug().
			Str("operation", "normalize").
			Interface("normalization_changes", map[string]interface{}{
				"first_name": map[string]string{"before": oldFirstName, "after": customer.FirstName},
				"last_name":  map[string]string{"before": oldLastName, "after": customer.LastName},
				"email":      map[string]string{"before": oldEmail, "after": customer.Email},
			}).
			Msg("Customer data normalized before saving")
	}
}

// isValidEmail — validation d'email simplifiée
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	domainParts := strings.Split(parts[1], ".")
	return len(domainParts) >= 2
}
