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
	orderRepo   repository.OrderRepository
	paymentRepo repository.PaymentRepository
	walletRepo  repository.MerchantWalletRepository
	freezeRepo  repository.AccountFreezeRepository
	batchRepo   repository.CommissionBatchRepository
	escrowRepo  repository.EscrowAccountRepository
}

// NewGetMerchantOverviewUsecase crée une nouvelle instance
func NewGetMerchantOverviewUsecase(
	orderRepo repository.OrderRepository,
	paymentRepo repository.PaymentRepository,
	walletRepo repository.MerchantWalletRepository,
	freezeRepo repository.AccountFreezeRepository,
	batchRepo repository.CommissionBatchRepository,
	escrowRepo repository.EscrowAccountRepository,
) *GetMerchantOverviewUsecase {
	return &GetMerchantOverviewUsecase{
		orderRepo:   orderRepo,
		paymentRepo: paymentRepo,
		walletRepo:  walletRepo,
		freezeRepo:  freezeRepo,
		batchRepo:   batchRepo,
		escrowRepo:  escrowRepo,
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

	// 3. Wallet et gel
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

	// 4. Commission du mois
	monthlyCommission, err := uc.batchRepo.GetMonthlyCommissionByShop(ctx, shopID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get monthly commission")
		monthlyCommission = 0
	}

	// 5. 🆕 CALCUL DU SOLDE EN ATTENTE (ESCROW)
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

	// 6. Construire la réponse
	// Note: Les champs liés au crédit sont mis à 0 car le module crédit a été supprimé.
	return &merchantdto.MerchantOverviewResponse{
		TotalSalesCents:           totalSales,
		TotalOrdersCount:          ordersCount,
		OnlineSalesCents:          onlineSales,
		CashSalesCents:            cashSales,
		ActiveContractsCount:      0, // Module crédit supprimé
		TotalFinancedCents:        0, // Module crédit supprimé
		TotalOutstandingCents:     0, // Module crédit supprimé
		RecoveryRatePercent:       0, // Module crédit supprimé
		OverdueAmountCents:        0, // Module crédit supprimé
		OverdueCount:              0, // Module crédit supprimé
		WalletBalanceCents:        walletBalance,
		PendingEscrowBalanceCents: pendingEscrowBalanceCents,
		IsFrozen:                  isFrozen,
		FreezeReason:              freezeReason,
		AmountDueCents:            amountDue,
		GracePeriodEndsAt:         gracePeriodEnds,
		MonthlyCommissionCents:    monthlyCommission,
		GeneratedAt:               time.Now().UTC(),
	}, nil
}
