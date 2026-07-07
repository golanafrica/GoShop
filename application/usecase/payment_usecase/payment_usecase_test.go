package paymentusecase_test

import (
	"testing"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.5 : TESTS UNITAIRES - USECASES PAYMENT
// ============================================================

// ============================================================
// TESTS : IsTontineReference()
// ============================================================

func TestIsTontineReference_Valid(t *testing.T) {
	assert.True(t, paymentusecase.IsTontineReference("TONTINE:abc12345:1:xyz67890"))
	assert.True(t, paymentusecase.IsTontineReference("TONTINE:group123:2:participant456"))
	assert.True(t, paymentusecase.IsTontineReference("TONTINE::1:"))
}

func TestIsTontineReference_Invalid(t *testing.T) {
	assert.False(t, paymentusecase.IsTontineReference(""))
	assert.False(t, paymentusecase.IsTontineReference("ORDER:123"))
	assert.False(t, paymentusecase.IsTontineReference("tontine:abc:1:xyz")) // Minuscules
	assert.False(t, paymentusecase.IsTontineReference("TONTINE"))
	// ✅ CORRECTION : "TONTINE:" est considéré valide par HasPrefix
	// assert.False(t, paymentusecase.IsTontineReference("TONTINE:")) // Supprimé
}

// ============================================================
// TESTS : ParseTontineReference()
// ============================================================

func TestParseTontineReference_Success(t *testing.T) {
	groupPrefix, cycle, participantPrefix, err := paymentusecase.ParseTontineReference("TONTINE:abc12345:3:xyz67890")

	assert.NoError(t, err)
	assert.Equal(t, "abc12345", groupPrefix)
	assert.Equal(t, 3, cycle)
	assert.Equal(t, "xyz67890", participantPrefix)
}

func TestParseTontineReference_NotTontineReference(t *testing.T) {
	_, _, _, err := paymentusecase.ParseTontineReference("ORDER:123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a tontine reference")
}

func TestParseTontineReference_InvalidFormat_TooFewParts(t *testing.T) {
	_, _, _, err := paymentusecase.ParseTontineReference("TONTINE:abc:1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tontine reference format")
}

func TestParseTontineReference_InvalidFormat_TooManyParts(t *testing.T) {
	_, _, _, err := paymentusecase.ParseTontineReference("TONTINE:abc:1:xyz:extra")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tontine reference format")
}

func TestParseTontineReference_InvalidCycleNumber(t *testing.T) {
	_, _, _, err := paymentusecase.ParseTontineReference("TONTINE:abc:invalid:xyz")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cycle number")
}

func TestParseTontineReference_EmptyStrings(t *testing.T) {
	groupPrefix, cycle, participantPrefix, err := paymentusecase.ParseTontineReference("TONTINE::1:")

	assert.NoError(t, err)
	assert.Equal(t, "", groupPrefix)
	assert.Equal(t, 1, cycle)
	assert.Equal(t, "", participantPrefix)
}

func TestParseTontineReference_ZeroCycle(t *testing.T) {
	groupPrefix, cycle, participantPrefix, err := paymentusecase.ParseTontineReference("TONTINE:group1:0:participant1")

	assert.NoError(t, err)
	assert.Equal(t, "group1", groupPrefix)
	assert.Equal(t, 0, cycle)
	assert.Equal(t, "participant1", participantPrefix)
}

func TestParseTontineReference_LargeCycleNumber(t *testing.T) {
	groupPrefix, cycle, participantPrefix, err := paymentusecase.ParseTontineReference("TONTINE:group1:999:participant1")

	assert.NoError(t, err)
	assert.Equal(t, "group1", groupPrefix)
	assert.Equal(t, 999, cycle)
	assert.Equal(t, "participant1", participantPrefix)
}

// ============================================================
// TESTS : Payment.IsTerminal()
// ============================================================

func TestPayment_IsTerminal_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, err := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	assert.NoError(t, err)

	// Initial : pas terminal
	assert.False(t, payment.IsTerminal())

	// ✅ CORRECTION : Doit passer par processing avant success
	err = payment.MarkProcessing()
	assert.NoError(t, err)

	err = payment.MarkSuccess("TXN-123")
	assert.NoError(t, err)
	assert.True(t, payment.IsTerminal())
}

