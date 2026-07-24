package paymentusecase_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	paymentdto "Goshop/application/dto/payment_dto"
	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"
	mockrepo "Goshop/mocks/repository"
	mockusecase "Goshop/mocks/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.9 : TESTS UNITAIRES - COMPLETE PAYMENT USECASE
// ============================================================

// ============================================================
// MOCK MANUEL : Provider + ProviderCompletable
// ============================================================

// mockProviderAndCompletable est un mock manuel qui implémente
// à la fois payment.Provider et payment.ProviderCompletable
type mockProviderAndCompletable struct {
	ctrl     *gomock.Controller
	recorder *mockProviderAndCompletableRecorder
}

type mockProviderAndCompletableRecorder struct {
	mock *mockProviderAndCompletable
}

func newMockProviderAndCompletable(ctrl *gomock.Controller) *mockProviderAndCompletable {
	mock := &mockProviderAndCompletable{ctrl: ctrl}
	mock.recorder = &mockProviderAndCompletableRecorder{mock}
	return mock
}

func (m *mockProviderAndCompletable) EXPECT() *mockProviderAndCompletableRecorder {
	return m.recorder
}

// === payment.Provider methods ===

func (m *mockProviderAndCompletable) Code() entity.PaymentProvider {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Code")
	ret0, _ := ret[0].(entity.PaymentProvider)
	return ret0
}

func (m *mockProviderAndCompletableRecorder) Code() *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "Code", reflect.TypeOf((*mockProviderAndCompletable)(nil).Code))
}

func (m *mockProviderAndCompletable) InitiatePayment(ctx context.Context, req *payment.PaymentRequest) (*payment.PaymentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitiatePayment", ctx, req)
	ret0, _ := ret[0].(*payment.PaymentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderAndCompletableRecorder) InitiatePayment(ctx, req interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "InitiatePayment", reflect.TypeOf((*mockProviderAndCompletable)(nil).InitiatePayment), ctx, req)
}

func (m *mockProviderAndCompletable) CheckStatus(ctx context.Context, providerRef string) (*payment.PaymentStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckStatus", ctx, providerRef)
	ret0, _ := ret[0].(*payment.PaymentStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderAndCompletableRecorder) CheckStatus(ctx, providerRef interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "CheckStatus", reflect.TypeOf((*mockProviderAndCompletable)(nil).CheckStatus), ctx, providerRef)
}

func (m *mockProviderAndCompletable) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*payment.WebhookEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateWebhook", ctx, payload, signature)
	ret0, _ := ret[0].(*payment.WebhookEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderAndCompletableRecorder) ValidateWebhook(ctx, payload, signature interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "ValidateWebhook", reflect.TypeOf((*mockProviderAndCompletable)(nil).ValidateWebhook), ctx, payload, signature)
}

func (m *mockProviderAndCompletable) Refund(ctx context.Context, providerRef string, amountCents int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Refund", ctx, providerRef, amountCents)
	ret0, _ := ret[0].(error)
	return ret0
}

func (m *mockProviderAndCompletableRecorder) Refund(ctx, providerRef, amountCents interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "Refund", reflect.TypeOf((*mockProviderAndCompletable)(nil).Refund), ctx, providerRef, amountCents)
}

func (m *mockProviderAndCompletable) IsAvailable(ctx context.Context) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsAvailable", ctx)
	ret0, _ := ret[0].(bool)
	return ret0
}

func (m *mockProviderAndCompletableRecorder) IsAvailable(ctx interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "IsAvailable", reflect.TypeOf((*mockProviderAndCompletable)(nil).IsAvailable), ctx)
}

// === payment.ProviderCompletable method ===

func (m *mockProviderAndCompletable) CompletePayment(ctx context.Context, paymentIntentID, operatorCode, customerMSISDN, otp string) (*payment.CompletePaymentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CompletePayment", ctx, paymentIntentID, operatorCode, customerMSISDN, otp)
	ret0, _ := ret[0].(*payment.CompletePaymentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderAndCompletableRecorder) CompletePayment(ctx, paymentIntentID, operatorCode, customerMSISDN, otp interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "CompletePayment", reflect.TypeOf((*mockProviderAndCompletable)(nil).CompletePayment), ctx, paymentIntentID, operatorCode, customerMSISDN, otp)
}

// ============================================================
// HELPERS
// ============================================================

func createProcessingPaymentWithMetadata(shopID uuid.UUID, amountCents int64) *entity.Payment {
	payment, _ := entity.NewPayment(shopID, uuid.New(), entity.ProviderYengaPay, amountCents)
	payment.MarkProcessing()

	phone := "+22670123456"
	payment.CustomerPhone = &phone
	payment.Metadata = map[string]interface{}{
		"operator": "ORANGE",
	}

	return payment
}

func createValidCompleteRequest(paymentID string) *paymentdto.CompletePaymentRequest {
	return &paymentdto.CompletePaymentRequest{
		PaymentID: paymentID,
		OTP:       "123456",
	}
}

