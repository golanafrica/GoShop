package paymentusecase

import (
	"context"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

// maxCommissionBps = 15 % (aligné entity.CommissionRate.Validate / CalculateCommission)
const maxCommissionBps = 1500

// OrderSettlement décrit la répartition d'un paiement order.
//
// Modèle métier (Phase 2) :
//
//	GrossCents          = montant payé par le client (payment.AmountCents)
//	ProviderFeesCents   = frais PSP Yenga (déjà prélevés côté provider)
//	CommissionRateBps   = taux GoShop 0–1500 (commission_rates ou défaut 250)
//	CommissionCents     = commission plateforme GoShop
//	EscrowTotalCents    = montant séquestré = Gross - ProviderFees
//	MerchantNetCents    = ce que reçoit le marchand à la release = EscrowTotal - Commission
//
// Invariant : Gross = ProviderFees + Commission + MerchantNet
// (si ProviderFees ou Commission > Gross, les montants sont clampés pour éviter un net négatif)
type OrderSettlement struct {
	GrossCents        int64
	ProviderFeesCents int64
	CommissionRateBps int
	CommissionCents   int64
	EscrowTotalCents  int64
	MerchantNetCents  int64
}

// ResolveOnlineCommissionBps retourne le taux GoShop en basis points pour un paiement online.
//
// Priorité :
//  1. Ligne active dans commission_rates (shop + transaction_type = online_payment)
//  2. Défaut plateforme entity.DefaultRateOnlinePayment (250 = 2.5 %)
//
// Toujours clampé dans [0, 1500].
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
func CalculateGoShopCommissionCents(amountCents int64, rateBps int) int64 {
	rateBps = clampBps(rateBps)
	if amountCents <= 0 || rateBps <= 0 {
		return 0
	}
	return entity.CalculateCommission(amountCents, rateBps)
}

// ComputeOrderSettlement calcule la répartition complète d'un paiement order.
//
// commissionBaseCents : base de calcul de la commission GoShop.
// En pratique = GrossCents (montant client). Les frais PSP ne réduisent pas
// la base commission plateforme (commission sur le CA brut).
//
// providerFeesCents : frais Yenga (0 si inconnu, ex. sync avant webhook fees).
func ComputeOrderSettlement(
	grossCents int64,
	providerFeesCents int64,
	commissionRateBps int,
) OrderSettlement {
	if grossCents < 0 {
		grossCents = 0
	}
	if providerFeesCents < 0 {
		providerFeesCents = 0
	}
	if providerFeesCents > grossCents {
		providerFeesCents = grossCents
	}

	rateBps := clampBps(commissionRateBps)
	commissionCents := CalculateGoShopCommissionCents(grossCents, rateBps)

	escrowTotal := grossCents - providerFeesCents
	if escrowTotal < 0 {
		escrowTotal = 0
	}

	// Commission ne peut pas dépasser le montant en escrow
	if commissionCents > escrowTotal {
		commissionCents = escrowTotal
	}

	merchantNet := escrowTotal - commissionCents
	if merchantNet < 0 {
		merchantNet = 0
	}

	return OrderSettlement{
		GrossCents:        grossCents,
		ProviderFeesCents: providerFeesCents,
		CommissionRateBps: rateBps,
		CommissionCents:   commissionCents,
		EscrowTotalCents:  escrowTotal,
		MerchantNetCents:  merchantNet,
	}
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
