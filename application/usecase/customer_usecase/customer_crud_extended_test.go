package customerusecase_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	customerusecase "Goshop/application/usecase/customer_usecase"
	"Goshop/domain/entity"
	mockrepo "Goshop/mocks/repository"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// ============================================================
// 🆕 v4.4.19 : TESTS COMPLÉMENTAIRES - CUSTOMER CRUD
// ============================================================

// ============================================================
// TESTS : CreateCustomerUsecase - Branches manquantes
// ============================================================

func TestCreateCustomerUsecase_EmailAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	existingCustomer := &entity.Customer{ID: "existing-id", Email: "john@mail.com"}
	mockRepoTx.EXPECT().FindByEmail(gomock.Any(), "john@mail.com").Return(existingCustomer, nil)

	// ✅ CORRECTION : FindUserByEmail ne prend qu'un seul argument (l'email)
	mockUserRepo.EXPECT().FindUserByEmail("john@mail.com").Return(nil, nil).AnyTimes()

	uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)

	customer := &entity.Customer{FirstName: "John", LastName: "Doe", Email: "john@mail.com"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreateCustomerUsecase_FindByEmailError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	// ✅ AJOUT : Mock pour FindUserByEmail qui est appelé AVANT FindByEmail
	mockUserRepo.EXPECT().FindUserByEmail("john@mail.com").Return(nil, nil).AnyTimes()

	// Erreur DB (pas ErrNoRows) → le code continue vers Create
	mockRepoTx.EXPECT().FindByEmail(gomock.Any(), "john@mail.com").Return(nil, errors.New("db error"))

	// ✅ AJOUTER : Le code continue et appelle Create (qui échoue aussi)
	mockRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)

	customer := &entity.Customer{FirstName: "John", LastName: "Doe", Email: "john@mail.com"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreateCustomerUsecase_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockUserRepo.EXPECT().FindUserByEmail("john@mail.com").Return(nil, nil).AnyTimes()
	mockRepoTx.EXPECT().FindByEmail(gomock.Any(), "john@mail.com").Return(nil, sql.ErrNoRows)
	mockRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)

	customer := &entity.Customer{FirstName: "John", LastName: "Doe", Email: "john@mail.com"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreateCustomerUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockUserRepo.EXPECT().FindUserByEmail("john@mail.com").Return(nil, nil).AnyTimes()
	mockRepoTx.EXPECT().FindByEmail(gomock.Any(), "john@mail.com").Return(nil, sql.ErrNoRows)
	mockRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&entity.Customer{ID: "c123"}, nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)

	customer := &entity.Customer{FirstName: "John", LastName: "Doe", Email: "john@mail.com"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreateCustomerUsecase_InvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)

	customer := &entity.Customer{FirstName: "John", LastName: "Doe", Email: "invalid-email"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "valid email")
}

func TestCreateCustomerUsecase_EmptyLastName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockUserRepo := mockrepo.NewMockUserRepository(ctrl)

	uc := customerusecase.NewCreateCustomerUsecase(mockRepo, mockUserRepo, mockTxManager)

	customer := &entity.Customer{FirstName: "John", LastName: "", Email: "john@mail.com"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "last name is required")
}

// ============================================================
// TESTS : GetCustomerByIdUsecase - Branches manquantes
// ============================================================

func TestGetCustomerByIdUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewGetCustomerByIdUsecase(mockRepo, mockTxManager)

	result, err := uc.Execute(context.Background(), "cust-123")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetCustomerByIdUsecase_FindByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-123").Return(nil, errors.New("db error"))

	uc := customerusecase.NewGetCustomerByIdUsecase(mockRepo, mockTxManager)

	result, err := uc.Execute(context.Background(), "cust-123")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetCustomerByIdUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	customer := &entity.Customer{ID: "cust-123", FirstName: "John"}
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-123").Return(customer, nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewGetCustomerByIdUsecase(mockRepo, mockTxManager)

	result, err := uc.Execute(context.Background(), "cust-123")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// ============================================================
// TESTS : GetAllCustomersUsecase - Branches manquantes
// ============================================================

func TestGetAllCustomersUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewGetAllCustomersUsecase(mockRepo, mockTxManager)

	result, err := uc.Execute(context.Background())

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetAllCustomersUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockRepoTx.EXPECT().FindAllCustomers(gomock.Any()).Return([]*entity.Customer{}, nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewGetAllCustomersUsecase(mockRepo, mockTxManager)

	result, err := uc.Execute(context.Background())

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetAllCustomersUsecase_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockRepoTx.EXPECT().FindAllCustomers(gomock.Any()).Return([]*entity.Customer{}, nil)

	uc := customerusecase.NewGetAllCustomersUsecase(mockRepo, mockTxManager)

	result, err := uc.Execute(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

// ============================================================
// TESTS : UpdateCustomerUsecase - Branches manquantes
// ============================================================

func TestUpdateCustomerUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewUpdateCustomerUsecase(mockRepo, mockTxManager)

	customer := &entity.Customer{ID: "id1", FirstName: "New"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateCustomerUsecase_FindByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(nil, errors.New("db error"))

	uc := customerusecase.NewUpdateCustomerUsecase(mockRepo, mockTxManager)

	customer := &entity.Customer{ID: "id1", FirstName: "New"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateCustomerUsecase_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	existing := &entity.Customer{ID: "id1", FirstName: "Old", LastName: "X", Email: "old@mail.com"}
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(existing, nil)
	mockRepoTx.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	uc := customerusecase.NewUpdateCustomerUsecase(mockRepo, mockTxManager)

	customer := &entity.Customer{ID: "id1", FirstName: "New"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateCustomerUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	existing := &entity.Customer{ID: "id1", FirstName: "Old", LastName: "X", Email: "old@mail.com"}
	updated := &entity.Customer{ID: "id1", FirstName: "New", LastName: "X", Email: "old@mail.com"}
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(existing, nil)
	mockRepoTx.EXPECT().UpdateCustomer(gomock.Any(), gomock.Any()).Return(updated, nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewUpdateCustomerUsecase(mockRepo, mockTxManager)

	customer := &entity.Customer{ID: "id1", FirstName: "New"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateCustomerUsecase_InvalidEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := customerusecase.NewUpdateCustomerUsecase(mockRepo, mockTxManager)

	customer := &entity.Customer{ID: "id1", Email: "invalid-email"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "valid email")
}

func TestUpdateCustomerUsecase_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := customerusecase.NewUpdateCustomerUsecase(mockRepo, mockTxManager)

	customer := &entity.Customer{ID: "", FirstName: "New"}
	result, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "customer ID is required")
}

// ============================================================
// TESTS : DeleteCustomerUsecase - Branches manquantes
// ============================================================

func TestDeleteCustomerUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "unknown").Return(nil, sql.ErrNoRows)

	uc := customerusecase.NewDeleteCustomerUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "unknown")

	assert.Error(t, err)
}

func TestDeleteCustomerUsecase_FindByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(nil, errors.New("db error"))

	uc := customerusecase.NewDeleteCustomerUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "id1")

	assert.Error(t, err)
}

func TestDeleteCustomerUsecase_DeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	existingCustomer := &entity.Customer{ID: "id1", FirstName: "John"}
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(existingCustomer, nil)
	mockRepoTx.EXPECT().DeleteCustomer(gomock.Any(), "id1").Return(errors.New("db error"))

	uc := customerusecase.NewDeleteCustomerUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "id1")

	assert.Error(t, err)
}

func TestDeleteCustomerUsecase_CommitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	existingCustomer := &entity.Customer{ID: "id1", FirstName: "John"}
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(existingCustomer, nil)
	mockRepoTx.EXPECT().DeleteCustomer(gomock.Any(), "id1").Return(nil)
	mockTx.EXPECT().Commit().Return(errors.New("commit failed"))

	uc := customerusecase.NewDeleteCustomerUsecase(mockRepo, mockTxManager)

	err := uc.Execute(context.Background(), "id1")

	assert.Error(t, err)
}
