package withdrawalusecase

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	walletRepo      repository.MerchantWalletRepository
	registry        PaymentRegistry
	yengaPayFactory YengaPayProviderFactory
	debitWalletUC   *walletusecase.DebitWalletUsecase
	creditWalletUC  *walletusecase.CreditWalletUsecase // P0-A : reverse si cash-out fail définitivement
}

func NewCreateWithdrawalUsecase(
	withdrawalRepo repository.WithdrawalRepository,
	shopPaymentRepo ShopPaymentSettingsRepository,
	walletRepo repository.MerchantWalletRepository,
	registry PaymentRegistry,
	debitWalletUC *walletusecase.DebitWalletUsecase,
	creditWalletUC *walletusecase.CreditWalletUsecase,
) *CreateWithdrawalUsecase {
	return &CreateWithdrawalUsecase{
		withdrawalRepo:  withdrawalRepo,
		shopPaymentRepo: shopPaymentRepo,
		walletRepo:      walletRepo,
		registry:        registry,
		yengaPayFactory: defaultYengaPayFactory,
		debitWalletUC:   debitWalletUC,
		creditWalletUC:  creditWalletUC,
	}
}

func defaultYengaPayFactory(config payment.YengaPayConfig) (CashOutProvider, error) {
	return payment.NewYengaPayProvider(config)
}

func NewCreateWithdrawalUsecaseWithFactory(
	withdrawalRepo repository.WithdrawalRepository,
	shopPaymentRepo ShopPaymentSettingsRepository,
	walletRepo repository.MerchantWalletRepository,
	registry PaymentRegistry,
	yengaPayFactory YengaPayProviderFactory,
	debitWalletUC *walletusecase.DebitWalletUsecase,
	creditWalletUC *walletusecase.CreditWalletUsecase,
) *CreateWithdrawalUsecase {
	return &CreateWithdrawalUsecase{
		withdrawalRepo:  withdrawalRepo,
		shopPaymentRepo: shopPaymentRepo,
		walletRepo:      walletRepo,
		registry:        registry,
		yengaPayFactory: yengaPayFactory,
		debitWalletUC:   debitWalletUC,
		creditWalletUC:  creditWalletUC,
	}
}