// ============================================================
// TESTS : CompletePaymentUsecase - Multi-tenant
// ============================================================

func TestCompletePaymentUsecase_MultiTenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION : Ajout de nil, nil pour les nouveaux paramètres
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := context.Background()
	req := createValidCompleteRequest(uuid.New().String())

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "multi-tenant")
}

// ============================================================
// TESTS : CompletePaymentUsecase - Validation
// ============================================================

func TestCompletePaymentUsecase_InvalidPaymentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	req := &paymentdto.CompletePaymentRequest{
		PaymentID: "invalid-uuid",
		OTP:       "123456",
	}

	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid payment_id format")
}

// ============================================================
// TESTS : CompletePaymentUsecase - Repository errors
// ============================================================

func TestCompletePaymentUsecase_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	paymentID := uuid.New()

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentID).
		Return(nil, errors.New("payment not found"))

	req := createValidCompleteRequest(paymentID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "payment not found")
}

// ============================================================
// TESTS : CompletePaymentUsecase - Règles métier
// ============================================================

func TestCompletePaymentUsecase_PaymentNotBelongToShop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()

	otherShopID := uuid.New()
	wrongShopPayment := createProcessingPaymentWithMetadata(otherShopID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), wrongShopPayment.ID).
		Return(wrongShopPayment, nil)

	req := createValidCompleteRequest(wrongShopPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "does not belong to current shop")
}

func TestCompletePaymentUsecase_PaymentNotProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	pendingPayment, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), pendingPayment.ID).
		Return(pendingPayment, nil)

	req := createValidCompleteRequest(pendingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "not in processing state")
}

func TestCompletePaymentUsecase_ProviderNotAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPaymentWithMetadata(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(nil, errors.New("provider not available"))

	req := createValidCompleteRequest(processingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "provider not available")
}

func TestCompletePaymentUsecase_ProviderNotCompletable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPaymentWithMetadata(shop.ID, 50000)

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(newMockProviderOnly(ctrl), nil)

	req := createValidCompleteRequest(processingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "does not support payment completion")
}

// ============================================================
// MOCK Provider Only (sans ProviderCompletable)
// ============================================================

type mockProviderOnly struct {
	ctrl     *gomock.Controller
	recorder *mockProviderOnlyRecorder
}

type mockProviderOnlyRecorder struct {
	mock *mockProviderOnly
}

func newMockProviderOnly(ctrl *gomock.Controller) *mockProviderOnly {
	mock := &mockProviderOnly{ctrl: ctrl}
	mock.recorder = &mockProviderOnlyRecorder{mock}
	return mock
}

func (m *mockProviderOnly) EXPECT() *mockProviderOnlyRecorder {
	return m.recorder
}

func (m *mockProviderOnly) Code() entity.PaymentProvider {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Code")
	ret0, _ := ret[0].(entity.PaymentProvider)
	return ret0
}

func (m *mockProviderOnlyRecorder) Code() *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "Code", reflect.TypeOf((*mockProviderOnly)(nil).Code))
}

func (m *mockProviderOnly) InitiatePayment(ctx context.Context, req *payment.PaymentRequest) (*payment.PaymentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitiatePayment", ctx, req)
	ret0, _ := ret[0].(*payment.PaymentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderOnlyRecorder) InitiatePayment(ctx, req interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "InitiatePayment", reflect.TypeOf((*mockProviderOnly)(nil).InitiatePayment), ctx, req)
}

func (m *mockProviderOnly) CheckStatus(ctx context.Context, providerRef string) (*payment.PaymentStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckStatus", ctx, providerRef)
	ret0, _ := ret[0].(*payment.PaymentStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderOnlyRecorder) CheckStatus(ctx, providerRef interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "CheckStatus", reflect.TypeOf((*mockProviderOnly)(nil).CheckStatus), ctx, providerRef)
}

func (m *mockProviderOnly) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*payment.WebhookEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateWebhook", ctx, payload, signature)
	ret0, _ := ret[0].(*payment.WebhookEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *mockProviderOnlyRecorder) ValidateWebhook(ctx, payload, signature interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "ValidateWebhook", reflect.TypeOf((*mockProviderOnly)(nil).ValidateWebhook), ctx, payload, signature)
}

func (m *mockProviderOnly) Refund(ctx context.Context, providerRef string, amountCents int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Refund", ctx, providerRef, amountCents)
	ret0, _ := ret[0].(error)
	return ret0
}

func (m *mockProviderOnlyRecorder) Refund(ctx, providerRef, amountCents interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "Refund", reflect.TypeOf((*mockProviderOnly)(nil).Refund), ctx, providerRef, amountCents)
}

func (m *mockProviderOnly) IsAvailable(ctx context.Context) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsAvailable", ctx)
	ret0, _ := ret[0].(bool)
	return ret0
}

