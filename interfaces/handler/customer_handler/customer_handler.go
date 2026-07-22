// interfaces/handler/customer_handler/customer_handler.go
package customerhandler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dto "Goshop/application/dto/customer_dto"
	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/domain/repository"
	userrepository "Goshop/domain/repository/user_repository" // ✅ AJOUTÉ
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type CustomerHandler struct {
	createCustomerUsecase  *customerusecase.CreateCustomerUsecase
	getAllCustomersUsecase *customerusecase.GetAllCustomersUsecase
	getCustomerByIdUsecase *customerusecase.GetCustomerByIdUsecase
	updateCustomerUsecase  *customerusecase.UpdateCustomerUsecase
	deleteCustomerUsecase  *customerusecase.DeleteCustomerUsecase
}

func NewCustomerHandler(
	repo repository.CustomerRepositoryInterface,
	userRepo userrepository.UserRepository, // ✅ AJOUTÉ
	txManager repository.TxManager,
) *CustomerHandler {
	return &CustomerHandler{
		createCustomerUsecase:  customerusecase.NewCreateCustomerUsecase(repo, userRepo, txManager), // ✅ MODIFIÉ
		getAllCustomersUsecase: customerusecase.NewGetAllCustomersUsecase(repo, txManager),
		getCustomerByIdUsecase: customerusecase.NewGetCustomerByIdUsecase(repo, txManager),
		updateCustomerUsecase:  customerusecase.NewUpdateCustomerUsecase(repo, txManager),
		deleteCustomerUsecase:  customerusecase.NewDeleteCustomerUsecase(repo, txManager),
	}
}

// @Summary Créer un nouveau client
// @Description Crée un nouveau profil client pour la boutique active. Si l'email existe déjà, le client est automatiquement lié à l'utilisateur existant.
// @Tags Customers
// @Accept json
// @Produce json
// @Param request body dto.CustomerRequestDto true "Détails du client à créer"
// @Success 201 {object} dto.CustomerResponseDto
// @Failure 400 {object} utils.AppError "Payload invalide ou validation échouée"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers [post]
func (h *CustomerHandler) CreateCustomerHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Starting customer creation")

	bodyBytes, _ := io.ReadAll(r.Body)
	// 🛡️ SÉCURITÉ : Ne jamais logger le raw_body brut (pourrait contenir des données sensibles)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req dto.CustomerRequestDto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("content_type", r.Header.Get("Content-Type")).
			Int64("content_length", r.ContentLength).
			Msg("Failed to decode JSON payload")
		return utils.ErrInvalidPayload
	}

	logger.Debug().
		Str("customer_email", req.Email).
		Str("customer_first_name", req.FirstName).
		Str("customer_last_name", req.LastName).
		Msg("Customer request decoded")

	logger.Debug().Msg("Starting validation")
	if err := h.validateCustomerRequest(&req, logger); err != nil {
		logger.Error().Err(err).Msg("Validation failed")
		return utils.ErrValidationFailed
	}
	logger.Debug().Msg("Validation passed")

	customer := dto.ToCustomerEntity(&req)
	logger.Debug().
		Str("customer_email", customer.Email).
		Str("customer_id", customer.ID).
		Msg("Customer entity created")

	logger.Info().
		Str("usecase_type", fmt.Sprintf("%T", h.createCustomerUsecase)).
		Msg("Executing create customer usecase")

	created, err := h.createCustomerUsecase.Execute(ctx, customer)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("error_message", err.Error()).
			Interface("customer_details", map[string]interface{}{
				"email":      customer.Email,
				"first_name": customer.FirstName,
				"last_name":  customer.LastName,
				"id":         customer.ID,
			}).
			Msg("Failed to create customer")
		return utils.ErrCustomerCreateFail
	}

	logger.Info().
		Str("customer_id", created.ID).
		Str("customer_email", created.Email).
		Msg("Customer created successfully")

	response := dto.ToCustomerResponse(created)

	logger.Info().
		Str("customer_id", response.ID).
		Dur("duration", time.Since(start)).
		Int("http_status", http.StatusCreated).
		Msg("Customer creation completed")

	utils.WriteJSON(w, http.StatusCreated, response)
	return nil
}

