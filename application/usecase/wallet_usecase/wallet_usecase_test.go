package walletusecase_test

import (
	"testing"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 🆕 v4.4.4 : TESTS UNITAIRES - USECASES WALLET
// ============================================================
//
// 🎯 Stratégie :
//   - Tests de validation (purs, sans dépendances)
//   - Tests des helpers d'entités (MerchantWallet, AccountFreeze)
//   - Tests de performance
//   - Les tests multi-tenant/transaction seront ajoutés plus tard
//
// ============================================================

// ============================================================
// TESTS : CreditWalletRequest.Validate()
// ============================================================

func TestCreditWalletRequest_Validate_Success(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000, // 100 FCFA
		TransactionType: entity.WalletTxSaleCredit,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestCreditWalletRequest_Validate_EmptyShopID(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "", // Vide
		AmountCents:     10000,
		TransactionType: entity.WalletTxSaleCredit,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestCreditWalletRequest_Validate_ZeroAmount(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     0, // Zéro
		TransactionType: entity.WalletTxSaleCredit,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestCreditWalletRequest_Validate_NegativeAmount(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     -10000, // Négatif
		TransactionType: entity.WalletTxSaleCredit,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestCreditWalletRequest_Validate_InvalidTransactionType(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: "invalid_type",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction type")
}

func TestCreditWalletRequest_Validate_DebitTypeInsteadOfCredit(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: entity.WalletTxCommissionDebit, // Débit au lieu de crédit
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a credit type")
}

func TestCreditWalletRequest_Validate_AllCreditTypes(t *testing.T) {
	creditTypes := []entity.WalletTransactionType{
		entity.WalletTxSaleCredit,
		entity.WalletTxCOD,
		entity.WalletTxSaleTontine,
		entity.WalletTxSaleCreditPlan,
		entity.WalletTxDeposit,
		entity.WalletTxUnfreezeDeposit,
	}

	for _, txType := range creditTypes {
		req := &walletusecase.CreditWalletRequest{
			ShopID:          "shop-123",
			AmountCents:     10000,
			TransactionType: txType,
		}

		err := req.Validate()
		assert.NoError(t, err, "Transaction type %s should be valid for credit", txType)
	}
}

// ============================================================
// TESTS : DebitWalletRequest.Validate()
// ============================================================

func TestDebitWalletRequest_Validate_Success(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000, // 100 FCFA
		TransactionType: entity.WalletTxCommissionDebit,
		AllowNegative:   true,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestDebitWalletRequest_Validate_EmptyShopID(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "", // Vide
		AmountCents:     10000,
		TransactionType: entity.WalletTxCommissionDebit,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestDebitWalletRequest_Validate_ZeroAmount(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     0, // Zéro
		TransactionType: entity.WalletTxCommissionDebit,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestDebitWalletRequest_Validate_NegativeAmount(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     -10000, // Négatif
		TransactionType: entity.WalletTxCommissionDebit,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_cents must be positive")
}

func TestDebitWalletRequest_Validate_InvalidTransactionType(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: "invalid_type",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction type")
}

func TestDebitWalletRequest_Validate_CreditTypeInsteadOfDebit(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: entity.WalletTxSaleCredit, // Crédit au lieu de débit
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a debit type")
}

func TestDebitWalletRequest_Validate_AllDebitTypes(t *testing.T) {
	debitTypes := []entity.WalletTransactionType{
		entity.WalletTxCommissionDebit,
		entity.WalletTxRefund,
		entity.WalletTxPayout,
		entity.WalletTxFreezePenalty,
	}

	for _, txType := range debitTypes {
		req := &walletusecase.DebitWalletRequest{
			ShopID:          "shop-123",
			AmountCents:     10000,
			TransactionType: txType,
		}

		err := req.Validate()
		assert.NoError(t, err, "Transaction type %s should be valid for debit", txType)
	}
}

// ============================================================
// TESTS : FreezeAccountRequest.Validate()
// ============================================================

func TestFreezeAccountRequest_Validate_Success(t *testing.T) {
	req := &walletusecase.FreezeAccountRequest{
		ShopID:         "shop-123",
		Reason:         entity.FreezeReasonNegativeBalance,
		AmountDueCents: 50000, // 500 FCFA
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestFreezeAccountRequest_Validate_EmptyShopID(t *testing.T) {
	req := &walletusecase.FreezeAccountRequest{
		ShopID:         "", // Vide
		Reason:         entity.FreezeReasonNegativeBalance,
		AmountDueCents: 50000,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestFreezeAccountRequest_Validate_InvalidReason(t *testing.T) {
	req := &walletusecase.FreezeAccountRequest{
		ShopID:         "shop-123",
		Reason:         "invalid_reason",
		AmountDueCents: 50000,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid freeze reason")
}

func TestFreezeAccountRequest_Validate_ZeroAmountDue(t *testing.T) {
	req := &walletusecase.FreezeAccountRequest{
		ShopID:         "shop-123",
		Reason:         entity.FreezeReasonNegativeBalance,
		AmountDueCents: 0, // Zéro
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_due_cents must be positive")
}

func TestFreezeAccountRequest_Validate_NegativeAmountDue(t *testing.T) {
	req := &walletusecase.FreezeAccountRequest{
		ShopID:         "shop-123",
		Reason:         entity.FreezeReasonNegativeBalance,
		AmountDueCents: -50000, // Négatif
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amount_due_cents must be positive")
}

func TestFreezeAccountRequest_Validate_ZeroGracePeriod(t *testing.T) {
	gracePeriod := 0
	req := &walletusecase.FreezeAccountRequest{
		ShopID:          "shop-123",
		Reason:          entity.FreezeReasonNegativeBalance,
		AmountDueCents:  50000,
		GracePeriodDays: &gracePeriod, // Zéro
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grace_period_days must be positive")
}

func TestFreezeAccountRequest_Validate_NegativeGracePeriod(t *testing.T) {
	gracePeriod := -7
	req := &walletusecase.FreezeAccountRequest{
		ShopID:          "shop-123",
		Reason:          entity.FreezeReasonNegativeBalance,
		AmountDueCents:  50000,
		GracePeriodDays: &gracePeriod, // Négatif
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grace_period_days must be positive")
}

func TestFreezeAccountRequest_Validate_PositiveGracePeriod(t *testing.T) {
	gracePeriod := 14
	req := &walletusecase.FreezeAccountRequest{
		ShopID:          "shop-123",
		Reason:          entity.FreezeReasonNegativeBalance,
		AmountDueCents:  50000,
		GracePeriodDays: &gracePeriod, // 14 jours
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestFreezeAccountRequest_Validate_AllReasons(t *testing.T) {
	reasons := []entity.FreezeReason{
		entity.FreezeReasonNegativeBalance,
		entity.FreezeReasonUnpaidCommission,
		entity.FreezeReasonFraudSuspected,
		entity.FreezeReasonAdminDecision,
	}

	for _, reason := range reasons {
		req := &walletusecase.FreezeAccountRequest{
			ShopID:         "shop-123",
			Reason:         reason,
			AmountDueCents: 50000,
		}

		err := req.Validate()
		assert.NoError(t, err, "Reason %s should be valid", reason)
	}
}

// ============================================================
// TESTS : UnfreezeAccountRequest.Validate()
// ============================================================

func TestUnfreezeAccountRequest_Validate_Success(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "shop-123",
		DepositAmountCents: 50000,
		Resolution:         entity.FreezeResolutionPaid,
		ResolvedBy:         "user-123",
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestUnfreezeAccountRequest_Validate_EmptyShopID(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "", // Vide
		DepositAmountCents: 50000,
		Resolution:         entity.FreezeResolutionPaid,
		ResolvedBy:         "user-123",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shop_id is required")
}

func TestUnfreezeAccountRequest_Validate_EmptyResolvedBy(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "shop-123",
		DepositAmountCents: 50000,
		Resolution:         entity.FreezeResolutionPaid,
		ResolvedBy:         "", // Vide
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolved_by is required")
}

func TestUnfreezeAccountRequest_Validate_InvalidResolution(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "shop-123",
		DepositAmountCents: 50000,
		Resolution:         "invalid_resolution",
		ResolvedBy:         "user-123",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid resolution")
}

func TestUnfreezeAccountRequest_Validate_ZeroDepositWithPaid(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "shop-123",
		DepositAmountCents: 0, // Zéro avec résolution "paid"
		Resolution:         entity.FreezeResolutionPaid,
		ResolvedBy:         "user-123",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deposit_amount_cents must be positive")
}

func TestUnfreezeAccountRequest_Validate_NegativeDepositWithPaid(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "shop-123",
		DepositAmountCents: -50000, // Négatif
		Resolution:         entity.FreezeResolutionPaid,
		ResolvedBy:         "user-123",
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deposit_amount_cents must be positive")
}

// ============================================================
// TESTS : MerchantWallet helpers
// ============================================================

func TestMerchantWallet_Credit_Success(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	initialBalance := wallet.BalanceCents

	err := wallet.Credit(10000)
	assert.NoError(t, err)
	assert.Equal(t, initialBalance+10000, wallet.BalanceCents)
}

func TestMerchantWallet_Credit_NegativeAmount(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")

	err := wallet.Credit(-10000)
	assert.Error(t, err)
}

func TestMerchantWallet_Credit_ZeroAmount(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")

	err := wallet.Credit(0)
	assert.Error(t, err)
}

func TestMerchantWallet_Debit_Success(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	wallet.Credit(50000) // Créditer d'abord
	previousBalance := wallet.BalanceCents

	err := wallet.Debit(10000)
	assert.NoError(t, err)
	assert.Equal(t, previousBalance-10000, wallet.BalanceCents)
}

// ✅ CORRECTION : Le débit autorise le solde négatif (comportement voulu)
func TestMerchantWallet_Debit_AllowsNegativeBalance(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	wallet.Credit(10000) // Créditer 100 FCFA

	// Le débit est autorisé même si le solde devient négatif
	// (comportement voulu pour les dettes de commission)
	err := wallet.Debit(20000) // Débite 200 FCFA
	assert.NoError(t, err, "Debit should allow negative balance")
	assert.Equal(t, int64(-10000), wallet.BalanceCents, "Balance should be -100 FCFA")
}

// ✅ NOUVEAU : Test de dépassement de la limite négative maximale
func TestMerchantWallet_Debit_ExceedsMaxNegative(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	wallet.Credit(10000) // Créditer 100 FCFA

	// Tenter de dépasser la limite négative maximale
	err := wallet.Debit(1000000000000) // Montant énorme
	assert.Error(t, err, "Debit should fail when exceeding max negative balance")
}

func TestMerchantWallet_Debit_NegativeAmount(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	wallet.Credit(50000)

	err := wallet.Debit(-10000)
	assert.Error(t, err)
}

func TestMerchantWallet_Freeze_Success(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	assert.False(t, wallet.IsFrozen)

	details := "Test freeze"
	err := wallet.Freeze(entity.FreezeReasonNegativeBalance, details)
	assert.NoError(t, err)
	assert.True(t, wallet.IsFrozen)
}

func TestMerchantWallet_Freeze_AlreadyFrozen(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	details := "Test freeze"
	wallet.Freeze(entity.FreezeReasonNegativeBalance, details)

	// Tenter de geler à nouveau
	err := wallet.Freeze(entity.FreezeReasonFraudSuspected, "Another freeze")
	assert.Error(t, err)
}

func TestMerchantWallet_ForceUnfreeze_Success(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	details := "Test freeze"
	wallet.Freeze(entity.FreezeReasonNegativeBalance, details)
	assert.True(t, wallet.IsFrozen)

	err := wallet.ForceUnfreeze()
	assert.NoError(t, err)
	assert.False(t, wallet.IsFrozen)
}

func TestMerchantWallet_ForceUnfreeze_NotFrozen(t *testing.T) {
	wallet := entity.NewMerchantWallet("shop-123")
	assert.False(t, wallet.IsFrozen)

	err := wallet.ForceUnfreeze()
	assert.Error(t, err)
}

// ============================================================
// TESTS : AccountFreeze helpers
// ============================================================

func TestNewAccountFreeze_Success(t *testing.T) {
	details := "Test freeze details"
	freeze, err := entity.NewAccountFreeze(
		"shop-123",
		entity.FreezeReasonNegativeBalance,
		50000,
		&details,
	)

	assert.NoError(t, err)
	assert.NotNil(t, freeze)
	assert.Equal(t, "shop-123", freeze.ShopID)
	assert.Equal(t, int64(50000), freeze.AmountDueCents)
	assert.False(t, freeze.IsResolved())
}

func TestAccountFreeze_DaysUntilExpiration(t *testing.T) {
	details := "Test freeze"
	freeze, _ := entity.NewAccountFreeze("shop-123", entity.FreezeReasonNegativeBalance, 50000, &details)

	days := freeze.DaysUntilExpiration()
	assert.True(t, days > 0, "Days until expiration should be positive")
	assert.True(t, days <= entity.DefaultGracePeriodDays, "Should not exceed default grace period")
}

func TestAccountFreeze_IsExpired_NotExpired(t *testing.T) {
	details := "Test freeze"
	freeze, _ := entity.NewAccountFreeze("shop-123", entity.FreezeReasonNegativeBalance, 50000, &details)

	assert.False(t, freeze.IsExpired(), "New freeze should not be expired")
}

// ============================================================
// TESTS : Edge cases
// ============================================================

func TestCreditWalletRequest_Validate_LargeAmount(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     1000000000, // 10 millions FCFA
		TransactionType: entity.WalletTxDeposit,
	}

	err := req.Validate()
	assert.NoError(t, err, "Large amount should be valid")
}

func TestDebitWalletRequest_Validate_AllowNegativeTrue(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: entity.WalletTxCommissionDebit,
		AllowNegative:   true,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestDebitWalletRequest_Validate_AllowNegativeFalse(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: entity.WalletTxCommissionDebit,
		AllowNegative:   false,
	}

	err := req.Validate()
	assert.NoError(t, err)
}

// ============================================================
// TESTS : Performance
// ============================================================

func TestCreditWalletRequest_Validate_Performance(t *testing.T) {
	req := &walletusecase.CreditWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: entity.WalletTxSaleCredit,
	}

	// Valider 1000 fois
	for i := 0; i < 1000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}

func TestDebitWalletRequest_Validate_Performance(t *testing.T) {
	req := &walletusecase.DebitWalletRequest{
		ShopID:          "shop-123",
		AmountCents:     10000,
		TransactionType: entity.WalletTxCommissionDebit,
	}

	// Valider 1000 fois
	for i := 0; i < 1000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}

func TestFreezeAccountRequest_Validate_Performance(t *testing.T) {
	req := &walletusecase.FreezeAccountRequest{
		ShopID:         "shop-123",
		Reason:         entity.FreezeReasonNegativeBalance,
		AmountDueCents: 50000,
	}

	// Valider 1000 fois
	for i := 0; i < 1000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}

func TestUnfreezeAccountRequest_Validate_Performance(t *testing.T) {
	req := &walletusecase.UnfreezeAccountRequest{
		ShopID:             "shop-123",
		DepositAmountCents: 50000,
		Resolution:         entity.FreezeResolutionPaid,
		ResolvedBy:         "user-123",
	}

	// Valider 1000 fois
	for i := 0; i < 1000; i++ {
		err := req.Validate()
		assert.NoError(t, err)
	}
}
