// interfaces/handler/customer_handler/customer_handler_test.go
package customerhandler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"Goshop/domain/entity"
	customerhandler "Goshop/interfaces/handler/customer_handler"
	"Goshop/interfaces/middl"
	mockrepo "Goshop/mocks/repository"
)

// Helper pour injecter le paramètre ID de façon fiable
func setupChiContext(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// Helper pour créer un customer de test
func createTestCustomer(id, email string) *entity.Customer {
	return &entity.Customer{
		ID:        id,
		FirstName: "John",
		LastName:  "Doe",
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestCreateCustomerHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Mock expectations - CORRECTION ICI
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)

	// AVANT : FindByCustomerID
	// APRÈS : FindByEmail
	mockRepoWithTx.EXPECT().FindByEmail(gomock.Any(), "john@example.com").Return(nil, sql.ErrNoRows)

	mockRepoWithTx.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, customer *entity.Customer) (*entity.Customer, error) {
		customer.ID = "cust-123"
		return customer, nil
	})
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	body := map[string]interface{}{
		"first_name": "John",
		"last_name":  "Doe",
		"email":      "john@example.com",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "cust-123", response["id"])
	assert.Equal(t, "John", response["first_name"])
	assert.Equal(t, "Doe", response["last_name"])
	assert.Equal(t, "john@example.com", response["email"])
}

func TestCreateCustomerHandler_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// JSON invalide
	invalidJSON := `{"first_name": "John", "email": "invalid-email"`

	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_PAYLOAD", response["code"])
}

func TestCreateCustomerHandler_ValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Email invalide
	body := map[string]interface{}{
		"first_name": "John",
		"last_name":  "Doe",
		"email":      "invalid-email",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/customers", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "VALIDATION_FAILED", response["code"])
}

func TestGetCustomerByIdHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	customer := createTestCustomer("cust-123", "john@example.com")

	// Mock expectations - ORDRE IMPORTANT
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-123").Return(customer, nil)
	mockTx.EXPECT().Commit().Return(nil)              // Commit AVANT Rollback
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // Rollback après

	req := httptest.NewRequest(http.MethodGet, "/customers/cust-123", nil)
	req = setupChiContext(req, "cust-123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetCustomerByIdHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "cust-123", response["id"])
	assert.Equal(t, "John", response["first_name"])
	assert.Equal(t, "Doe", response["last_name"])
	assert.Equal(t, "john@example.com", response["email"])
}

func TestGetCustomerByIdHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Mock expectations
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().
		FindByCustomerID(gomock.Any(), "cust-999").
		Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil) // Pas de Commit, seulement Rollback

	req := httptest.NewRequest(http.MethodGet, "/customers/cust-999", nil)
	req = setupChiContext(req, "cust-999")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetCustomerByIdHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "CUSTOMER_NOT_FOUND", response["code"])
}

func TestGetCustomerByIdHandler_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// ID vide
	req := httptest.NewRequest(http.MethodGet, "/customers/", nil)
	req = setupChiContext(req, "")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetCustomerByIdHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_PAYLOAD", response["code"])
}

func TestGetAllCustomersHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	customers := []*entity.Customer{
		createTestCustomer("cust-1", "john1@example.com"),
		createTestCustomer("cust-2", "john2@example.com"),
	}

	// Mock expectations - ORDRE CORRECT
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindAllCustomers(gomock.Any()).Return(customers, nil)
	mockTx.EXPECT().Commit().Return(nil)              // S'assurer que Commit est appelé
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // Après Commit

	req := httptest.NewRequest(http.MethodGet, "/customers", nil)
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllCustomersHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, "cust-1", response[0]["id"])
	assert.Equal(t, "cust-2", response[1]["id"])
}

func TestGetAllCustomersHandler_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Mock expectations - L'ORDRE EST CRITIQUE
	// 1. BeginTx doit être appelé en premier
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)

	// 2. WithTX doit être appelé ensuite
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)

	// 3. FindAllCustomers doit être appelé
	mockRepoWithTx.EXPECT().FindAllCustomers(gomock.Any()).Return([]*entity.Customer{}, nil)

	// 4. Commit doit être appelé APRÈS FindAllCustomers
	mockTx.EXPECT().Commit().Return(nil)

	// 5. Rollback peut être appelé à tout moment (AnyTimes)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	req := httptest.NewRequest(http.MethodGet, "/customers", nil)
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllCustomersHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Empty(t, response)
}

func TestUpdateCustomerHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	existingCustomer := createTestCustomer("cust-123", "old@example.com")
	updatedCustomer := createTestCustomer("cust-123", "new@example.com")
	updatedCustomer.FirstName = "Jane"
	updatedCustomer.LastName = "Smith"

	// Mock expectations - ORDRE IMPORTANT
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-123").Return(existingCustomer, nil)
	mockRepoWithTx.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(updatedCustomer, nil)
	mockTx.EXPECT().Commit().Return(nil)              // Commit AVANT Rollback
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // Rollback après

	body := map[string]interface{}{
		"first_name": "Jane",
		"last_name":  "Smith",
		"email":      "new@example.com",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/customers/cust-123", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupChiContext(req, "cust-123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "cust-123", response["id"])
	assert.Equal(t, "Jane", response["first_name"])
	assert.Equal(t, "Smith", response["last_name"])
	assert.Equal(t, "new@example.com", response["email"])
}

func TestUpdateCustomerHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Mock expectations
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().
		FindByCustomerID(gomock.Any(), "cust-999").
		Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil) // Pas de Commit, seulement Rollback

	body := map[string]interface{}{
		"first_name": "NewName",
		"last_name":  "NewLast",
		"email":      "new@example.com",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/customers/cust-999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupChiContext(req, "cust-999")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "CUSTOMER_NOT_FOUND", response["code"])
}

func TestUpdateCustomerHandler_ValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Email invalide
	body := map[string]interface{}{
		"first_name": "Jane",
		"last_name":  "Smith",
		"email":      "invalid-email",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/customers/cust-123", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupChiContext(req, "cust-123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "VALIDATION_FAILED", response["code"])
}

func TestDeleteCustomerHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	customer := createTestCustomer("cust-123", "john@example.com")

	// Mock expectations - ORDRE IMPORTANT
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-123").Return(customer, nil)
	mockRepoWithTx.EXPECT().DeleteCustomer(gomock.Any(), "cust-123").Return(nil)
	mockTx.EXPECT().Commit().Return(nil)              // Commit AVANT Rollback
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes() // Rollback après

	req := httptest.NewRequest(http.MethodDelete, "/customers/cust-123", nil)
	req = setupChiContext(req, "cust-123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.DeleteCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestDeleteCustomerHandler_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	handler := customerhandler.NewCustomerHandler(mockRepo, mockTxManager)

	// Mock expectations
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTx)
	mockRepoWithTx.EXPECT().
		FindByCustomerID(gomock.Any(), "cust-999").
		Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil) // Pas de Commit, seulement Rollback

	req := httptest.NewRequest(http.MethodDelete, "/customers/cust-999", nil)
	req = setupChiContext(req, "cust-999")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.DeleteCustomerHandler)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "CUSTOMER_NOT_FOUND", response["code"])
}