// @Summary Récupérer un client par son ID
// @Description Retourne les détails d'un client spécifique appartenant à la boutique active.
// @Tags Customers
// @Accept json
// @Produce json
// @Param id path string true "ID du client (UUID)"
// @Success 200 {object} dto.CustomerResponseDto
// @Failure 400 {object} utils.AppError "ID client manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 404 {object} utils.AppError "Client introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers/{id} [get]
func (h *CustomerHandler) GetCustomerByIdHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	id := chi.URLParam(r, "id")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("customer_id", id).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Retrieving customer by ID")

	if id == "" {
		logger.Warn().Msg("Empty customer ID provided")
		return utils.ErrInvalidPayload
	}

	logger.Debug().Msg("Executing get customer by ID usecase")
	customer, err := h.getCustomerByIdUsecase.Execute(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			logger.Warn().
				Err(err).
				Dur("duration_before_error", time.Since(start)).
				Msg("Customer not found")
			return utils.ErrCustomerNotFound
		}

		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Msg("Failed to retrieve customer")
		return utils.ErrInternalServer
	}

	logger.Debug().
		Str("customer_email", customer.Email).
		Str("customer_name", customer.FirstName+" "+customer.LastName).
		Msg("Customer retrieved")

	response := dto.ToCustomerResponse(customer)

	logger.Info().
		Str("customer_id", response.ID).
		Dur("duration", time.Since(start)).
		Int("http_status", http.StatusOK).
		Msg("Customer retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Lister tous les clients de la boutique
// @Description Retourne la liste de tous les clients enregistrés pour la boutique active.
// @Tags Customers
// @Accept json
// @Produce json
// @Success 200 {array} dto.CustomerResponseDto
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers [get]
func (h *CustomerHandler) GetAllCustomersHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("query_params", r.URL.RawQuery).
		Msg("Retrieving all customers")

	logger.Debug().Msg("Executing get all customers usecase")
	customers, err := h.getAllCustomersUsecase.Execute(ctx)
	if err != nil {
		logger.Error().
			Err(err).
			Stack().
			Msg("Failed to retrieve customers")
		return utils.ErrInternalServer
	}

	logger.Debug().Int("customers_count", len(customers)).Msg("Customers retrieved")

	if len(customers) == 0 {
		logger.Info().Msg("No customers found")
		utils.WriteJSON(w, http.StatusOK, []interface{}{})
		return nil
	}

	responses := dto.ToCustomerResponses(customers)

	logger.Info().
		Int("customers_returned", len(responses)).
		Dur("duration", time.Since(start)).
		Int("http_status", http.StatusOK).
		Msg("All customers retrieved successfully")

	utils.WriteJSON(w, http.StatusOK, responses)
	return nil
}

// @Summary Mettre à jour un client
// @Description Met à jour les informations d'un client existant dans la boutique active.
// @Tags Customers
// @Accept json
// @Produce json
// @Param id path string true "ID du client (UUID)"
// @Param request body dto.CustomerRequestDto true "Nouvelles données du client"
// @Success 200 {object} dto.CustomerResponseDto
// @Failure 400 {object} utils.AppError "Payload invalide ou ID manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 404 {object} utils.AppError "Client introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers/{id} [put]
func (h *CustomerHandler) UpdateCustomerHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	id := chi.URLParam(r, "id")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("customer_id", id).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Starting customer update")

	if id == "" {
		logger.Warn().Msg("Empty customer ID provided")
		return utils.ErrInvalidPayload
	}

	var req dto.CustomerRequestDto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().
			Err(err).
			Str("content_type", r.Header.Get("Content-Type")).
			Int64("content_length", r.ContentLength).
			Msg("Failed to decode JSON payload for update")
		return utils.ErrInvalidPayload
	}

	logger.Debug().
		Str("customer_email", req.Email).
		Str("customer_first_name", req.FirstName).
		Str("customer_last_name", req.LastName).
		Msg("Update request decoded")

	if err := h.validateCustomerRequest(&req, logger); err != nil {
		return utils.ErrValidationFailed
	}

	customer := dto.ToCustomerEntity(&req)
	customer.ID = id
	logger.Debug().Str("customer_email", customer.Email).Msg("Customer entity prepared for update")

	logger.Info().Msg("Executing update customer usecase")
	updated, err := h.updateCustomerUsecase.Execute(ctx, customer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			logger.Warn().Err(err).Msg("Customer not found for update")
			return utils.ErrCustomerNotFound
		}

		logger.Error().
			Err(err).
			Stack().
			Interface("update_data", map[string]interface{}{
				"id":         customer.ID,
				"email":      customer.Email,
				"first_name": customer.FirstName,
				"last_name":  customer.LastName,
			}).
			Msg("Failed to update customer")
		return utils.ErrCustomerUpdateFail
	}

	logger.Info().
		Str("customer_id", updated.ID).
		Str("customer_email", updated.Email).
		Msg("Customer updated successfully")

	response := dto.ToCustomerResponse(updated)

	logger.Info().
		Str("customer_id", response.ID).
		Dur("duration", time.Since(start)).
		Int("http_status", http.StatusOK).
		Msg("Customer update completed")

	utils.WriteJSON(w, http.StatusOK, response)
	return nil
}

