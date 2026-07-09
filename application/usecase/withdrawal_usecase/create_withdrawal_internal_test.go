package withdrawalusecase

import (
	"os"
	"testing"
	"time"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	"Goshop/domain/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// TESTS BOÎTE BLANCHE : accès aux fonctions non exportées
// ============================================================

func TestGetEnvOrDefault_ValuePresent(t *testing.T) {
	os.Setenv("TEST_ENV_VAR_XYZ", "custom-value")
	defer os.Unsetenv("TEST_ENV_VAR_XYZ")

	result := getEnvOrDefault("TEST_ENV_VAR_XYZ", "default-value")
	assert.Equal(t, "custom-value", result)
}

func TestGetEnvOrDefault_ValueAbsent(t *testing.T) {
	os.Unsetenv("TEST_ENV_VAR_ABSENT")

	result := getEnvOrDefault("TEST_ENV_VAR_ABSENT", "default-value")
	assert.Equal(t, "default-value", result)
}

func TestToResponse_AllOptionalFields(t *testing.T) {
	errMsg := "provider timeout"
	opTxID := "OP-TX-789"
	processedAt := time.Now()
	providerRef := "REF-123"
	destName := "John Doe"
	destEmail := "john@example.com"
	description := "test"

	w := &entity.Withdrawal{
		ID:                    uuid.New(),
		ShopID:                uuid.New(),
		AmountCents:           50000,
		Currency:              "XOF",
		Status:                entity.WithdrawalStatusFailed,
		PaymentMethod:         entity.WithdrawalPaymentMethod("ORANGE_MONEY"),
		DestinationNumber:     "+22670123456",
		ProviderRef:           &providerRef,
		DestinationName:       &destName,
		DestinationEmail:      &destEmail,
		Description:           &description,
		ErrorMessage:          &errMsg,
		OperatorTransactionID: &opTxID,
		ProcessedAt:           &processedAt,
		CreatedAt:             time.Now(),
	}

	resp := toResponse(w)

	assert.Equal(t, errMsg, resp.ErrorMessage)
	assert.Equal(t, opTxID, resp.OperatorTransactionID)
	assert.NotEmpty(t, resp.ProcessedAt)
	assert.Equal(t, providerRef, resp.ProviderRef)
	assert.Equal(t, destName, resp.DestinationName)
	assert.Equal(t, destEmail, resp.DestinationEmail)
	assert.Equal(t, description, resp.Description)
}

var _ = withdrawaldto.WithdrawalResponse{} // évite un import inutilisé si besoin
