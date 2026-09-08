package installmentusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/repository"

	"github.com/rs/zerolog/log"
)

// ReleaseResponse est défini ici pour que le handler puisse l'utiliser
type ReleaseResponse struct {
	NetMerchantCents int64 `json:"net_merchant_cents"`
	CommissionCents  int64 `json:"commission_cents"`
}

type ReleaseEscrowFundsUsecase struct {
	txManager          repository.TxManager
	orderRepo          repository.OrderRepository
	walletRepo         repository.MerchantWalletRepository
	commissionRateRepo repository.CommissionRateRepository
}

func NewReleaseEscrowFundsUsecase(
	txManager repository.TxManager,
	orderRepo repository.OrderRepository,
	walletRepo repository.MerchantWalletRepository,
	commissionRateRepo repository.CommissionRateRepository,
) *ReleaseEscrowFundsUsecase {
	return &ReleaseEscrowFundsUsecase{
		txManager:          txManager,
		orderRepo:          orderRepo,
		walletRepo:         walletRepo,
		commissionRateRepo: commissionRateRepo,
	}
}

// Execute ne prend PLUS shopID en paramètre, il le récupère lui-même via la commande
func (uc *ReleaseEscrowFundsUsecase) Execute(ctx context.Context, orderID string) (*ReleaseResponse, error) {
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Récupérer la commande pour obtenir le shopID et le montant total
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("commande introuvable: %w", err)
	}
	shopID := order.ShopID

	// 2. Récupérer le taux de commission (ex: 5% = 500 bps)
	commissionRateBps := 500 // À remplacer par uc.commissionRateRepo.GetPlatformRate(ctx) plus tard

	// 3. Calculs financiers
	totalCents := order.TotalCents
	commissionCents := (totalCents * int64(commissionRateBps)) / 10000
	netMerchantCents := totalCents - commissionCents

	// 4. Libérer les fonds de manière atomique
	wallet, err := uc.walletRepo.FindByShopIDForUpdate(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("portefeuille marchand introuvable: %w", err)
	}

	if wallet.HeldCents < totalCents {
		return nil, fmt.Errorf("fonds séquestrés insuffisants: attendu %d, disponible %d", totalCents, wallet.HeldCents)
	}

	if err := wallet.ReleaseHeld(totalCents); err != nil {
		return nil, fmt.Errorf("échec de la libération du séquestre: %w", err)
	}

	wallet.BalanceCents -= commissionCents
	wallet.TotalCommissionsCents += commissionCents
	wallet.UpdatedAt = time.Now().UTC()

	if err := uc.walletRepo.Update(ctx, wallet); err != nil {
		return nil, fmt.Errorf("échec de la mise à jour du portefeuille après libération: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("échec du commit de la libération des fonds: %w", err)
	}

	log.Info().Str("order_id", orderID).Int64("total_cents", totalCents).Int64("commission_cents", commissionCents).Int64("net_cents", netMerchantCents).Msg("Escrow funds released successfully")

	return &ReleaseResponse{
		NetMerchantCents: netMerchantCents,
		CommissionCents:  commissionCents,
	}, nil
}
