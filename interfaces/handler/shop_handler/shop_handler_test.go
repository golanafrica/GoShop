package shophandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	shopdto "Goshop/application/dto/shop_dto"
	"Goshop/domain/entity"
	shophandler "Goshop/interfaces/handler/shop_handler"
	"Goshop/interfaces/middl"
	"Goshop/interfaces/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============ MOCK USECASES ============

type MockCreateShopUsecase struct {
	mock.Mock
}

func (m *MockCreateShopUsecase) Execute(ctx context.Context, name, slug, customDomain string) (*entity.Shop, error) {
	args := m.Called(ctx, name, slug, customDomain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Shop), args.Error(1)
}

type MockListShopsUsecase struct {
	mock.Mock
}

func (m *MockListShopsUsecase) Execute(ctx context.Context) ([]*entity.Shop, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Shop), args.Error(1)
}

type MockUpdateShopUsecase struct {
	mock.Mock
}

func (m *MockUpdateShopUsecase) Execute(ctx context.Context, shopID string, name *string, customDomain *string, plan *string, isActive *bool) (*entity.Shop, error) {
	args := m.Called(ctx, shopID, name, customDomain, plan, isActive)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Shop), args.Error(1)
}

// ============ HELPERS ============

func newTestShop(id, name, slug, ownerID string) *entity.Shop {
	return &entity.Shop{
		ID:       uuid.MustParse(id),
		Name:     name,
		Slug:     slug,
		OwnerID:  ownerID,
		Plan:     entity.ShopPlanFree,
		IsActive: true,
	}
}

func contextWithUser(userID string) context.Context {
	return utils.WithUserID(context.Background(), userID)
}

// ============ TESTS CREATE SHOP HANDLER ============

func TestShopHandler_CreateShop_Success(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	reqBody := shopdto.CreateShopRequest{
		Name: "Ma Boutique",
		Slug: "ma-boutique",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/shops", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	shop := newTestShop("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "Ma Boutique", "ma-boutique", "user-123")
	mockCreate.On("Execute", mock.Anything, "Ma Boutique", "ma-boutique", "").Return(shop, nil)

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var response shopdto.ShopResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Ma Boutique", response.Name)
	assert.Equal(t, "ma-boutique", response.Slug)
	mockCreate.AssertExpectations(t)
}

func TestShopHandler_CreateShop_InvalidPayload(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	req := httptest.NewRequest(http.MethodPost, "/api/shops", bytes.NewReader([]byte("invalid json")))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	mockCreate.AssertNotCalled(t, "Execute")
}

func TestShopHandler_CreateShop_ValidationFailed(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	reqBody := shopdto.CreateShopRequest{
		Name: "Ma Boutique",
		Slug: "INVALID SLUG!",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/shops", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	mockCreate.AssertNotCalled(t, "Execute")
}

func TestShopHandler_CreateShop_SlugTaken(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	reqBody := shopdto.CreateShopRequest{
		Name: "Ma Boutique",
		Slug: "ma-boutique",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/shops", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	mockCreate.On("Execute", mock.Anything, "Ma Boutique", "ma-boutique", "").Return(nil, fmt.Errorf("slug 'ma-boutique' is already taken"))

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.CreateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	mockCreate.AssertExpectations(t)
}

// ============ TESTS LIST SHOPS HANDLER ============

func TestShopHandler_ListShops_Success(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	req := httptest.NewRequest(http.MethodGet, "/api/shops", nil)
	req = req.WithContext(ctx)

	shops := []*entity.Shop{
		newTestShop("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "Shop 1", "shop-1", "user-123"),
		newTestShop("b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22", "Shop 2", "shop-2", "user-123"),
	}
	mockList.On("Execute", mock.Anything).Return(shops, nil)

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.ListShops)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response []shopdto.ShopResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)
	mockList.AssertExpectations(t)
}

func TestShopHandler_ListShops_Empty(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	req := httptest.NewRequest(http.MethodGet, "/api/shops", nil)
	req = req.WithContext(ctx)

	mockList.On("Execute", mock.Anything).Return([]*entity.Shop{}, nil)

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.ListShops)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response []shopdto.ShopResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Empty(t, response)
	mockList.AssertExpectations(t)
}

// ============ TESTS UPDATE SHOP HANDLER ============

func TestShopHandler_UpdateShop_Success(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	newName := "Nouveau Nom"

	reqBody := shopdto.UpdateShopRequest{
		Name: &newName,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/shops/"+shopID, bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Simuler le URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", shopID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	shop := newTestShop(shopID, "Nouveau Nom", "shop-1", "user-123")
	// ✅ Utiliser mock.Anything pour le contexte (chi ajoute RouteContext)
	mockUpdate.On("Execute", mock.Anything, shopID, &newName, (*string)(nil), (*string)(nil), (*bool)(nil)).Return(shop, nil)

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockUpdate.AssertExpectations(t)
}

func TestShopHandler_UpdateShop_MissingID(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	req := httptest.NewRequest(http.MethodPut, "/api/shops/", nil)
	req = req.WithContext(ctx)

	// Pas de URL param
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	mockUpdate.AssertNotCalled(t, "Execute")
}

func TestShopHandler_UpdateShop_NotOwner(t *testing.T) {
	mockCreate := new(MockCreateShopUsecase)
	mockList := new(MockListShopsUsecase)
	mockUpdate := new(MockUpdateShopUsecase)

	handler := shophandler.NewShopHandler(mockCreate, mockList, mockUpdate)

	ctx := contextWithUser("user-123")
	shopID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	newName := "Nouveau Nom"

	reqBody := shopdto.UpdateShopRequest{
		Name: &newName,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/shops/"+shopID, bytes.NewReader(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", shopID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// ✅ Utiliser mock.Anything pour le contexte
	mockUpdate.On("Execute", mock.Anything, shopID, &newName, (*string)(nil), (*string)(nil), (*bool)(nil)).
		Return(nil, fmt.Errorf("you are not the owner of this shop"))

	rr := httptest.NewRecorder()

	httpHandler := middl.ErrorHandler(handler.UpdateShop)
	httpHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	mockUpdate.AssertExpectations(t)
}
