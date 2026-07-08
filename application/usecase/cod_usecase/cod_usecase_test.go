package codusecase_test

import (
	"testing"
	"time"

	codusecase "Goshop/application/usecase/cod_usecase"
	"Goshop/domain/entity"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.5 : TESTS UNITAIRES - USECASES COD
// ============================================================
//
// 🎯 Stratégie :
//   - Tests de validation des requêtes (purs, sans dépendances)
//   - Tests des helpers d'entité CODProof
//   - Tests de cohérence et deadlines
//   - Les tests multi-tenant/transaction seront ajoutés plus tard
//
// ============================================================

// ============================================================
// TESTS : CollectCommissionRequest.Validate()
// ============================================================

func TestCollectCommissionRequest_Validate_Success(t *testing.T) {
	req := &codusecase.CollectCommissionRequest{
		OrderID:      "order-123",
		CollectedBy:  "merchant-123",
		ForceCollect: false,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestCollectCommissionRequest_Validate_EmptyOrderID(t *testing.T) {
	req := &codusecase.CollectCommissionRequest{
		OrderID:      "", // Vide
		CollectedBy:  "merchant-123",
		ForceCollect: false,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestCollectCommissionRequest_Validate_EmptyCollectedBy(t *testing.T) {
	req := &codusecase.CollectCommissionRequest{
		OrderID:      "order-123",
		CollectedBy:  "", // Vide
		ForceCollect: false,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collected_by is required")
}

func TestCollectCommissionRequest_Validate_WithForceCollect(t *testing.T) {
	req := &codusecase.CollectCommissionRequest{
		OrderID:      "order-123",
		CollectedBy:  "admin-123",
		ForceCollect: true, // Admin force la collecte
	}

	err := req.Validate()
	assert.NoError(t, err)
}

// ============================================================
// TESTS : SubmitClientProofRequest.Validate()
// ============================================================

func TestSubmitClientProofRequest_Validate_Success(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,                          // 500 FCFA
		PaymentDate: time.Now().Add(-1 * time.Hour), // 1 heure dans le passé
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestSubmitClientProofRequest_Validate_EmptyOrderID(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "", // Vide
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestSubmitClientProofRequest_Validate_EmptyCustomerID(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "", // Vide
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestSubmitClientProofRequest_Validate_EmptyProofURL(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "", // Vide
		AmountCents: 50000,
		PaymentDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proof_url is required")
}

func TestSubmitClientProofRequest_Validate_ZeroAmount(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 0, // Zéro
		PaymentDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitClientProofRequest_Validate_NegativeAmount(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: -50000, // Négatif
		PaymentDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitClientProofRequest_Validate_ZeroPaymentDate(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Time{}, // Zero value
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment_date is required")
}

func TestSubmitClientProofRequest_Validate_FuturePaymentDate(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now().Add(1 * time.Hour), // Dans le futur
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment_date cannot be in the future")
}

func TestSubmitClientProofRequest_Validate_WithOptionalFields(t *testing.T) {
	receiptNumber := "RECEIPT-123"
	notes := "Payment completed"

	req := &codusecase.SubmitClientProofRequest{
		OrderID:       "order-123",
		CustomerID:    "customer-123",
		ProofURL:      "https://example.com/proof.jpg",
		AmountCents:   50000,
		PaymentDate:   time.Now().Add(-1 * time.Hour),
		ReceiptNumber: &receiptNumber,
		Notes:         &notes,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

// ============================================================
// TESTS : SubmitMerchantProofRequest.Validate()
// ============================================================

func TestSubmitMerchantProofRequest_Validate_Success(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,                          // 500 FCFA
		ReceiptDate: time.Now().Add(-1 * time.Hour), // 1 heure dans le passé
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestSubmitMerchantProofRequest_Validate_EmptyOrderID(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "", // Vide
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestSubmitMerchantProofRequest_Validate_EmptyProofURL(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "", // Vide
		AmountCents: 50000,
		ReceiptDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proof_url is required")
}

func TestSubmitMerchantProofRequest_Validate_ZeroAmount(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 0, // Zéro
		ReceiptDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitMerchantProofRequest_Validate_NegativeAmount(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: -50000, // Négatif
		ReceiptDate: time.Now(),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestSubmitMerchantProofRequest_Validate_ZeroReceiptDate(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Time{}, // Zero value
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "receipt_date is required")
}

func TestSubmitMerchantProofRequest_Validate_FutureReceiptDate(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(1 * time.Hour), // Dans le futur
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "receipt_date cannot be in the future")
}

func TestSubmitMerchantProofRequest_Validate_WithNotes(t *testing.T) {
	notes := "Cash received"

	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(-1 * time.Hour),
		Notes:       &notes,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

// ============================================================
// TESTS : CODProof entity - NewCODProof()
// ============================================================

func TestNewCODProof_Success(t *testing.T) {
	proof, err := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	assert.NoError(t, err)
	assert.NotNil(t, proof)
	assert.Equal(t, "order-123", proof.OrderID)
	assert.Equal(t, "shop-123", proof.ShopID)
	assert.Equal(t, "customer-123", proof.CustomerID)
	assert.Equal(t, entity.CODProofPendingProofs, proof.Status)
	assert.Equal(t, entity.CODCommissionPending, proof.CommissionStatus)
	assert.Greater(t, proof.CommissionCents, int64(0)) // Commission calculée
}

func TestNewCODProof_EmptyOrderID(t *testing.T) {
	proof, err := entity.NewCODProof("", "shop-123", "customer-123", 50000)

	assert.Error(t, err)
	assert.Nil(t, proof)
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestNewCODProof_EmptyShopID(t *testing.T) {
	proof, err := entity.NewCODProof("order-123", "", "customer-123", 50000)

	assert.Error(t, err)
	assert.Nil(t, proof)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestNewCODProof_EmptyCustomerID(t *testing.T) {
	proof, err := entity.NewCODProof("order-123", "shop-123", "", 50000)

	assert.Error(t, err)
	assert.Nil(t, proof)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestNewCODProof_ZeroAmount(t *testing.T) {
	proof, err := entity.NewCODProof("order-123", "shop-123", "customer-123", 0)

	assert.Error(t, err)
	assert.Nil(t, proof)
	assert.Contains(t, err.Error(), "order_amount_cents must be positive")
}

func TestNewCODProof_NegativeAmount(t *testing.T) {
	proof, err := entity.NewCODProof("order-123", "shop-123", "customer-123", -50000)

	assert.Error(t, err)
	assert.Nil(t, proof)
	assert.Contains(t, err.Error(), "order_amount_cents must be positive")
}

// ============================================================
// TESTS : CODProof - SubmitClientProof()
// ============================================================

func TestCODProof_SubmitClientProof_Success(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.SubmitClientProof(
		"https://example.com/proof.jpg",
		50000,
		time.Now().Add(-1*time.Hour),
		"RECEIPT-123",
		"Payment completed",
	)

	assert.NoError(t, err)
	assert.True(t, proof.HasClientProof())
	assert.Equal(t, entity.CODProofClientProofSent, proof.Status)
	assert.NotNil(t, proof.ClientPaymentProofURL)
	assert.NotNil(t, proof.ClientPaymentAmountCents)
	assert.NotNil(t, proof.ClientPaymentDate)
	assert.NotNil(t, proof.ClientSubmittedAt)
}

func TestCODProof_SubmitClientProof_InvalidStatus(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.Status = entity.CODProofCompleted // Statut terminal

	err := proof.SubmitClientProof(
		"https://example.com/proof.jpg",
		50000,
		time.Now(),
		"",
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot submit client proof with status")
}

func TestCODProof_SubmitClientProof_EmptyProofURL(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.SubmitClientProof(
		"", // Vide
		50000,
		time.Now(),
		"",
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_payment_proof_url is required")
}

func TestCODProof_SubmitClientProof_ZeroAmount(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.SubmitClientProof(
		"https://example.com/proof.jpg",
		0, // Zéro
		time.Now(),
		"",
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_payment_amount_cents must be positive")
}

// ============================================================
// TESTS : CODProof - SubmitMerchantProof()
// ============================================================

func TestCODProof_SubmitMerchantProof_Success(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.SubmitMerchantProof(
		"https://example.com/receipt.jpg",
		50000,
		time.Now().Add(-1*time.Hour),
		"Cash received",
	)

	assert.NoError(t, err)
	assert.True(t, proof.HasMerchantProof())
	assert.Equal(t, entity.CODProofMerchantProofSent, proof.Status)
	assert.NotNil(t, proof.MerchantReceiptProofURL)
	assert.NotNil(t, proof.MerchantReceivedAmountCents)
	assert.NotNil(t, proof.MerchantReceiptDate)
	assert.NotNil(t, proof.MerchantSubmittedAt)
}

func TestCODProof_SubmitMerchantProof_InvalidStatus(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.Status = entity.CODProofCompleted // Statut terminal

	err := proof.SubmitMerchantProof(
		"https://example.com/receipt.jpg",
		50000,
		time.Now(),
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot submit merchant proof with status")
}

func TestCODProof_SubmitMerchantProof_EmptyProofURL(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.SubmitMerchantProof(
		"", // Vide
		50000,
		time.Now(),
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "merchant_receipt_proof_url is required")
}

func TestCODProof_SubmitMerchantProof_ZeroAmount(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.SubmitMerchantProof(
		"https://example.com/receipt.jpg",
		0, // Zéro
		time.Now(),
		"",
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "merchant_received_amount_cents must be positive")
}

// ============================================================
// TESTS : CODProof - Coherence
// ============================================================

func TestCODProof_IsCoherent_BothProofsMatching(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Soumettre preuve client
	proof.SubmitClientProof(
		"https://example.com/client.jpg",
		50000,
		paymentDate,
		"",
		"",
	)

	// Soumettre preuve marchand avec mêmes montants et dates
	proof.SubmitMerchantProof(
		"https://example.com/merchant.jpg",
		50000,
		paymentDate,
		"",
	)

	assert.True(t, proof.IsComplete())
	assert.True(t, proof.IsCoherent())
	assert.Equal(t, entity.CODProofConfirmed, proof.Status)
}

func TestCODProof_IsCoherent_AmountsMismatch(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Client dit avoir payé 50000
	proof.SubmitClientProof(
		"https://example.com/client.jpg",
		50000,
		paymentDate,
		"",
		"",
	)

	// Marchand dit avoir reçu 40000 (différence > tolérance)
	proof.SubmitMerchantProof(
		"https://example.com/merchant.jpg",
		40000,
		paymentDate,
		"",
	)

	assert.True(t, proof.IsComplete())
	assert.False(t, proof.IsCoherent())
	assert.Equal(t, entity.CODProofDisputed, proof.Status)
}

func TestCODProof_IsCoherent_AmountsWithinTolerance(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Client dit avoir payé 50000
	proof.SubmitClientProof(
		"https://example.com/client.jpg",
		50000,
		paymentDate,
		"",
		"",
	)

	// Marchand dit avoir reçu 49950 (différence = 50 < tolérance 100)
	proof.SubmitMerchantProof(
		"https://example.com/merchant.jpg",
		49950,
		paymentDate,
		"",
	)

	assert.True(t, proof.IsComplete())
	assert.True(t, proof.IsCoherent())
	assert.Equal(t, entity.CODProofConfirmed, proof.Status)
}

func TestCODProof_IsCoherent_Incomplete(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	// Seulement preuve client
	proof.SubmitClientProof(
		"https://example.com/client.jpg",
		50000,
		time.Now(),
		"",
		"",
	)

	assert.False(t, proof.IsComplete())
	assert.False(t, proof.IsCoherent())
}

// ============================================================
// TESTS : CODProof - Commission
// ============================================================

func TestCODProof_MarkCommissionCollected_Success(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Créer preuves cohérentes
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")

	assert.True(t, proof.IsCoherent())
	assert.Equal(t, entity.CODCommissionPending, proof.CommissionStatus)

	err := proof.MarkCommissionCollected()
	assert.NoError(t, err)
	assert.Equal(t, entity.CODCommissionCollected, proof.CommissionStatus)
	assert.Equal(t, entity.CODProofCompleted, proof.Status)
	assert.NotNil(t, proof.CommissionCollectedAt)
}

func TestCODProof_MarkCommissionCollected_NotCoherent(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Créer preuves incohérentes
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 40000, paymentDate, "")

	assert.False(t, proof.IsCoherent())

	err := proof.MarkCommissionCollected()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proofs must be coherent")
}

func TestCODProof_MarkCommissionCollected_AlreadyCollected(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.CommissionStatus = entity.CODCommissionCollected

	err := proof.MarkCommissionCollected()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot collect commission with status")
}

func TestCODProof_MarkCommissionDue_Success(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	assert.Equal(t, entity.CODCommissionPending, proof.CommissionStatus)

	err := proof.MarkCommissionDue()
	assert.NoError(t, err)
	assert.Equal(t, entity.CODCommissionDue, proof.CommissionStatus)
}

func TestCODProof_MarkCommissionDue_AlreadyDue(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.CommissionStatus = entity.CODCommissionDue

	err := proof.MarkCommissionDue()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot mark commission due with status")
}

func TestCODProof_CanCollectCommission(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Créer preuves cohérentes
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")

	assert.True(t, proof.CanCollectCommission())
}

// ============================================================
// TESTS : CODProof - Dispute
// ============================================================

func TestCODProof_RaiseDispute_Success(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.RaiseDispute("Amounts do not match")
	assert.NoError(t, err)
	assert.Equal(t, entity.CODProofDisputed, proof.Status)
	assert.NotNil(t, proof.DisputeRaisedAt)
	assert.NotNil(t, proof.DisputeReason)
	assert.Equal(t, "Amounts do not match", *proof.DisputeReason)
}

func TestCODProof_RaiseDispute_EmptyReason(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.RaiseDispute("") // Vide
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispute reason is required")
}

func TestCODProof_RaiseDispute_TerminalStatus(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.Status = entity.CODProofCompleted // Terminal

	err := proof.RaiseDispute("Reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot raise dispute with terminal status")
}

// ✅ CORRECTION : ResolveDispute(true) appelle MarkCommissionCollected()
// qui change le statut à "completed" (pas "resolved")
func TestCODProof_ResolveDispute_CollectCommission(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	paymentDate := time.Now().Add(-1 * time.Hour)

	// Créer preuves cohérentes
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")

	// Ouvrir litige
	proof.RaiseDispute("Dispute")
	assert.Equal(t, entity.CODProofDisputed, proof.Status)

	// Résoudre en collectant la commission
	err := proof.ResolveDispute(true)
	assert.NoError(t, err)
	// ✅ CORRECTION : Le statut final est "completed" car MarkCommissionCollected()
	// est appelé après avoir mis le statut à "resolved"
	assert.Equal(t, entity.CODProofCompleted, proof.Status)
	assert.Equal(t, entity.CODCommissionCollected, proof.CommissionStatus)
	assert.NotNil(t, proof.DisputeResolvedAt)
}

func TestCODProof_ResolveDispute_WaiveCommission(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.RaiseDispute("Dispute")

	// Résoudre en annulant la commission
	err := proof.ResolveDispute(false)
	assert.NoError(t, err)
	assert.Equal(t, entity.CODProofResolved, proof.Status)
	assert.Equal(t, entity.CODCommissionWaived, proof.CommissionStatus)
}

func TestCODProof_ResolveDispute_NotDisputed(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.Status = entity.CODProofPendingProofs

	err := proof.ResolveDispute(true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resolve dispute with status")
}

// ============================================================
// TESTS : CODProof - Deadline
// ============================================================

func TestCODProof_ProofDeadline(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	deadline := proof.ProofDeadline()
	expected := proof.CreatedAt.AddDate(0, 0, entity.CODProofDeadlineDays)

	assert.Equal(t, expected.Year(), deadline.Year())
	assert.Equal(t, expected.Month(), deadline.Month())
	assert.Equal(t, expected.Day(), deadline.Day())
}

func TestCODProof_IsPastDeadline_NotPast(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	// Nouveau proof n'est pas expiré
	assert.False(t, proof.IsPastDeadline())
}

func TestCODProof_IsPastDeadline_Past(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	// Simuler un proof créé il y a 10 jours
	proof.CreatedAt = time.Now().AddDate(0, 0, -10)

	assert.True(t, proof.IsPastDeadline())
}

func TestCODProof_DaysUntilDeadline(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	days := proof.DaysUntilDeadline()
	assert.GreaterOrEqual(t, days, 0)
	assert.LessOrEqual(t, days, entity.CODProofDeadlineDays)
}

func TestCODProof_DaysUntilDeadline_Past(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.CreatedAt = time.Now().AddDate(0, 0, -10)

	days := proof.DaysUntilDeadline()
	assert.Equal(t, 0, days)
}

// ============================================================
// TESTS : CODProof - Status helpers
// ============================================================

func TestCODProof_IsPending(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	assert.True(t, proof.IsPending())

	proof.SubmitClientProof("https://example.com/proof.jpg", 50000, time.Now(), "", "")
	assert.False(t, proof.IsPending())
}

func TestCODProof_IsComplete(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	assert.False(t, proof.IsComplete())

	proof.SubmitClientProof("https://example.com/client.jpg", 50000, time.Now(), "", "")
	assert.False(t, proof.IsComplete())

	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, time.Now(), "")
	assert.True(t, proof.IsComplete())
}

func TestCODProof_IsConfirmed(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	assert.False(t, proof.IsConfirmed())

	paymentDate := time.Now().Add(-1 * time.Hour)
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")

	assert.True(t, proof.IsConfirmed())
}

func TestCODProof_IsDisputed(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	assert.False(t, proof.IsDisputed())

	proof.RaiseDispute("Reason")
	assert.True(t, proof.IsDisputed())
}

func TestCODProof_IsCompleted(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	assert.False(t, proof.IsCompleted())

	paymentDate := time.Now().Add(-1 * time.Hour)
	proof.SubmitClientProof("https://example.com/client.jpg", 50000, paymentDate, "", "")
	proof.SubmitMerchantProof("https://example.com/merchant.jpg", 50000, paymentDate, "")
	proof.MarkCommissionCollected()

	assert.True(t, proof.IsCompleted())
}

func TestCODProof_IsResolved(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	assert.False(t, proof.IsResolved())

	proof.RaiseDispute("Reason")
	proof.ResolveDispute(false)

	assert.True(t, proof.IsResolved())
}

// ============================================================
// TESTS : CODProofStatus helpers
// ============================================================

func TestCODProofStatus_IsValid(t *testing.T) {
	assert.True(t, entity.CODProofPendingProofs.IsValid())
	assert.True(t, entity.CODProofClientProofSent.IsValid())
	assert.True(t, entity.CODProofMerchantProofSent.IsValid())
	assert.True(t, entity.CODProofConfirmed.IsValid())
	assert.True(t, entity.CODProofDisputed.IsValid())
	assert.True(t, entity.CODProofResolved.IsValid())
	assert.True(t, entity.CODProofCompleted.IsValid())

	assert.False(t, entity.CODProofStatus("invalid").IsValid())
}

func TestCODProofStatus_IsTerminal(t *testing.T) {
	assert.False(t, entity.CODProofPendingProofs.IsTerminal())
	assert.False(t, entity.CODProofClientProofSent.IsTerminal())
	assert.False(t, entity.CODProofMerchantProofSent.IsTerminal())
	assert.False(t, entity.CODProofConfirmed.IsTerminal())
	assert.False(t, entity.CODProofDisputed.IsTerminal())

	assert.True(t, entity.CODProofResolved.IsTerminal())
	assert.True(t, entity.CODProofCompleted.IsTerminal())
}

func TestCODCommissionStatus_IsValid(t *testing.T) {
	assert.True(t, entity.CODCommissionPending.IsValid())
	assert.True(t, entity.CODCommissionCollected.IsValid())
	assert.True(t, entity.CODCommissionDue.IsValid())
	assert.True(t, entity.CODCommissionWaived.IsValid())

	assert.False(t, entity.CODCommissionStatus("invalid").IsValid())
}

// ============================================================
// TESTS : CODProof - Validate()
// ============================================================

func TestCODProof_Validate_Success(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)

	err := proof.Validate()
	assert.NoError(t, err)
}

func TestCODProof_Validate_EmptyOrderID(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.OrderID = ""

	err := proof.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order_id is required")
}

func TestCODProof_Validate_EmptyShopID(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.ShopID = ""

	err := proof.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestCODProof_Validate_EmptyCustomerID(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.CustomerID = ""

	err := proof.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestCODProof_Validate_NegativeCommission(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.CommissionCents = -100

	err := proof.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "commission_cents cannot be negative")
}

func TestCODProof_Validate_InvalidStatus(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.Status = "invalid_status"

	err := proof.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestCODProof_Validate_InvalidCommissionStatus(t *testing.T) {
	proof, _ := entity.NewCODProof("order-123", "shop-123", "customer-123", 50000)
	proof.CommissionStatus = "invalid_status"

	err := proof.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid commission status")
}

// ============================================================
// TESTS : Performance
// ============================================================

func TestCollectCommissionRequest_Validate_Performance(t *testing.T) {
	req := &codusecase.CollectCommissionRequest{
		OrderID:     "order-123",
		CollectedBy: "merchant-123",
	}

	for i := 0; i < 10000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}

func TestSubmitClientProofRequest_Validate_Performance(t *testing.T) {
	req := &codusecase.SubmitClientProofRequest{
		OrderID:     "order-123",
		CustomerID:  "customer-123",
		ProofURL:    "https://example.com/proof.jpg",
		AmountCents: 50000,
		PaymentDate: time.Now().Add(-1 * time.Hour),
	}

	for i := 0; i < 10000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}

func TestSubmitMerchantProofRequest_Validate_Performance(t *testing.T) {
	req := &codusecase.SubmitMerchantProofRequest{
		OrderID:     "order-123",
		ProofURL:    "https://example.com/receipt.jpg",
		AmountCents: 50000,
		ReceiptDate: time.Now().Add(-1 * time.Hour),
	}

	for i := 0; i < 10000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}
