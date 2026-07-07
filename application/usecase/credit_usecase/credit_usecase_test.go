package creditusecase_test

import (
	"testing"

	creditusecase "Goshop/application/usecase/credit_usecase"
	"Goshop/domain/entity"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.4 : TESTS UNITAIRES - USECASES CREDIT
// ============================================================

// ============================================================
// TESTS : ConfigureCreditPlanRequest.Validate()
// ============================================================

func TestConfigureCreditPlanRequest_Validate_Success(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		IsEnabled:             true,
		MinDownPaymentPercent: entity.MinDownPaymentPercent, // Valeur limite valide
		MaxDurationMonths:     entity.MinDurationMonths,     // Valeur limite valide
		InterestRateBps:       entity.MinInterestRateBps,    // Valeur limite valide
		PenaltyRateBps:        entity.MinPenaltyRateBps,     // Valeur limite valide
		MinCreditScore:        entity.MinCreditScore,        // Valeur limite valide
	}

	err := req.Validate()
	assert.NoError(t, err, "Valeurs limites minimales doivent être valides")
}

func TestConfigureCreditPlanRequest_Validate_Success_Max(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		IsEnabled:             true,
		MinDownPaymentPercent: entity.MaxDownPaymentPercent, // Valeur limite valide
		MaxDurationMonths:     entity.MaxDurationMonths,     // Valeur limite valide
		InterestRateBps:       entity.MaxInterestRateBps,    // Valeur limite valide
		PenaltyRateBps:        entity.MaxPenaltyRateBps,     // Valeur limite valide
		MinCreditScore:        entity.MaxCreditScore,        // Valeur limite valide
	}

	err := req.Validate()
	assert.NoError(t, err, "Valeurs limites maximales doivent être valides")
}

func TestConfigureCreditPlanRequest_Validate_EmptyProductID(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "", // Vide
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product_id is required")
}

func TestConfigureCreditPlanRequest_Validate_DownPayment_TooLow(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent - 1, // Sous la limite
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_down_payment_percent")
}

func TestConfigureCreditPlanRequest_Validate_DownPayment_TooHigh(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MaxDownPaymentPercent + 1, // Au-dessus de la limite
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_down_payment_percent")
}

func TestConfigureCreditPlanRequest_Validate_Duration_TooShort(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths - 1, // Sous la limite
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_duration_months")
}

func TestConfigureCreditPlanRequest_Validate_Duration_TooLong(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MaxDurationMonths + 1, // Au-dessus de la limite
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_duration_months")
}

func TestConfigureCreditPlanRequest_Validate_InterestRate_TooLow(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps - 1, // Sous la limite
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interest_rate_bps")
}

func TestConfigureCreditPlanRequest_Validate_InterestRate_TooHigh(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MaxInterestRateBps + 1, // Au-dessus de la limite
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interest_rate_bps")
}

func TestConfigureCreditPlanRequest_Validate_PenaltyRate_TooLow(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps - 1, // Sous la limite
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "penalty_rate_bps")
}

func TestConfigureCreditPlanRequest_Validate_PenaltyRate_TooHigh(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MaxPenaltyRateBps + 1, // Au-dessus de la limite
		MinCreditScore:        entity.MinCreditScore,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "penalty_rate_bps")
}

func TestConfigureCreditPlanRequest_Validate_CreditScore_TooLow(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore - 1, // Sous la limite
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_credit_score")
}

func TestConfigureCreditPlanRequest_Validate_CreditScore_TooHigh(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MaxCreditScore + 1, // Au-dessus de la limite
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_credit_score")
}

// ============================================================
// TESTS : ApplyForCreditRequest.Validate()
// ============================================================

func TestApplyForCreditRequest_Validate_Success(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MinDurationMonths,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestApplyForCreditRequest_Validate_EmptyCustomerID(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "", // Vide
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MinDurationMonths,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "customer_id is required")
}

