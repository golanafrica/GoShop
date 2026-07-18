package withdrawalusecase

import (
	"context"
	"fmt"
	"os"

	withdrawaldto "Goshop/application/dto/withdrawal_dto"
	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/rs/zerolog"
)

type CreateWithdrawalUsecase struct {
	withdrawalRepo  repository.WithdrawalRepository
	shopPaymentRepo ShopPaymentSettingsRepository
	registry        PaymentRegistry
	yengaPayFactory YengaPayProviderFactory
	debitWalletUC   *walletusecase.DebitWalletUsecase // ✅ AJOUT : Pour débiter le wallet avant le retrait
}

func NewCreateWithdrawalUsecase(
	withdrawalRepo repository.WithdrawalRepository,
	shopPaymentRepo ShopPaymentSettingsRepository,
	registry PaymentRegistry,
	debitWalletUC *walletusecase.DebitWalletUsecase, // ✅ AJOUT
) *CreateWithdrawalUsecase {
	return &CreateWithdrawalUsecase{
		withdrawalRepo:  withdrawalRepo,
		shopPaymentRepo: shopPaymentRepo,
		registry:        registry,
		yengaPayFactory: defaultYengaPayFactory,
		debitWalletUC:   debitWalletUC, // ✅ AJOUT
	}
}

// defaultYengaPayFactory est la factory par défaut qui crée un vrai provider
func defaultYengaPayFactory(config payment.YengaPayConfig) (CashOutProvider, error) {
	return payment.NewYengaPayProvider(config)
}

// NewCreateWithdrawalUsecaseWithFactory crée une instance avec factory personnalisée (pour tests)
func NewCreateWithdrawalUsecaseWithFactory(
	withdrawalRepo repository.WithdrawalRepository,
	shopPaymentRepo ShopPaymentSettingsRepository,
	registry PaymentRegistry,
	yengaPayFactory YengaPayProviderFactory,
	debitWalletUC *walletusecase.DebitWalletUsecase, // ✅ AJOUT
) *CreateWithdrawalUsecase {
	return &CreateWithdrawalUsecase{
		withdrawalRepo:  withdrawalRepo,
		shopPaymentRepo: shopPaymentRepo,
		registry:        registry,
		yengaPayFactory: yengaPayFactory,
		debitWalletUC:   debitWalletUC, // ✅ AJOUT
	}
}

