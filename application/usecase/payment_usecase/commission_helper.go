package paymentusecase

import (
	"context"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// maxCommissionBps = 15 % (aligné entity.CommissionRate.Validate / CalculateCommission)
const maxCommissionBps = 1500

// ResolveOnlineCommissionBps retourne le taux GoShop en basis points pour un paiement online.
//
// Priorité :
//  1. Ligne active dans commission_rates (shop + transaction_type = online_payment)
//  2. Défaut plateforme entity.DefaultRateOnlinePayment (250 = 2.5 %)
//
// Le résultat est toujours clampé dans [0, 1500].
// rateRepo peut être nil (tests / wiring incomplet) → fallback plateforme.
func ResolveOnlineCommissionBps(
	ctx context.Context,
	rateRepo repository.CommissionRateRepository,
	shopID string,
	logger *zerolog.Logger,
) int {
	fallback := entity.DefaultRateOnlinePayment // 250

	if rateRepo == nil || shopID == "" {
		return clampBps(fallback)
	}

	rate, err := rateRepo.FindByShopAndType(ctx, shopID, entity.TransactionTypeOnlinePayment)
	if err != nil || rate == nil || !rate.IsActive {
		if logger != nil {
			logger.Debug().
				Str("shop_id", shopID).
				Str("tx_type", entity.TransactionTypeOnlinePayment).
				Int("fallback_bps", fallback).
				Msg("no active shop online_payment rate — using platform default")
		}
		return clampBps(fallback)
	}

	if logger != nil {
		logger.Info().
			Str("shop_id", shopID).
			Str("tx_type", entity.TransactionTypeOnlinePayment).
			Int("rate_bps", rate.RateBps).
			Msg("using shop-specific online_payment commission rate")
	}
	return clampBps(rate.RateBps)
}

// CalculateGoShopCommissionCents calcule la commission plateforme en centimes.
// Réutilise entity.CalculateCommission (même formule que tontine + escrow).
func CalculateGoShopCommissionCents(amountCents int64, rateBps int) int64 {
	rateBps = clampBps(rateBps)
	if amountCents <= 0 || rateBps <= 0 {
		return 0
	}
	return entity.CalculateCommission(amountCents, rateBps)
}

func clampBps(bps int) int {
	if bps < 0 {
		return 0
	}
	if bps > maxCommissionBps {
		return maxCommissionBps
	}
	return bps
}
