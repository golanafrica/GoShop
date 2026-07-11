package customerusecase

import (
	"context"
	"testing"

	"Goshop/domain/entity"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.20 : TESTS UNITAIRES - FONCTIONS PRIVÉES
// ============================================================

// ============================================================
// TESTS : maskEmail
// ============================================================

func TestMaskEmail_Empty(t *testing.T) {
	result := maskEmail("")
	assert.Equal(t, "", result)
}

func TestMaskEmail_InvalidFormat(t *testing.T) {
	result := maskEmail("invalid-email")
	assert.Equal(t, "***@***", result)
}

func TestMaskEmail_ShortUsername(t *testing.T) {
	result := maskEmail("ab@example.com")
	assert.Equal(t, "***@example.com", result)
}

func TestMaskEmail_LongUsername(t *testing.T) {
	result := maskEmail("john.doe@example.com")
	assert.Equal(t, "jo***@example.com", result)
}

// ============================================================
// TESTS : isValidEmails
// ============================================================

func TestIsValidEmails_Empty(t *testing.T) {
	result := isValidEmails("")
	assert.False(t, result)
}

func TestIsValidEmails_NoAtSymbol(t *testing.T) {
	result := isValidEmails("invalid-email")
	assert.False(t, result)
}

func TestIsValidEmails_NoDot(t *testing.T) {
	result := isValidEmails("user@domain")
	assert.False(t, result)
}

func TestIsValidEmails_Valid(t *testing.T) {
	result := isValidEmails("user@example.com")
	assert.True(t, result)
}

func TestIsValidEmails_EmptyParts(t *testing.T) {
	result := isValidEmails("@example.com")
	assert.False(t, result)
}

// ============================================================
// TESTS : normalizeCustomerData (UpdateCustomerUsecase)
// ============================================================

func TestUpdateCustomerUsecase_NormalizeCustomerData_WithChanges(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "  John  ",
		LastName:  "  Doe  ",
		Email:     "  JOHN@EXAMPLE.COM  ",
	}

	uc.normalizeCustomerData(ctx, customer)

	assert.Equal(t, "John", customer.FirstName)
	assert.Equal(t, "Doe", customer.LastName)
	assert.Equal(t, "john@example.com", customer.Email)
}

func TestUpdateCustomerUsecase_NormalizeCustomerData_NoChanges(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}

	uc.normalizeCustomerData(ctx, customer)

	assert.Equal(t, "John", customer.FirstName)
	assert.Equal(t, "Doe", customer.LastName)
	assert.Equal(t, "john@example.com", customer.Email)
}

// ============================================================
// TESTS : logChanges
// ============================================================

func TestUpdateCustomerUsecase_LogChanges_AllFields(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	existing := &entity.Customer{
		ID:        "cust-1",
		FirstName: "Old",
		LastName:  "Old",
		Email:     "old@example.com",
	}

	new := &entity.Customer{
		FirstName: "New",
		LastName:  "New",
		Email:     "new@example.com",
	}

	changes := uc.logChanges(ctx, existing, new)

	assert.Len(t, changes, 3)
	assert.Contains(t, changes, "first_name")
	assert.Contains(t, changes, "last_name")
	assert.Contains(t, changes, "email")
}

func TestUpdateCustomerUsecase_LogChanges_NoChanges(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	existing := &entity.Customer{
		ID:        "cust-1",
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}

	new := &entity.Customer{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}

	changes := uc.logChanges(ctx, existing, new)

	assert.Len(t, changes, 0)
}

// ============================================================
// TESTS : applyUpdates
// ============================================================

func TestUpdateCustomerUsecase_ApplyUpdates_AllFields(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	existing := &entity.Customer{
		ID:        "cust-1",
		FirstName: "Old",
		LastName:  "Old",
		Email:     "old@example.com",
	}

	new := &entity.Customer{
		FirstName: "New",
		LastName:  "New",
		Email:     "new@example.com",
	}

	changes := []string{"first_name", "last_name", "email"}
	result := uc.applyUpdates(ctx, existing, new, changes)

	assert.Equal(t, "New", result.FirstName)
	assert.Equal(t, "New", result.LastName)
	assert.Equal(t, "new@example.com", result.Email)
}

func TestUpdateCustomerUsecase_ApplyUpdates_EmptyFields(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	existing := &entity.Customer{
		ID:        "cust-1",
		FirstName: "Old",
		LastName:  "Old",
		Email:     "old@example.com",
	}

	new := &entity.Customer{
		FirstName: "",
		LastName:  "",
		Email:     "",
	}

	changes := []string{}
	result := uc.applyUpdates(ctx, existing, new, changes)

	assert.Equal(t, "Old", result.FirstName)
	assert.Equal(t, "Old", result.LastName)
	assert.Equal(t, "old@example.com", result.Email)
}

// ============================================================
// TESTS : validateCustomerData (CreateCustomerUsecase)
// ============================================================

func TestCreateCustomerUsecase_ValidateCustomerData_EmptyLastName(t *testing.T) {
	uc := &CreateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "",
		Email:     "john@example.com",
	}

	err := uc.validateCustomerData(ctx, customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "last name is required")
}

func TestCreateCustomerUsecase_ValidateCustomerData_LastNameTooShort(t *testing.T) {
	uc := &CreateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "D",
		Email:     "john@example.com",
	}

	err := uc.validateCustomerData(ctx, customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "last name must be at least 2 characters")
}

func TestCreateCustomerUsecase_ValidateCustomerData_InvalidEmail(t *testing.T) {
	uc := &CreateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "invalid-email",
	}

	err := uc.validateCustomerData(ctx, customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "valid email")
}

func TestCreateCustomerUsecase_ValidateCustomerData_Valid(t *testing.T) {
	uc := &CreateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}

	err := uc.validateCustomerData(ctx, customer)

	assert.NoError(t, err)
}

// ============================================================
// TESTS : validateUpdateData (UpdateCustomerUsecase)
// ============================================================

func TestUpdateCustomerUsecase_ValidateUpdateData_FirstNameTooShort(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "J",
	}

	err := uc.validateUpdateData(ctx, customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "first name must be at least 2 characters")
}

func TestUpdateCustomerUsecase_ValidateUpdateData_LastNameTooShort(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		LastName: "D",
	}

	err := uc.validateUpdateData(ctx, customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "last name must be at least 2 characters")
}

func TestUpdateCustomerUsecase_ValidateUpdateData_InvalidEmail(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		Email: "invalid-email",
	}

	err := uc.validateUpdateData(ctx, customer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "valid email")
}

func TestUpdateCustomerUsecase_ValidateUpdateData_Valid(t *testing.T) {
	uc := &UpdateCustomerUsecase{}
	ctx := zerolog.New(nil).WithContext(context.Background())

	customer := &entity.Customer{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john@example.com",
	}

	err := uc.validateUpdateData(ctx, customer)

	assert.NoError(t, err)
}