func (m *mockProviderOnlyRecorder) IsAvailable(ctx interface{}) *gomock.Call {
	m.mock.ctrl.T.Helper()
	return m.mock.ctrl.RecordCallWithMethodType(m.mock, "IsAvailable", reflect.TypeOf((*mockProviderOnly)(nil).IsAvailable), ctx)
}

// ============================================================
// TESTS : CompletePaymentUsecase - Missing fields
// ============================================================

func TestCompletePaymentUsecase_MissingMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	paymentWithoutMetadata, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentWithoutMetadata.MarkProcessing()
	paymentWithoutMetadata.Metadata = nil

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentWithoutMetadata.ID).
		Return(paymentWithoutMetadata, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	req := createValidCompleteRequest(paymentWithoutMetadata.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "no metadata")
}

func TestCompletePaymentUsecase_MissingOperatorCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	paymentWithoutOperator, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentWithoutOperator.MarkProcessing()
	paymentWithoutOperator.Metadata = map[string]interface{}{
		"other_field": "value",
	}

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentWithoutOperator.ID).
		Return(paymentWithoutOperator, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	req := createValidCompleteRequest(paymentWithoutOperator.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "operator code not found")
}

func TestCompletePaymentUsecase_MissingCustomerPhone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	paymentWithoutPhone, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentWithoutPhone.MarkProcessing()
	paymentWithoutPhone.Metadata = map[string]interface{}{
		"operator": "ORANGE",
	}
	paymentWithoutPhone.CustomerPhone = nil

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentWithoutPhone.ID).
		Return(paymentWithoutPhone, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	req := createValidCompleteRequest(paymentWithoutPhone.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "customer phone not found")
}

func TestCompletePaymentUsecase_MissingProviderRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	paymentWithoutRef, _ := entity.NewPayment(shop.ID, uuid.New(), entity.ProviderYengaPay, 50000)
	paymentWithoutRef.MarkProcessing()
	paymentWithoutRef.Metadata = map[string]interface{}{
		"operator": "ORANGE",
	}
	phone := "+22670123456"
	paymentWithoutRef.CustomerPhone = &phone
	paymentWithoutRef.ProviderRef = nil

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), paymentWithoutRef.ID).
		Return(paymentWithoutRef, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	req := createValidCompleteRequest(paymentWithoutRef.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "provider reference not found")
}

// ============================================================
// TESTS : CompletePaymentUsecase - Provider errors
// ============================================================

func TestCompletePaymentUsecase_ProviderCompletionFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPaymentWithMetadata(shop.ID, 50000)
	providerRef := "YENGA-REF-123"
	processingPayment.ProviderRef = &providerRef

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	mockCompletable.EXPECT().
		CompletePayment(gomock.Any(), providerRef, "ORANGE", "+22670123456", "123456").
		Return(nil, errors.New("invalid OTP"))

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidCompleteRequest(processingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "complete payment with provider")
}

// ============================================================
// TESTS : CompletePaymentUsecase - Happy paths
// ============================================================

func TestCompletePaymentUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPaymentWithMetadata(shop.ID, 50000)
	providerRef := "YENGA-REF-123"
	processingPayment.ProviderRef = &providerRef

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	mockCompletable.EXPECT().
		CompletePayment(gomock.Any(), providerRef, "ORANGE", "+22670123456", "123456").
		Return(&payment.CompletePaymentResponse{
			Status:        "DONE",
			TransactionID: "TXN-456",
			Amount:        50000,
		}, nil)

	mockPaymentRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil)

	req := createValidCompleteRequest(processingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, entity.PaymentStatusSuccess, response.Status)
	assert.Equal(t, providerRef, response.ProviderRef)
}

func TestCompletePaymentUsecase_UnexpectedStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentRepo := mockrepo.NewMockPaymentRepository(ctrl)
	mockRegistry := mockusecase.NewMockPaymentRegistry(ctrl)
	mockCompletable := newMockProviderAndCompletable(ctrl)

	// ✅ CORRECTION
	uc := paymentusecase.NewCompletePaymentUsecase(mockPaymentRepo, mockRegistry, nil, nil)

	ctx := createTestContextForPayment()
	shop, _ := tenant.FromContext(ctx)

	processingPayment := createProcessingPaymentWithMetadata(shop.ID, 50000)
	providerRef := "YENGA-REF-123"
	processingPayment.ProviderRef = &providerRef

	mockPaymentRepo.EXPECT().
		FindByID(gomock.Any(), processingPayment.ID).
		Return(processingPayment, nil)

	mockRegistry.EXPECT().
		GetAvailable(gomock.Any(), entity.ProviderYengaPay).
		Return(mockCompletable, nil)

	mockCompletable.EXPECT().
		CompletePayment(gomock.Any(), providerRef, "ORANGE", "+22670123456", "123456").
		Return(&payment.CompletePaymentResponse{
			Status:        "PENDING",
			TransactionID: "TXN-789",
		}, nil)

	req := createValidCompleteRequest(processingPayment.ID.String())
	response, err := uc.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "unexpected status from provider")
}
