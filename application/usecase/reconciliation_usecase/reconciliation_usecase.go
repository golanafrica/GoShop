package reconciliationusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/application/dto/reconciliation_dto"
	"Goshop/domain/repository"

	"github.com/rs/zerolog"
)

type ReconciliationUsecase struct {
	walletRepo repository.MerchantWalletRepository
	shopRepo   repository.ShopRepository
	logger     zerolog.Logger // 🆕 PR-D FIX : Pour tracer les erreurs silencieuses
}

func NewReconciliationUsecase(
	walletRepo repository.MerchantWalletRepository,
	shopRepo repository.ShopRepository,
	logger zerolog.Logger,
) *ReconciliationUsecase {
	return &ReconciliationUsecase{
		walletRepo: walletRepo,
		shopRepo:   shopRepo,
		logger:     logger.With().Str("component", "reconciliation_usecase").Logger(),
	}
}

// CheckShop vérifie la cohérence financière d'un shop spécifique via un snapshot atomique
func (uc *ReconciliationUsecase) CheckShop(ctx context.Context, shopID string) (*reconciliation_dto.ShopReconciliationResponse, error) {
	// 🆕 PR-D FIX : Utilisation du snapshot atomique pour éviter les race conditions
	snapshot, err := uc.walletRepo.GetReconciliationSnapshot(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation snapshot: %w", err)
	}

	resp := &reconciliation_dto.ShopReconciliationResponse{
		ShopID:        shopID,
		WalletBalance: snapshot.WalletBalance,
		LedgerBalance: snapshot.LedgerNet,
		BalanceDiff:   snapshot.WalletBalance - snapshot.LedgerNet,
		WalletHeld:    snapshot.WalletHeld,
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

	// Vérification bonus : Dette négative incohérente
	if snapshot.WalletDebt < 0 {
		if resp.Status == "OK" {
			resp.Status = "WARNING"
		}
		resp.Discrepancies = append(resp.Discrepancies,
			fmt.Sprintf("WARNING: Negative debt value detected (%d cents)", snapshot.WalletDebt))
	}

	return resp, nil
}

// CheckGlobal scanne les shops par pages et retourne un résumé agrégé
func (uc *ReconciliationUsecase) CheckGlobal(ctx context.Context, pageSize int) (*reconciliation_dto.GlobalReconciliationResponse, error) {
	if pageSize <= 0 {
		pageSize = 100 // Défaut sécurisé
	}

	resp := &reconciliation_dto.GlobalReconciliationResponse{
		FlaggedShops: []reconciliation_dto.ShopReconciliationResponse{},
	}

	offset := 0
	maxPages := 50 // 🆕 PR-D FIX : Protection contre les boucles infinies ou OOM (max 5000 shops par appel)

	for page := 0; page < maxPages; page++ {
		filters := &repository.ShopAdminFilters{
			Limit:  pageSize,
			Offset: offset,
		}

		shops, total, err := uc.shopRepo.FindAllShopsAdmin(ctx, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to list shops (page %d): %w", page, err)
		}

		if len(shops) == 0 {
			break // Plus de shops à traiter
		}

		for _, shop := range shops {
			check, err := uc.CheckShop(ctx, shop.ID.String())
			if err != nil {
				// 🆕 PR-D FIX : Ne plus ignorer silencieusement. Comptabiliser et logger.
				resp.ShopsFailed++
				uc.logger.Error().Err(err).Str("shop_id", shop.ID.String()).
					Msg("Reconciliation check failed for shop")
				continue
			}

			resp.TotalShopsChecked++

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

		// Si on a atteint le total, on a tout traité
		if offset+pageSize >= total {
			resp.HasMore = false
			break
		}

		offset += pageSize
		resp.HasMore = true // Il reste des pages à traiter au-delà de la limite actuelle
	}

	return resp, nil
}