func (uc *CreateWithdrawalUsecase) Execute(ctx context.Context, req *withdrawaldto.CreateWithdrawalRequest) (*withdrawaldto.WithdrawalResponse, error) {
	logger := zerolog.Ctx(ctx)

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
	// KYC
	// ============================================================
	if !shop.CanWithdraw() {
		logger.Warn().
			Str("shop_id", shop.ID.String()).
			Str("shop_name", shop.Name).
			Str("kyc_status", string(shop.KYCStatus)).
			Int64("amount_cents", req.AmountCents).
			Msg("Retrait bloque : KYC non verifie")

		var message string
		switch shop.KYCStatus {
		case entity.ShopKYCStatusUnverified:
			message = "Verification KYC requise pour les retraits. Veuillez soumettre votre piece d'identite et le registre de commerce."
		case entity.ShopKYCStatusPending:
			message = "Vos documents KYC sont en cours d'examen. Les retraits seront disponibles une fois la verification terminee."
		case entity.ShopKYCStatusRejected:
			if shop.KYCRejectionReason != nil {
				message = fmt.Sprintf("Votre KYC a ete rejete : %s. Veuillez renvoyer des documents corriges.", *shop.KYCRejectionReason)
			} else {
				message = "Votre KYC a ete rejete. Veuillez renvoyer des documents corriges pour debloquer les retraits."
			}
		default:
			message = "Verification KYC marchand requise pour les retraits."
		}

		return nil, fmt.Errorf("%s (statut actuel : %s)", message, shop.KYCStatus)
	}

	logger.Debug().
		Str("shop_id", shop.ID.String()).
		Str("kyc_status", string(shop.KYCStatus)).
		Msg("KYC verifie, retrait autorise")

	// ============================================================
	// Fail-fast : available + debt + freeze
	// ============================================================
	if req.AmountCents <= 0 {
		return nil, fmt.Errorf("Le montant du retrait doit etre positif")
	}

	if uc.walletRepo != nil {
		wallet, err := uc.walletRepo.FindByShopID(ctx, shop.ID.String())
		if err != nil {
			logger.Error().Err(err).Msg("Failed to load wallet for available check")
			return nil, fmt.Errorf("impossible de charger le portefeuille : %w", err)
		}

		if wallet.IsFrozen {
			logger.Warn().
				Str("shop_id", shop.ID.String()).
				Msg("Retrait bloque : portefeuille gele")
			return nil, fmt.Errorf(
				"Retrait impossible : votre portefeuille est temporairement gele. Contactez le support si besoin",
			)
		}

		if wallet.DebtCents > 0 {
			logger.Warn().
				Str("shop_id", shop.ID.String()).
				Int64("debt_cents", wallet.DebtCents).
				Int64("balance_cents", wallet.BalanceCents).
				Int64("requested", req.AmountCents).
				Msg("Retrait bloque : dette residuelle")
			return nil, fmt.Errorf(
				"Retrait impossible : une dette residuelle de %d XOF doit etre remboursee avant tout retrait (solde : %d XOF)",
				wallet.DebtCents/100, wallet.BalanceCents/100,
			)
		}

		available := wallet.AvailableCents()

		if wallet.BalanceCents <= 0 || available <= 0 {
			logger.Warn().
				Str("shop_id", shop.ID.String()).
				Int64("balance_cents", wallet.BalanceCents).
				Int64("held_cents", wallet.HeldCents).
				Int64("available_cents", available).
				Msg("Retrait bloque : solde disponible nul ou negatif")
			return nil, fmt.Errorf(
				"Solde insuffisant pour un retrait (disponible : %d XOF, solde : %d XOF, gele : %d XOF)",
				available/100, wallet.BalanceCents/100, wallet.HeldCents/100,
			)
		}

		if req.AmountCents > available {
			logger.Warn().
				Str("shop_id", shop.ID.String()).
				Int64("requested", req.AmountCents).
				Int64("balance_cents", wallet.BalanceCents).
				Int64("held_cents", wallet.HeldCents).
				Int64("available_cents", available).
				Msg("Retrait bloque : fonds insuffisants (held inclus)")
			return nil, fmt.Errorf(
				"Solde disponible insuffisant : demande %d XOF, disponible %d XOF (solde %d XOF, dont %d XOF geles)",
				req.AmountCents/100, available/100, wallet.BalanceCents/100, wallet.HeldCents/100,
			)
		}
	} else {
		logger.Warn().Msg("walletRepo not configured on CreateWithdrawalUsecase — skipping fail-fast available check")
	}

	// ============================================================
	// 1) Entite + persist AVANT debit (pour reference_id ledger)
	// ============================================================
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

	if err := uc.withdrawalRepo.Create(ctx, withdrawal); err != nil {
		return nil, fmt.Errorf("save withdrawal: %w", err)
	}

	withdrawalID := withdrawal.ID.String()
	refTypePayout := "payout"
	refID := withdrawalID
	debitDesc := fmt.Sprintf("Withdrawal payout %s", withdrawalID)

	// ============================================================
	// 2) Debit wallet (garde atomique held dans DebitWalletUsecase)
	// ============================================================
	if uc.debitWalletUC == nil {
		_ = markWithdrawalFailed(uc, ctx, withdrawal, "debit wallet usecase not configured")
		return nil, fmt.Errorf("debit wallet usecase not configured")
	}

	debitReq := &walletusecase.DebitWalletRequest{
		ShopID:          shop.ID.String(),
		AmountCents:     req.AmountCents,
		TransactionType: entity.WalletTxPayout,
		ReferenceType:   &refTypePayout,
		ReferenceID:     &refID,
		Description:     &debitDesc,
		AllowNegative:   false,
	}

	_, err = uc.debitWalletUC.Execute(ctx, debitReq)
	if err != nil {
		_ = markWithdrawalFailed(uc, ctx, withdrawal, err.Error())
		logger.Error().Err(err).Msg("Failed to debit wallet for withdrawal")
		return nil, fmt.Errorf("Fonds insuffisants ou erreur portefeuille : %w", err)
	}

	debited := true

	// Helper local : reverse + mark failed
	failAfterDebit := func(cause error, reason string) (*withdrawaldto.WithdrawalResponse, error) {
		revErr := uc.reversePayoutDebit(ctx, shop.ID.String(), req.AmountCents, withdrawalID, reason)
		if revErr != nil {
			logger.Error().
				Err(revErr).
				Str("withdrawal_id", withdrawalID).
				Str("original_error", cause.Error()).
				Msg("CRITICAL: payout debit reverse failed — manual reconciliation required")
		}
		_ = markWithdrawalFailed(uc, ctx, withdrawal, cause.Error())
		return nil, fmt.Errorf("%s: %w", reason, cause)
	}

	// ============================================================
	// 3) Config Yenga + CashOut
	// ============================================================
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
		providerConfig = payment.YengaPayConfig{
			APIKey:         getEnvOrDefault("YENGA_PAY_API_KEY", ""),
			OrganizationID: getEnvOrDefault("YENGA_PAY_ORGANIZATION_ID", ""),
			ProjectID:      getEnvOrDefault("YENGA_PAY_PROJECT_ID", ""),
			WebhookSecret:  getEnvOrDefault("YENGA_PAY_WEBHOOK_SECRET", ""),
			Env:            getEnvOrDefault("YENGA_PAY_ENV", "test"),
		}
		logger.Info().Msg("Using global Yenga Pay configuration for cash-out")
	}

	yengaProvider, err := uc.yengaPayFactory(providerConfig)
	if err != nil {
		if debited {
			return failAfterDebit(err, "create yenga provider")
		}
		return nil, fmt.Errorf("create yenga provider: %w", err)
	}

	cashOutReq := &payment.CashOutRequest{
		AmountCents:       req.AmountCents,
		PaymentMethod:     req.PaymentMethod,
		DestinationNumber: req.DestinationNumber,
		DestinationName:   req.DestinationName,
		DestinationEmail:  req.DestinationEmail,
		Description:       req.Description,
	}

	// ============================================================
	// 4) Appel Provider + Gestion P0 des erreurs (Timeout Trap)
	// ============================================================
	cashOutResp, err := yengaProvider.CashOut(ctx, cashOutReq)
	if err != nil {
		// 🛡️ P0 CRITIQUE : Distinguer les erreurs réseau/timeout des erreurs métier définitives
		if isNetworkOrTimeoutError(err) {
			// Erreur ambiguë (timeout, 502, etc.) : NE PAS ANNULER LE DÉBIT.
			// On laisse le retrait en statut "processing" et on attend le webhook de résolution.
			logger.Warn().
				Err(err).
				Str("withdrawal_id", withdrawalID).
				Msg("CashOut network error/timeout. Leaving as PROCESSING for webhook reconciliation. DO NOT REVERSE DEBIT.")

			if markErr := withdrawal.MarkProcessing(""); markErr != nil {
				return nil, fmt.Errorf("mark processing after network error: %w", markErr)
			}
			if updateErr := uc.withdrawalRepo.Update(ctx, withdrawal); updateErr != nil {
				return nil, fmt.Errorf("update withdrawal after network error: %w", updateErr)
			}

			return nil, fmt.Errorf("le retrait est en cours de traitement et sera confirmé par le fournisseur sous peu (référence: %s)", withdrawalID)
		}

		// Erreur métier définitive (ex: numéro invalide, compte inexistant) → Annulation sûre
		return failAfterDebit(err, "cash-out definitively rejected by provider")
	}

	// CashOut accepte : NE PAS reverse (fonds engagés côté provider)
	if err := withdrawal.MarkProcessing(cashOutResp.ProviderRef); err != nil {
		logger.Error().Err(err).
			Str("withdrawal_id", withdrawalID).
			Str("provider_ref", cashOutResp.ProviderRef).
			Msg("MarkProcessing failed after successful CashOut — do NOT reverse wallet")
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

// reversePayoutDebit restaure le solde apres un debit payout si le cash-out a échoué définitivement.
// Idempotent via reference_type=withdrawal_reversal + reference_id=withdrawal_id.
func (uc *CreateWithdrawalUsecase) reversePayoutDebit(
	ctx context.Context,
	shopID string,
	amountCents int64,
	withdrawalID string,
	reason string,
) error {
	logger := zerolog.Ctx(ctx)

	if uc.creditWalletUC == nil {
		return fmt.Errorf("credit wallet usecase not configured — cannot reverse payout debit")
	}
	if amountCents <= 0 {
		return nil
	}

	refType := "withdrawal_reversal"
	refID := withdrawalID
	desc := fmt.Sprintf("Payout reverse for withdrawal %s (%s)", withdrawalID, reason)

	req := &walletusecase.CreditWalletRequest{
		ShopID:      shopID,
		AmountCents: amountCents,
		// 🛡️ P0 : Utiliser un type dédié à l'annulation pour la traçabilité du ledger (au lieu de Deposit)
		// Assurez-vous que entity.WalletTxRefund existe dans votre package entity. Sinon, utilisez entity.WalletTxDeposit.
		TransactionType: entity.WalletTxRefund,
		ReferenceType:   &refType,
		ReferenceID:     &refID,
		Description:     &desc,
		HoldAfterCredit: false,
	}

	_, err := uc.creditWalletUC.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("credit reverse failed: %w", err)
	}

	logger.Info().
		Str("shop_id", shopID).
		Str("withdrawal_id", withdrawalID).
		Int64("amount_cents", amountCents).
		Str("reason", reason).
		Msg("Payout debit reversed after definitive provider failure")

	return nil
}

// isNetworkOrTimeoutError vérifie si l'erreur est une erreur réseau/timeout ambiguë
// où le provider a pu traiter la demande malgré tout (risque de double paiement si on annule).
func isNetworkOrTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Ajoutez ici les mots-clés spécifiques à votre provider YengaPay si nécessaire
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "502 bad gateway") ||
		strings.Contains(errStr, "504 gateway timeout") ||
		strings.Contains(errStr, "i/o timeout")
}

func markWithdrawalFailed(
	uc *CreateWithdrawalUsecase,
	ctx context.Context,
	w *entity.Withdrawal,
	msg string,
) error {
	if w == nil {
		return nil
	}
	if markErr := w.MarkFailed(msg); markErr != nil {
		return markErr
	}
	return uc.withdrawalRepo.Update(ctx, w)
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