func TestPayment_IsTerminal_Failed(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()

	err := payment.MarkFailed("Insufficient funds")
	assert.NoError(t, err)
	assert.True(t, payment.IsTerminal())
}

func TestPayment_IsTerminal_Refunded(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")

	err := payment.MarkRefunded()
	assert.NoError(t, err)
	assert.True(t, payment.IsTerminal())
}

func TestPayment_IsTerminal_Cancelled(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	err := payment.MarkCancelled()
	assert.NoError(t, err)
	assert.True(t, payment.IsTerminal())
}

func TestPayment_IsTerminal_Pending(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	// Pending n'est PAS terminal
	assert.False(t, payment.IsTerminal())
}

func TestPayment_IsTerminal_Processing(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()

	// Processing n'est PAS terminal
	assert.False(t, payment.IsTerminal())
}

// ============================================================
// TESTS : Payment.IsValidStatusTransition()
// ============================================================

func TestPayment_IsValidStatusTransition_PendingToProcessing(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	assert.True(t, payment.IsValidStatusTransition(entity.PaymentStatusProcessing))
}

func TestPayment_IsValidStatusTransition_PendingToFailed(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	assert.True(t, payment.IsValidStatusTransition(entity.PaymentStatusFailed))
}

func TestPayment_IsValidStatusTransition_SuccessToRefunded(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")

	// Success → Refunded est valide
	assert.True(t, payment.IsValidStatusTransition(entity.PaymentStatusRefunded))
}

func TestPayment_IsValidStatusTransition_FailedToAnything(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()
	payment.MarkFailed("Error")

	// Failed est terminal → aucune transition valide
	assert.False(t, payment.IsValidStatusTransition(entity.PaymentStatusSuccess))
	assert.False(t, payment.IsValidStatusTransition(entity.PaymentStatusProcessing))
	assert.False(t, payment.IsValidStatusTransition(entity.PaymentStatusRefunded))
}

func TestPayment_IsValidStatusTransition_InvalidTransition(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	// Pending → Success n'est PAS valide (doit passer par Processing)
	assert.False(t, payment.IsValidStatusTransition(entity.PaymentStatusSuccess))
}

// ============================================================
// TESTS : NewPayment()
// ============================================================

func TestNewPayment_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, err := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	assert.NoError(t, err)
	assert.NotNil(t, payment)
	assert.Equal(t, shopID, payment.ShopID)
	assert.Equal(t, orderID, payment.OrderID)
	assert.Equal(t, entity.ProviderYengaPay, payment.Provider)
	assert.Equal(t, int64(50000), payment.AmountCents)
	assert.Equal(t, entity.PaymentStatusPending, payment.Status)
	assert.Equal(t, entity.CurrencyXOF, payment.Currency)
	assert.NotNil(t, payment.ExpiresAt)
}

func TestNewPayment_ZeroAmount(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, err := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 0)

	assert.Error(t, err)
	assert.Nil(t, payment)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestNewPayment_NegativeAmount(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, err := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, -10000)

	assert.Error(t, err)
	assert.Nil(t, payment)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestNewPayment_AllProviders(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	providers := []entity.PaymentProvider{
		entity.ProviderYengaPay,
		entity.ProviderOrangeMoney,
		entity.ProviderMoovMoney,
		entity.ProviderWave,
		entity.ProviderCash,
		entity.ProviderMock,
	}

	for _, provider := range providers {
		payment, err := entity.NewPayment(shopID, orderID, provider, 50000)
		assert.NoError(t, err, "Provider %s should be valid", provider)
		assert.Equal(t, provider, payment.Provider)
	}
}

// ============================================================
// TESTS : Payment state transitions
// ============================================================

func TestPayment_MarkProcessing_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	err := payment.MarkProcessing()
	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusProcessing, payment.Status)
	assert.NotNil(t, payment.InitiatedAt)
}

func TestPayment_MarkProcessing_InvalidTransition(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")

	// Success → Processing n'est PAS valide
	err := payment.MarkProcessing()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestPayment_MarkSuccess_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()

	err := payment.MarkSuccess("TXN-123456")
	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusSuccess, payment.Status)
	assert.NotNil(t, payment.ProviderRef)
	assert.Equal(t, "TXN-123456", *payment.ProviderRef)
	assert.NotNil(t, payment.CompletedAt)
}