// @Summary Supprimer un client
// @Description Supprime un client de la boutique active.
// @Tags Customers
// @Accept json
// @Produce json
// @Param id path string true "ID du client (UUID)"
// @Success 204 "Client supprimé avec succès"
// @Failure 400 {object} utils.AppError "ID client manquant"
// @Failure 401 {object} utils.AppError "Contexte multi-tenant requis"
// @Failure 404 {object} utils.AppError "Client introuvable"
// @Failure 500 {object} utils.AppError "Erreur interne du serveur"
// @Security ApiKeyAuth
// @Router /api/customers/{id} [delete]
func (h *CustomerHandler) DeleteCustomerHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	start := time.Now()
	id := chi.URLParam(r, "id")
	logger := zerolog.Ctx(ctx)

	logger.Info().
		Str("customer_id", id).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Starting customer deletion")

	if id == "" {
		logger.Warn().Msg("Empty customer ID provided")
		return utils.ErrInvalidPayload
	}

	logger.Debug().Msg("Executing delete customer usecase")
	err := h.deleteCustomerUsecase.Execute(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			logger.Warn().
				Err(err).
				Dur("duration_before_error", time.Since(start)).
				Msg("Customer not found for deletion")
			return utils.ErrCustomerNotFound
		}

		logger.Error().
			Err(err).
			Stack().
			Dur("duration_before_error", time.Since(start)).
			Msg("Failed to delete customer")
		return utils.ErrCustomerDeleteFail
	}

	logger.Info().
		Str("customer_id", id).
		Dur("duration", time.Since(start)).
		Int("http_status", http.StatusNoContent).
		Msg("Customer deletion completed")

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *CustomerHandler) validateCustomerRequest(req *dto.CustomerRequestDto, logger *zerolog.Logger) error {
	var validationErrors []string

	if req.FirstName == "" {
		validationErrors = append(validationErrors, "first_name is required")
		logger.Debug().Str("field", "first_name").Msg("Validation failed: first_name is required")
	} else if len(req.FirstName) < 2 {
		validationErrors = append(validationErrors, "first_name must be at least 2 characters")
		logger.Debug().
			Str("field", "first_name").
			Str("value", req.FirstName).
			Int("length", len(req.FirstName)).
			Msg("Validation failed: first_name too short")
	}

	if req.LastName == "" {
		validationErrors = append(validationErrors, "last_name is required")
		logger.Debug().Str("field", "last_name").Msg("Validation failed: last_name is required")
	} else if len(req.LastName) < 2 {
		validationErrors = append(validationErrors, "last_name must be at least 2 characters")
		logger.Debug().
			Str("field", "last_name").
			Str("value", req.LastName).
			Int("length", len(req.LastName)).
			Msg("Validation failed: last_name too short")
	}

	if req.Email == "" {
		validationErrors = append(validationErrors, "email is required")
		logger.Debug().Str("field", "email").Msg("Validation failed: email is required")
	} else if !isValidEmail(req.Email) {
		validationErrors = append(validationErrors, "email must be a valid email address")
		logger.Debug().
			Str("field", "email").
			Str("value", req.Email).
			Msg("Validation failed: invalid email format")
	}

	if len(validationErrors) > 0 {
		logger.Warn().
			Int("error_count", len(validationErrors)).
			Strs("validation_errors", validationErrors).
			Msg("Customer request validation failed")
		return errors.New(validationErrors[0])
	}

	logger.Debug().Msg("Customer request validation passed")
	return nil
}

func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
