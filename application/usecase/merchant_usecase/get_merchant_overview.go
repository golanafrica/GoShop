package merchantusecase

import (
	"context"
	"fmt"
	"time"

	merchantdto "Goshop/application/dto/merchant_dto"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// GetMerchantOverviewUsecase retourne la vue d'ensemble d'un marchand
type GetMerchantOverviewUsecase struct {
	orderRepo       repository.OrderRepository
	paymentRepo     repository.PaymentRepository
	contractRepo    repository.CreditContractRepository
	installmentRepo repository.CreditInstallmentRepository
	walletRepo      repository.MerchantWalletRepository
	freezeRepo      repository.AccountFreezeRepository
	batchRepo       repository.CommissionBatchRepository
	escrowRepo      repository.EscrowAccountRepository // 🆕 AJOUT pour le solde en attente
}

// NewGetMerchantOverviewUsecase crée une nouvelle instance
func NewGetMerchantOverviewUsecase(
	orderRepo repository.OrderRepository,
	paymentRepo repository.PaymentRepository,
	contractRepo repository.CreditContractRepository,
	installmentRepo repository.CreditInstallmentRepository,
	walletRepo repository.MerchantWalletRepository,
	freezeRepo repository.AccountFreezeRepository,
	batchRepo repository.CommissionBatchRepository,
	escrowRepo repository.EscrowAccountRepository, // 🆕 AJOUT
) *GetMerchantOverviewUsecase {
	return &GetMerchantOverviewUsecase{
		orderRepo:       orderRepo,
		paymentRepo:     paymentRepo,
		contractRepo:    contractRepo,
		installmentRepo: installmentRepo,
		walletRepo:      walletRepo,
		freezeRepo:      freezeRepo,
		batchRepo:       batchRepo,
		escrowRepo:      escrowRepo, // 🆕 AJOUT
	}
}

// Execute retourne la vue d'ensemble du marchand
func (uc *GetMerchantOverviewUsecase) Execute(ctx context.Context) (*merchantdto.MerchantOverviewResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	logger.Info().Str("shop_id", shopID).Msg("Generating merchant overview")

	// 2. Ventes totales (à implémenter avec des requêtes agrégées plus tard, 0 pour l'instant)
	totalSales := int64(0)
	ordersCount := 0
	onlineSales := int64(0)
	cashSales := totalSales - onlineSales

	// 3. Statistiques de crédit
	creditStats, err := uc.contractRepo.GetMerchantCreditStats(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get credit stats")
		return nil, fmt.Errorf("get credit stats: %w", err)
	}

	// 4. Statistiques de recouvrement
	recoveryStats, err := uc.installmentRepo.GetMerchantRecoveryStats(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get recovery stats")
		return nil, fmt.Errorf("get recovery stats: %w", err)
	}

	// 5. Wallet et gel
	wallet, err := uc.walletRepo.FindByShopID(ctx, shopID)
	if err != nil {
		logger.Warn().Err(err).Msg("Wallet not found, using default")
		wallet = nil
	}

	var walletBalance int64
	var isFrozen bool
	var freezeReason *string
	var amountDue int64
	var gracePeriodEnds *time.Time

	if wallet != nil {
		walletBalance = wallet.BalanceCents
		isFrozen = wallet.IsFrozen

		if isFrozen {
			freeze, err := uc.freezeRepo.FindActiveByShopID(ctx, shopID)
			if err == nil && freeze != nil {
				reasonStr := string(freeze.FreezeReason)
				freezeReason = &reasonStr
				amountDue = freeze.AmountDueCents
				gracePeriodEnds = &freeze.GracePeriodEndsAt
			}
		}
	}

	// 6. Commission du mois
	monthlyCommission, err := uc.batchRepo.GetMonthlyCommissionByShop(ctx, shopID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get monthly commission")
		monthlyCommission = 0
	}

	// 7. 🆕 CALCUL DU SOLDE EN ATTENTE (ESCROW)
	var pendingEscrowBalanceCents int64
	heldEscrows, err := uc.escrowRepo.FindHeldByShopID(ctx, shopID)
	if err == nil {
		for _, esc := range heldEscrows {
			// GetMerchantAmount() retourne déjà (TotalAmountCents - CommissionCents)
			pendingEscrowBalanceCents += esc.GetMerchantAmount()
		}
	} else {
		logger.Warn().Err(err).Msg("Failed to get held escrows for overview")
	}

	// 8. Construire la réponse
	// ⚠️ NOTE : Assure-toi d'ajouter le champ `PendingEscrowBalanceCents int64`
	// dans la struct `MerchantOverviewResponse` de ton fichier DTO.
	return &merchantdto.MerchantOverviewResponse{
		TotalSalesCents:           totalSales,
		TotalOrdersCount:          ordersCount,
		OnlineSalesCents:          onlineSales,
		CashSalesCents:            cashSales,
		ActiveContractsCount:      creditStats.ActiveContractsCount,
		TotalFinancedCents:        creditStats.TotalFinancedCents,
		TotalOutstandingCents:     creditStats.TotalOutstandingCents,
		RecoveryRatePercent:       recoveryStats.RecoveryRatePercent,
		OverdueAmountCents:        recoveryStats.OverdueAmountCents,
		OverdueCount:              recoveryStats.OverdueCount,
		WalletBalanceCents:        walletBalance,
		PendingEscrowBalanceCents: pendingEscrowBalanceCents, // 🆕 CHAMP AJOUTÉ
		IsFrozen:                  isFrozen,
		FreezeReason:              freezeReason,
		AmountDueCents:            amountDue,
		GracePeriodEndsAt:         gracePeriodEnds,
		MonthlyCommissionCents:    monthlyCommission,
		GeneratedAt:               time.Now().UTC(),
	}, nil
}
