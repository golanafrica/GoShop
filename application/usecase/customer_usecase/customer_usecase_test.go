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

//
// -----------------------------------------------------------
// CREATE CUSTOMER
// -----------------------------------------------------------
//

func TestCreateCustomerUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@mail.com",
	}

	createdCustomer := &entity.Customer{
		ID:        "c123",
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@mail.com",
	}

	// Mock expectations - CORRECTION ICI
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)

	// AVANT : FindByCustomerID
	// APRÈS : FindByEmail
	mockRepoTx.EXPECT().FindByEmail(gomock.Any(), "john@mail.com").Return(nil, sql.ErrNoRows)

	mockRepoTx.EXPECT().Create(gomock.Any(), gomock.Any()).Return(createdCustomer, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := customerusecase.NewCreateCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	result, err := uc.Execute(context.Background(), customer)

	assert.NoError(t, err)
	assert.Equal(t, "c123", result.ID)
}

func TestCreateCustomerUsecase_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	uc := customerusecase.NewCreateCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	_, err := uc.Execute(context.Background(), &entity.Customer{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer first name is required")
}

func TestCreateCustomerUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewCreateCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	_, err := uc.Execute(context.Background(), &entity.Customer{
		FirstName: "Valid",
		LastName:  "Name",
		Email:     "valid@mail.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}

//
// -----------------------------------------------------------
// GET CUSTOMER BY ID
// -----------------------------------------------------------
//

func TestGetCustomerByIdUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	customer := &entity.Customer{
		ID:        "cust-123",
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}

	// Mock expectations - AVEC TRANSACTION
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "cust-123").Return(customer, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := customerusecase.NewGetCustomerByIdUsecase(
		mockRepo,
		mockTxManager,
	)

	result, err := uc.Execute(context.Background(), "cust-123")

	assert.NoError(t, err)
	assert.Equal(t, customer, result)
}

func TestGetCustomerByIdUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	// Mock expectations - AVEC TRANSACTION
	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "unknown").Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil)

	uc := customerusecase.NewGetCustomerByIdUsecase(
		mockRepo,
		mockTxManager,
	)

	_, err := uc.Execute(context.Background(), "unknown")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer not found")
}

//
// -----------------------------------------------------------
// GET ALL CUSTOMERS
// -----------------------------------------------------------
//

func TestGetAllCustomersUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindAllCustomers(gomock.Any()).Return([]*entity.Customer{
		{ID: "1"}, {ID: "2"},
	}, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := customerusecase.NewGetAllCustomersUsecase(
		mockRepo,
		mockTxManager,
	)

	result, err := uc.Execute(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetAllCustomersUsecase_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindAllCustomers(gomock.Any()).Return(nil, errors.New("db error"))
	mockTx.EXPECT().Rollback().Return(nil)

	uc := customerusecase.NewGetAllCustomersUsecase(
		mockRepo,
		mockTxManager,
	)

	_, err := uc.Execute(context.Background())

	assert.Error(t, err)
}

//
// -----------------------------------------------------------
// UPDATE CUSTOMER
// -----------------------------------------------------------
//

func TestUpdateCustomerUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	customer := &entity.Customer{ID: "id1", FirstName: "New"}

	existing := &entity.Customer{
		ID:        "id1",
		FirstName: "Old",
		LastName:  "X",
		Email:     "old@mail.com",
	}

	updated := &entity.Customer{
		ID:        "id1",
		FirstName: "New",
		LastName:  "X",
		Email:     "old@mail.com",
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(existing, nil)
	mockRepoTx.EXPECT().UpdateCustomer(gomock.Any(), updated).Return(updated, nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := customerusecase.NewUpdateCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	result, err := uc.Execute(context.Background(), customer)

	assert.NoError(t, err)
	assert.Equal(t, "New", result.FirstName)
}

func TestUpdateCustomerUsecase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	customer := &entity.Customer{ID: "unknown"}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "unknown").Return(nil, sql.ErrNoRows)
	mockTx.EXPECT().Rollback().Return(nil)

	uc := customerusecase.NewUpdateCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	_, err := uc.Execute(context.Background(), customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer not found")
}

//
// -----------------------------------------------------------
// DELETE CUSTOMER
// -----------------------------------------------------------
//

func TestDeleteCustomerUsecase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockTx := mockrepo.NewMockTx(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)
	mockRepoTx := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	existingCustomer := &entity.Customer{
		ID:        "id1",
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@mail.com",
	}

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(mockTx, nil)
	mockRepo.EXPECT().WithTX(mockTx).Return(mockRepoTx)
	mockRepoTx.EXPECT().FindByCustomerID(gomock.Any(), "id1").Return(existingCustomer, nil)
	mockRepoTx.EXPECT().DeleteCustomer(gomock.Any(), "id1").Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil).AnyTimes()

	uc := customerusecase.NewDeleteCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	err := uc.Execute(context.Background(), "id1")

	assert.NoError(t, err)
}

func TestDeleteCustomerUsecase_BeginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTxManager := mockrepo.NewMockTxManager(ctrl)
	mockRepo := mockrepo.NewMockCustomerRepositoryInterface(ctrl)

	mockTxManager.EXPECT().BeginTx(gomock.Any()).Return(nil, errors.New("tx error"))

	uc := customerusecase.NewDeleteCustomerUsecase(
		mockRepo,
		mockTxManager,
	)

	err := uc.Execute(context.Background(), "id1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to begin transaction")
}