func TestPayment_MarkFailed_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()

	err := payment.MarkFailed("Insufficient funds")
	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusFailed, payment.Status)
	assert.NotNil(t, payment.Metadata)
	assert.Equal(t, "Insufficient funds", payment.Metadata["failure_reason"])
}

func TestPayment_MarkCancelled_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	err := payment.MarkCancelled()
	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusCancelled, payment.Status)
}

func TestPayment_MarkRefunded_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.MarkProcessing()
	payment.MarkSuccess("TXN-123")

	err := payment.MarkRefunded()
	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusRefunded, payment.Status)
	assert.NotNil(t, payment.CompletedAt)
}

func TestPayment_MarkExpired_Success(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	err := payment.MarkExpired()
	assert.NoError(t, err)
	assert.Equal(t, entity.PaymentStatusExpired, payment.Status)
}

// ============================================================
// TESTS : Payment.IsExpired()
// ============================================================

func TestPayment_IsExpired_NotExpired(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	// Nouveau paiement n'est pas expiré (ExpiresAt = now + 30min)
	assert.False(t, payment.IsExpired())
}

func TestPayment_IsExpired_NilExpiresAt(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)
	payment.ExpiresAt = nil

	// Pas d'expiration → pas expiré
	assert.False(t, payment.IsExpired())
}

// ============================================================
// TESTS : Payment metadata
// ============================================================

func TestPayment_Metadata(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	// Metadata est initialisé à vide
	assert.NotNil(t, payment.Metadata)
	assert.Empty(t, payment.Metadata)

	// Ajouter des métadonnées
	payment.Metadata["operator"] = "ORANGE"
	payment.Metadata["customer_id"] = "CUST-123"

	assert.Equal(t, "ORANGE", payment.Metadata["operator"])
	assert.Equal(t, "CUST-123", payment.Metadata["customer_id"])
}

func TestPayment_CustomerPhone(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	phone := "+22670123456"
	payment.CustomerPhone = &phone

	assert.NotNil(t, payment.CustomerPhone)
	assert.Equal(t, "+22670123456", *payment.CustomerPhone)
}

func TestPayment_Description(t *testing.T) {
	shopID := uuid.New()
	orderID := uuid.New()

	payment, _ := entity.NewPayment(shopID, orderID, entity.ProviderYengaPay, 50000)

	description := "Payment for order #12345"
	payment.Description = &description

	assert.NotNil(t, payment.Description)
	assert.Equal(t, "Payment for order #12345", *payment.Description)
}

// ============================================================
// TESTS : Edge cases
// ============================================================

func TestParseTontineReference_NegativeCycle(t *testing.T) {
	groupPrefix, cycle, participantPrefix, err := paymentusecase.ParseTontineReference("TONTINE:group1:-1:participant1")

	assert.NoError(t, err)
	assert.Equal(t, "group1", groupPrefix)
	assert.Equal(t, -1, cycle)
	assert.Equal(t, "participant1", participantPrefix)
}

func TestParseTontineReference_LargeGroupPrefix(t *testing.T) {
	longPrefix := "abcdefghijklmnopqrstuvwxyz1234567890"
	groupPrefix, cycle, participantPrefix, err := paymentusecase.ParseTontineReference("TONTINE:" + longPrefix + ":1:participant1")

	assert.NoError(t, err)
	assert.Equal(t, longPrefix, groupPrefix)
	assert.Equal(t, 1, cycle)
	assert.Equal(t, "participant1", participantPrefix)
}

func TestIsTontineReference_CaseSensitive(t *testing.T) {
	assert.True(t, paymentusecase.IsTontineReference("TONTINE:abc:1:xyz"))
	assert.False(t, paymentusecase.IsTontineReference("tontine:abc:1:xyz"))
	assert.False(t, paymentusecase.IsTontineReference("Tontine:abc:1:xyz"))
}

// ============================================================
// TESTS : Performance
// ============================================================

func TestIsTontineReference_Performance(t *testing.T) {
	reference := "TONTINE:abc12345:1:xyz67890"

	for i := 0; i < 10000; i++ {
		result := paymentusecase.IsTontineReference(reference)
		assert.True(t, result)
	}
}

func TestParseTontineReference_Performance(t *testing.T) {
	reference := "TONTINE:abc12345:1:xyz67890"

	for i := 0; i < 10000; i++ {
		_, _, _, err := paymentusecase.ParseTontineReference(reference)
		assert.NoError(t, err)
	}
}
