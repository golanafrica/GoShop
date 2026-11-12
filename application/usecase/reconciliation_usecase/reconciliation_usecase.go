package reconciliationusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/application/dto/reconciliation_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

type ReconciliationUsecase struct {
	walletRepo repository.MerchantWalletRepository
	shopRepo   repository.ShopRepository
}

func NewReconciliationUsecase(
	walletRepo repository.MerchantWalletRepository,
	shopRepo repository.ShopRepository,
) *ReconciliationUsecase {
	return &ReconciliationUsecase{
		walletRepo: walletRepo,
		shopRepo:   shopRepo,
	}
}

// CheckShop vérifie la cohérence financière d'un shop spécifique
func (uc *ReconciliationUsecase) CheckShop(ctx context.Context, shopID string) (*reconciliation_dto.ShopReconciliationResponse, error) {
	wallet, err := uc.walletRepo.FindByShopID(ctx, shopID)
	if err != nil {
		// Si le wallet n'existe pas encore, on considère un état à 0 pour ne pas bloquer l'analyse
		wallet = &entity.MerchantWallet{ShopID: shopID, BalanceCents: 0, HeldCents: 0, DebtCents: 0}
	}

	ledgerSum, err := uc.walletRepo.GetLedgerSum(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger sum: %w", err)
	}

	resp := &reconciliation_dto.ShopReconciliationResponse{
		ShopID:        shopID,
		WalletBalance: wallet.BalanceCents,
		LedgerBalance: ledgerSum.NetBalance,
		BalanceDiff:   wallet.BalanceCents - ledgerSum.NetBalance,
		WalletHeld:    wallet.HeldCents,
		LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Analyse des écarts
	if resp.BalanceDiff != 0 {
		resp.Status = "CRITICAL"
		resp.Discrepancies = append(resp.Discrepancies,
			fmt.Sprintf("CRITICAL: Balance mismatch. Wallet=%d, Ledger=%d, Diff=%d cents",
				resp.WalletBalance, resp.LedgerBalance, resp.BalanceDiff))
	} else {
		resp.Status = "OK"
	}

	// Vérification bonus : Dette négative incohérente (si jamais une logique buguée l'a causé)
	if wallet.DebtCents < 0 {
		// On passe en WARNING si c'était OK, sinon on garde CRITICAL
		if resp.Status == "OK" {
			resp.Status = "WARNING"
		}
		resp.Discrepancies = append(resp.Discrepancies,
			fmt.Sprintf("WARNING: Negative debt value detected (%d cents)", wallet.DebtCents))
	}

	return resp, nil
}

// CheckGlobal scanne les shops et retourne un résumé agrégé pour le tableau de bord admin
func (uc *ReconciliationUsecase) CheckGlobal(ctx context.Context, limit int) (*reconciliation_dto.GlobalReconciliationResponse, error) {
	// ✅ FIX : Utilisation de FindAllShopsAdmin avec une limite pour éviter de charger toute la base en mémoire
	filters := &repository.ShopAdminFilters{
		Limit:  limit,
		Offset: 0,
	}

	shops, _, err := uc.shopRepo.FindAllShopsAdmin(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list shops: %w", err)
	}

	resp := &reconciliation_dto.GlobalReconciliationResponse{
		TotalShopsChecked: len(shops),
		FlaggedShops:      []reconciliation_dto.ShopReconciliationResponse{},
	}

	for _, shop := range shops {
		check, err := uc.CheckShop(ctx, shop.ID.String())
		if err != nil {
			// On ignore les erreurs individuelles pour ne pas bloquer le rapport global entier
			// (En production, il est recommandé d'ajouter un log zerolog ici)
			continue
		}

		// ✅ FIX QF1003 : Utilisation d'un switch tagué pour une meilleure lisibilité et conformité au linter
		switch check.Status {
		case "CRITICAL":
			resp.ShopsCritical++
			resp.FlaggedShops = append(resp.FlaggedShops, *check)
		case "WARNING":
			resp.ShopsWarning++
			resp.FlaggedShops = append(resp.FlaggedShops, *check)
		default:
			resp.ShopsOK++
		}
	}

	return resp, nil
}