func TestApplyForCreditRequest_Validate_EmptyProductID(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "", // Vide
		RequestedDurationMonths: entity.MinDurationMonths,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product_id is required")
}

func TestApplyForCreditRequest_Validate_Duration_TooShort(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MinDurationMonths - 1, // Sous la limite
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requested_duration_months")
}

func TestApplyForCreditRequest_Validate_Duration_TooLong(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MaxDurationMonths + 1, // Au-dessus de la limite
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requested_duration_months")
}

func TestApplyForCreditRequest_Validate_MinDuration(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MinDurationMonths,
	}

	err := req.Validate()
	assert.NoError(t, err, "Min duration should be valid")
}

func TestApplyForCreditRequest_Validate_MaxDuration(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MaxDurationMonths,
	}

	err := req.Validate()
	assert.NoError(t, err, "Max duration should be valid")
}

// ============================================================
// TESTS : ApproveCreditRequest.Validate()
// ============================================================

func TestApproveCreditRequest_Validate_Success(t *testing.T) {
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-123",
		ReviewedBy:    "merchant-123",
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestApproveCreditRequest_Validate_EmptyApplicationID(t *testing.T) {
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "", // Vide
		ReviewedBy:    "merchant-123",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "application_id is required")
}

func TestApproveCreditRequest_Validate_EmptyReviewedBy(t *testing.T) {
	req := &creditusecase.ApproveCreditRequest{
		ApplicationID: "app-123",
		ReviewedBy:    "", // Vide
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reviewed_by is required")
}

// ============================================================
// TESTS : RejectCreditRequest.Validate()
// ============================================================

func TestRejectCreditRequest_Validate_Success(t *testing.T) {
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "app-123",
		ReviewedBy:      "merchant-123",
		RejectionReason: "Credit score too low",
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestRejectCreditRequest_Validate_EmptyApplicationID(t *testing.T) {
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "", // Vide
		ReviewedBy:      "merchant-123",
		RejectionReason: "Credit score too low",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "application_id is required")
}

func TestRejectCreditRequest_Validate_EmptyReviewedBy(t *testing.T) {
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "app-123",
		ReviewedBy:      "", // Vide
		RejectionReason: "Credit score too low",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reviewed_by is required")
}

func TestRejectCreditRequest_Validate_EmptyRejectionReason(t *testing.T) {
	req := &creditusecase.RejectCreditRequest{
		ApplicationID:   "app-123",
		ReviewedBy:      "merchant-123",
		RejectionReason: "", // Vide
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rejection_reason is required")
}

// ============================================================
// TESTS : PayDownPaymentRequest.Validate()
// ============================================================

func TestPayDownPaymentRequest_Validate_Success(t *testing.T) {
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "contract-123",
		PaymentID:  "payment-123",
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestPayDownPaymentRequest_Validate_EmptyContractID(t *testing.T) {
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "", // Vide
		PaymentID:  "payment-123",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contract_id is required")
}

func TestPayDownPaymentRequest_Validate_EmptyPaymentID(t *testing.T) {
	req := &creditusecase.PayDownPaymentRequest{
		ContractID: "contract-123",
		PaymentID:  "", // Vide
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment_id is required")
}

// ============================================================
// TESTS : CreditScore helpers
// ============================================================

func TestCreditScore_MeetsMinRequirement_Above(t *testing.T) {
	score := &entity.CreditScore{
		Score: 650,
	}

	assert.True(t, score.MeetsMinRequirement(600), "650 >= 600")
}

func TestCreditScore_MeetsMinRequirement_Equal(t *testing.T) {
	score := &entity.CreditScore{
		Score: 650,
	}

	assert.True(t, score.MeetsMinRequirement(650), "650 >= 650")
}

func TestCreditScore_MeetsMinRequirement_Below(t *testing.T) {
	score := &entity.CreditScore{
		Score: 650,
	}

	assert.False(t, score.MeetsMinRequirement(700), "650 < 700")
}

func TestCreditScore_RecordNewContract(t *testing.T) {
	score := &entity.CreditScore{
		Score:          650,
		TotalContracts: 5,
	}

	score.RecordNewContract()
	assert.Equal(t, 6, score.TotalContracts)
}

func TestCreditScore_RecordNewContract_FromZero(t *testing.T) {
	score := &entity.CreditScore{
		Score:          500,
		TotalContracts: 0,
	}

	score.RecordNewContract()
	assert.Equal(t, 1, score.TotalContracts)
}

// ============================================================
// TESTS : CreditApplication helpers
// ============================================================

func TestCreditApplication_Status_Pending(t *testing.T) {
	app := &entity.CreditApplication{
		Status: entity.CreditApplicationPending,
	}

	assert.Equal(t, entity.CreditApplicationPending, app.Status)
}

func TestCreditApplication_Status_Approved(t *testing.T) {
	app := &entity.CreditApplication{
		Status: entity.CreditApplicationApproved,
	}

	assert.Equal(t, entity.CreditApplicationApproved, app.Status)
}

func TestCreditApplication_Status_Rejected(t *testing.T) {
	app := &entity.CreditApplication{
		Status: entity.CreditApplicationRejected,
	}

	assert.Equal(t, entity.CreditApplicationRejected, app.Status)
}

// ============================================================
// TESTS : CreditContract helpers
// ============================================================

func TestCreditContract_Status_Active(t *testing.T) {
	contract := &entity.CreditContract{
		Status: entity.CreditContractActive,
	}

	assert.Equal(t, entity.CreditContractActive, contract.Status)
}

func TestCreditContract_Status_Completed(t *testing.T) {
	contract := &entity.CreditContract{
		Status: entity.CreditContractCompleted,
	}

	assert.Equal(t, entity.CreditContractCompleted, contract.Status)
}

// ============================================================
// TESTS : CreditPlan helpers
// ============================================================

func TestCreditPlan_InterestRatePercent_1500(t *testing.T) {
	plan := &entity.CreditPlan{
		InterestRateBps: 1500, // 15%
	}

	assert.Equal(t, 15.0, plan.InterestRatePercent())
}

func TestCreditPlan_InterestRatePercent_500(t *testing.T) {
	plan := &entity.CreditPlan{
		InterestRateBps: 500, // 5%
	}

	assert.Equal(t, 5.0, plan.InterestRatePercent())
}

func TestCreditPlan_InterestRatePercent_100(t *testing.T) {
	plan := &entity.CreditPlan{
		InterestRateBps: 100, // 1%
	}

	assert.Equal(t, 1.0, plan.InterestRatePercent())
}

func TestCreditPlan_Validate_Success(t *testing.T) {
	plan := &entity.CreditPlan{
		ProductID:             "product-123",
		ShopID:                "shop-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	err := plan.Validate()
	assert.NoError(t, err)
}

// ============================================================
// TESTS : Performance
// ============================================================

func TestConfigureCreditPlanRequest_Validate_Performance(t *testing.T) {
	req := &creditusecase.ConfigureCreditPlanRequest{
		ProductID:             "product-123",
		MinDownPaymentPercent: entity.MinDownPaymentPercent,
		MaxDurationMonths:     entity.MinDurationMonths,
		InterestRateBps:       entity.MinInterestRateBps,
		PenaltyRateBps:        entity.MinPenaltyRateBps,
		MinCreditScore:        entity.MinCreditScore,
	}

	// Valider 1000 fois
	for i := 0; i < 1000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}

func TestApplyForCreditRequest_Validate_Performance(t *testing.T) {
	req := &creditusecase.ApplyForCreditRequest{
		CustomerID:              "customer-123",
		ProductID:               "product-123",
		RequestedDurationMonths: entity.MinDurationMonths,
	}

	// Valider 1000 fois
	for i := 0; i < 1000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}