func (uc *CreateWithdrawalUsecase) Execute(ctx context.Context, req *withdrawaldto.CreateWithdrawalRequest) (*withdrawaldto.WithdrawalResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	logger.Info().
		Str("shop_id", shop.ID.String()).
		Int64("amount_cents", req.AmountCents).
		Str("payment_method", req.PaymentMethod).
		Msg("Creating withdrawal")

	// ============================================================
	// 🆕 v4.1.0 : Vérification KYC pour les retraits
	// ============================================================
	if !shop.CanWithdraw() {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Str("shop_name", shop.Name).
			Str("kyc_status", string(shop.KYCStatus)).
			Int64("amount_cents", req.AmountCents).
			Msg("❌ Retrait bloqué : KYC non vérifié")

		var message string
		switch shop.KYCStatus {
		case entity.ShopKYCStatusUnverified:
			message = "Merchant KYC verification required for withdrawals. Please submit your identity document and business registry to unlock withdrawals."
		case entity.ShopKYCStatusPending:
			message = "Your KYC documents are currently under review. Withdrawals will be available once verification is complete."
		case entity.ShopKYCStatusRejected:
			if shop.KYCRejectionReason != nil {
				message = fmt.Sprintf("Your KYC was rejected: %s. Please resubmit corrected documents.", *shop.KYCRejectionReason)
			} else {
				message = "Your KYC was rejected. Please resubmit corrected documents to unlock withdrawals."
			}
		default:
			message = "Merchant KYC verification required for withdrawals."
		}

		return nil, fmt.Errorf("%s (current status: %s)", message, shop.KYCStatus)
	}

	logger.Debug().
		Str("shop_id", shop.ID.String()).
		Str("kyc_status", string(shop.KYCStatus)).
		Msg("✅ KYC vérifié, retrait autorisé")

	// ============================================================
	// ✅ 2. NOUVEAU : Débiter le wallet AVANT d'appeler l'API externe
	// Cela garantit que le marchand a les fonds et évite les découverts non gérés.
	// ============================================================
	debitReq := &walletusecase.DebitWalletRequest{
		ShopID:          shop.ID.String(),
		AmountCents:     req.AmountCents,
		TransactionType: entity.WalletTxPayout, // ✅ Le nom exact de la constante dans ton code
		AllowNegative:   false,                 // ✅ Interdire le négatif pour un retrait
	}

	_, err = uc.debitWalletUC.Execute(ctx, debitReq)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to debit wallet for withdrawal")
		return nil, fmt.Errorf("insufficient funds or wallet error: %w", err)
	}

	// 3. Créer l'entité Withdrawal
	withdrawal, err := entity.NewWithdrawal(
		shop.ID,
		req.AmountCents,
		entity.WithdrawalPaymentMethod(req.PaymentMethod),
		req.DestinationNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("create withdrawal entity: %w", err)
	}

	if req.DestinationName != "" {
		withdrawal.DestinationName = &req.DestinationName
	}
	if req.DestinationEmail != "" {
		withdrawal.DestinationEmail = &req.DestinationEmail
	}
	if req.Description != "" {
		withdrawal.Description = &req.Description
	}

	// 4. Sauvegarder en statut PENDING
	if err := uc.withdrawalRepo.Create(ctx, withdrawal); err != nil {
		return nil, fmt.Errorf("save withdrawal: %w", err)
	}

	// 5. Récupérer la config Yenga Pay de la boutique (fallback hybride)
	var providerConfig payment.YengaPayConfig

	settings, err := uc.shopPaymentRepo.GetPaymentSettings(ctx, shop.ID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get shop settings, using global config")
	} else if settings.YengaPay.Enabled && settings.YengaPay.APIKey != "" {
		providerConfig = payment.YengaPayConfig{
			APIKey:         settings.YengaPay.APIKey,
			OrganizationID: settings.YengaPay.OrganizationID,
			ProjectID:      settings.YengaPay.ProjectID,
			WebhookSecret:  settings.YengaPay.WebhookSecret,
			Env:            settings.YengaPay.Env,
		}
		logger.Info().Msg("Using shop-specific Yenga Pay configuration for cash-out")
	} else {
		// Fallback sur config globale
		providerConfig = payment.YengaPayConfig{
			APIKey:         getEnvOrDefault("YENGA_PAY_API_KEY", ""),
			OrganizationID: getEnvOrDefault("YENGA_PAY_ORGANIZATION_ID", ""),
			ProjectID:      getEnvOrDefault("YENGA_PAY_PROJECT_ID", ""),
			WebhookSecret:  getEnvOrDefault("YENGA_PAY_WEBHOOK_SECRET", ""),
			Env:            getEnvOrDefault("YENGA_PAY_ENV", "test"),
		}
		logger.Info().Msg("Using global Yenga Pay configuration for cash-out")
	}

	// 6. Créer un provider Yenga Pay avec la config
	yengaProvider, err := uc.yengaPayFactory(providerConfig)
	if err != nil {
		// ✅ CORRECTION : Marquer le retrait comme échoué si la création du provider échoue
		if markErr := withdrawal.MarkFailed(err.Error()); markErr == nil {
			_ = uc.withdrawalRepo.Update(ctx, withdrawal)
		}
		return nil, fmt.Errorf("create yenga provider: %w", err)
	}

	// 7. Appeler l'API cash-out
	cashOutReq := &payment.CashOutRequest{
		AmountCents:       req.AmountCents,
		PaymentMethod:     req.PaymentMethod,
		DestinationNumber: req.DestinationNumber,
		DestinationName:   req.DestinationName,
		DestinationEmail:  req.DestinationEmail,
		Description:       req.Description,
	}

	cashOutResp, err := yengaProvider.CashOut(ctx, cashOutReq)
	if err != nil {
		if markErr := withdrawal.MarkFailed(err.Error()); markErr == nil {
			_ = uc.withdrawalRepo.Update(ctx, withdrawal)
		}
		return nil, fmt.Errorf("cash-out with Yenga Pay: %w", err)
	}

	// 8. Mettre à jour le retrait avec la réponse
	if err := withdrawal.MarkProcessing(cashOutResp.ProviderRef); err != nil {
		return nil, fmt.Errorf("mark processing: %w", err)
	}

	if cashOutResp.Status == "SUCCESS" || cashOutResp.Status == "DONE" {
		if err := withdrawal.MarkSuccess(
			cashOutResp.Fees*100,
			cashOutResp.Amount*100,
			"",
		); err != nil {
			return nil, fmt.Errorf("mark success: %w", err)
		}
	}

	if err := uc.withdrawalRepo.Update(ctx, withdrawal); err != nil {
		return nil, fmt.Errorf("update withdrawal: %w", err)
	}

	logger.Info().
		Str("withdrawal_id", withdrawal.ID.String()).
		Str("provider_ref", cashOutResp.ProviderRef).
		Str("status", cashOutResp.Status).
		Msg("Withdrawal created and processed successfully")

	return toResponse(withdrawal), nil
}

func toResponse(w *entity.Withdrawal) *withdrawaldto.WithdrawalResponse {
	resp := &withdrawaldto.WithdrawalResponse{
		ID:                w.ID.String(),
		ShopID:            w.ShopID.String(),
		Provider:          w.Provider,
		AmountCents:       w.AmountCents,
		Currency:          string(w.Currency),
		FeesCents:         w.FeesCents,
		NetAmountCents:    w.NetAmountCents,
		Status:            string(w.Status),
		PaymentMethod:     string(w.PaymentMethod),
		DestinationNumber: w.DestinationNumber,
		CreatedAt:         w.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	}

	if w.ProviderRef != nil {
		resp.ProviderRef = *w.ProviderRef
	}
	if w.DestinationName != nil {
		resp.DestinationName = *w.DestinationName
	}
	if w.DestinationEmail != nil {
		resp.DestinationEmail = *w.DestinationEmail
	}
	if w.Description != nil {
		resp.Description = *w.Description
	}
	if w.ErrorMessage != nil {
		resp.ErrorMessage = *w.ErrorMessage
	}
	if w.OperatorTransactionID != nil {
		resp.OperatorTransactionID = *w.OperatorTransactionID
	}
	if w.ProcessedAt != nil {
		resp.ProcessedAt = w.ProcessedAt.UTC().Format("2006-01-02 15:04:05")
	}

	return resp
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
