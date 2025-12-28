// interfaces/handler/product/product_handler_test.go
package producthandler_test

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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	dto "Goshop/application/dto/product_dto"
	"Goshop/domain/entity"
	producthandler "Goshop/interfaces/handler/product"
	"Goshop/interfaces/middl"
	mockrepo "Goshop/mocks/repository"
)

func createTestProduct(id string) *entity.Product {
	return &entity.Product{
		ID:          id,
		Name:        "Test Product " + id,
		Description: "Description " + id,
		PriceCents:  1000 + int64(len(id)*100),
		Stock:       10,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func setupChiContext(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ========================================
// CREATE PRODUCT TESTS (avec transactions)
// ========================================

func TestProductHandler_CreateProduct_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTX := mockrepo.NewMockProductRepository(ctrl)

	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	reqBody := dto.CreateProductRequest{
		Name:        "New Laptop",
		Description: "High performance laptop",
		PriceCents:  150000,
		Stock:       25,
	}

	// ✅ CREATE utilise une transaction
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTX)
	mockRepoWithTX.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, product *entity.Product) error {
			product.ID = "generated-id-123"
			product.CreatedAt = time.Now()
			product.UpdatedAt = time.Now()
			return nil
		})
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/products", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateProduct)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp dto.ProductResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "New Laptop", resp.Name)
	assert.Equal(t, "High performance laptop", resp.Description)
	assert.Equal(t, int64(150000), resp.PriceCents)
	assert.Equal(t, 25, resp.Stock)
	assert.NotEmpty(t, resp.ID)
}

func TestProductHandler_CreateProduct_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	invalidJSON := `{"name": "test", "price_cents": "not a number"}`

	req := httptest.NewRequest("POST", "/products", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateProduct)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_PAYLOAD", resp["code"])
}

// ========================================
// GET PRODUCT BY ID TESTS (sans transactions)
// ========================================

func TestProductHandler_GetProductById_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl) // Pas utilisé mais nécessaire pour le constructeur

	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	product := createTestProduct("123")

	// ✅ GET BY ID n'utilise PAS de transaction
	mockRepo.EXPECT().FindByID(gomock.Any(), "123").Return(product, nil)
	// ❌ NE PAS mocker: BeginTx, WithTX, Commit, Rollback

	req := httptest.NewRequest("GET", "/products/123", nil)
	req = setupChiContext(req, "123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetProductById)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.ProductResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "123", resp.ID)
	assert.Equal(t, "Test Product 123", resp.Name)
}

func TestProductHandler_GetProductById_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	// ✅ GET BY ID n'utilise PAS de transaction
	mockRepo.EXPECT().FindByID(gomock.Any(), "999").Return(nil, sql.ErrNoRows)
	// ❌ NE PAS mocker: BeginTx, WithTX, Rollback

	req := httptest.NewRequest("GET", "/products/999", nil)
	req = setupChiContext(req, "999")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetProductById)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "PRODUCT_NOT_FOUND", resp["code"])
}

// ========================================
// UPDATE PRODUCT TESTS (avec transactions)
// ========================================

func TestProductHandler_UpdateProduct_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTX := mockrepo.NewMockProductRepository(ctrl)

	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	reqBody := dto.UpdateProductRequest{
		Name:        "Updated Name",
		Description: "Updated Description",
		PriceCents:  2000,
		Stock:       15,
	}

	existingProduct := createTestProduct("123")
	updatedProduct := createTestProduct("123")
	updatedProduct.Name = "Updated Name"
	updatedProduct.Description = "Updated Description"
	updatedProduct.PriceCents = 2000
	updatedProduct.Stock = 15
	updatedProduct.UpdatedAt = time.Now()

	// ✅ UPDATE utilise une transaction
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTX)
	mockRepoWithTX.EXPECT().FindByID(gomock.Any(), "123").Return(existingProduct, nil)
	mockRepoWithTX.EXPECT().Update(gomock.Any(), gomock.Any()).Return(updatedProduct, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/products/123", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = setupChiContext(req, "123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateProduct)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.ProductResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.Name)
}

// ========================================
// DELETE PRODUCT TESTS (avec transactions)
// ========================================

func TestProductHandler_DeleteProduct_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTX := mockrepo.NewMockProductRepository(ctrl)

	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	// ✅ DELETE utilise une transaction
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTX)
	mockRepoWithTX.EXPECT().FindByID(gomock.Any(), "123").Return(createTestProduct("123"), nil)
	mockRepoWithTX.EXPECT().Delete(gomock.Any(), "123").Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	req := httptest.NewRequest("DELETE", "/products/123", nil)
	req = setupChiContext(req, "123")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.DeleteProduct)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestProductHandler_DeleteProduct_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoWithTX := mockrepo.NewMockProductRepository(ctrl)

	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	// ✅ DELETE utilise une transaction
	mockTxMgr.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoWithTX)
	mockRepoWithTX.EXPECT().FindByID(gomock.Any(), "999").Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil).Times(1)

	req := httptest.NewRequest("DELETE", "/products/999", nil)
	req = setupChiContext(req, "999")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.DeleteProduct)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "PRODUCT_NOT_FOUND", resp["code"])
}

// ========================================
// GET ALL PRODUCTS TESTS (sans transactions)
// ========================================

func TestProductHandler_GetAllProducts_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	products := []*entity.Product{
		createTestProduct("1"),
		createTestProduct("2"),
		createTestProduct("3"),
	}

	// ✅ GET ALL n'utilise PAS de transaction
	mockRepo.EXPECT().FindAll(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, limit, offset int) ([]*entity.Product, error) {
			assert.Equal(t, 50, limit) // Default limit
			assert.Equal(t, 0, offset) // Default offset
			return products, nil
		})
	// ❌ NE PAS mocker: BeginTx, WithTX, Commit, Rollback

	req := httptest.NewRequest("GET", "/products", nil)
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.GetAllProducts)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []dto.ProductResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 3)
}

// ========================================
// VALIDATION TESTS
// ========================================

func TestProductHandler_CreateProduct_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockProductRepository(ctrl)
	mockTxMgr := mockrepo.NewMockTxManager(ctrl)
	handler := producthandler.NewProductHandler(mockRepo, mockTxMgr)

	// Données avec prix négatif
	reqBody := map[string]interface{}{
		"name":        "Invalid Product",
		"description": "Product with negative price",
		"price_cents": -100,
		"stock":       10,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/products", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateProduct)
	httpHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
