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
	SweptCents       int64 `json:"swept_cents,omitempty"`
	DebtCentsAfter   int64 `json:"debt_cents_after,omitempty"`
	BalanceCents     int64 `json:"balance_cents,omitempty"`
	AvailableCents   int64 `json:"available_cents,omitempty"`
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

// Execute libère le held installment, prélève la commission plateforme,
// puis applique un debt sweep sur le disponible (balance - held).
//
// Modèle :
//  1. ReleaseHeld(total)     → held ↓, disponible ↑ (balance inchangée)
//  2. Balance -= commission  → part plateforme
//  3. Sweep dette            → min(debt, available) : DebtCents ↓ et BalanceCents ↓
//
// Pas de CreditWithDebtSweep ici : les fonds sont déjà dans balance (held).
func (uc *ReleaseEscrowFundsUsecase) Execute(ctx context.Context, orderID string) (*ReleaseResponse, error) {
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Commande → shopID + montant
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("commande introuvable: %w", err)
	}
	shopID := order.ShopID

	// 2. Taux commission (TODO: commissionRateRepo.GetPlatformRate)
	commissionRateBps := 500 // 5%

	// 3. Calculs
	totalCents := order.TotalCents
	if totalCents <= 0 {
		return nil, fmt.Errorf("montant commande invalide: %d", totalCents)
	}
	commissionCents := (totalCents * int64(commissionRateBps)) / 10000
	if commissionCents < 0 {
		commissionCents = 0
	}
	if commissionCents > totalCents {
		commissionCents = totalCents
	}
	netMerchantCents := totalCents - commissionCents

	// 4. Wallet FOR UPDATE (même TX)
	walletRepoTx := uc.walletRepo
	if withTX, ok := interface{}(uc.walletRepo).(interface {
		WithTX(tx repository.Tx) repository.MerchantWalletRepository
	}); ok {
		walletRepoTx = withTX.WithTX(tx)
	}

	wallet, err := walletRepoTx.FindByShopIDForUpdate(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("portefeuille marchand introuvable: %w", err)
	}

	if wallet.HeldCents < totalCents {
		return nil, fmt.Errorf(
			"fonds séquestrés insuffisants: attendu %d, disponible %d",
			totalCents, wallet.HeldCents,
		)
	}

	// 5. Libérer le held (disponible augmente de totalCents)
	if err := wallet.ReleaseHeld(totalCents); err != nil {
		return nil, fmt.Errorf("échec de la libération du séquestre: %w", err)
	}

	// 6. Commission plateforme sur le balance
	if commissionCents > 0 {
		if wallet.BalanceCents < commissionCents {
			return nil, fmt.Errorf(
				"solde insuffisant pour commission: balance=%d commission=%d",
				wallet.BalanceCents, commissionCents,
			)
		}
		wallet.BalanceCents -= commissionCents
		wallet.TotalCommissionsCents += commissionCents
	}

	// 7. Debt sweep sur le disponible (après commission)
	//    Même sémantique que payer une dette avec du cash libéré :
	//    DebtCents ↓ et BalanceCents ↓ du montant swept.
	var swept int64
	if wallet.DebtCents > 0 {
		available := wallet.AvailableCents()
		if available < 0 {
			available = 0
		}
		swept = wallet.DebtCents
		if available < swept {
			swept = available
		}
		if swept > 0 {
			wallet.DebtCents -= swept
			wallet.BalanceCents -= swept
			if wallet.DebtCents < 0 {
				wallet.DebtCents = 0
			}
		}
	}

	wallet.UpdatedAt = time.Now().UTC()

	if err := walletRepoTx.Update(ctx, wallet); err != nil {
		return nil, fmt.Errorf("échec de la mise à jour du portefeuille après libération: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("échec du commit de la libération des fonds: %w", err)
	}

	log.Info().
		Str("order_id", orderID).
		Str("shop_id", shopID).
		Int64("total_cents", totalCents).
		Int64("commission_cents", commissionCents).
		Int64("net_cents", netMerchantCents).
		Int64("swept_cents", swept).
		Int64("balance_cents", wallet.BalanceCents).
		Int64("held_cents", wallet.HeldCents).
		Int64("debt_cents", wallet.DebtCents).
		Int64("available_cents", wallet.AvailableCents()).
		Msg("Installment escrow released (commission + debt sweep if any)")

	return &ReleaseResponse{
		NetMerchantCents: netMerchantCents,
		CommissionCents:  commissionCents,
		SweptCents:       swept,
		DebtCentsAfter:   wallet.DebtCents,
		BalanceCents:     wallet.BalanceCents,
		AvailableCents:   wallet.AvailableCents(),
	}, nil
}
