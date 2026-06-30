package codusecase

import (
	"context"
	"fmt"
	"time"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ============================================================
// COLLECT COMMISSION USECASE
// ============================================================

// CollectCommissionRequest représente la requête pour collecter une commission COD
type CollectCommissionRequest struct {
	// Identifiants
	OrderID string `json:"order_id"`

	// Qui collecte (user_id du marchand ou admin)
	CollectedBy string `json:"collected_by"`

	// Optionnel : forcer la collecte même si incohérent (admin only)
	ForceCollect bool `json:"force_collect"`
}

// CollectCommissionResponse représente la réponse après collecte
type CollectCommissionResponse struct {
	// Preuve mise à jour
	ProofID          string                     `json:"proof_id"`
	OrderID          string                     `json:"order_id"`
	ShopID           string                     `json:"shop_id"`
	Status           entity.CODProofStatus      `json:"status"`
	CommissionStatus entity.CODCommissionStatus `json:"commission_status"`

	// Commission
	CommissionCents       int64      `json:"commission_cents"`
	CommissionCollectedAt *time.Time `json:"commission_collected_at,omitempty"`

	// Transaction wallet
	WalletTransactionID *string `json:"wallet_transaction_id,omitempty"`
	WalletBalanceBefore int64   `json:"wallet_balance_before"`
	WalletBalanceAfter  int64   `json:"wallet_balance_after"`

	// Cohérence
	AmountsMatch bool `json:"amounts_match"`
	DatesMatch   bool `json:"dates_match"`
	IsCoherent   bool `json:"is_coherent"`

	// Gel automatique (si déclenché)
	AccountFrozen  bool   `json:"account_frozen"`
	FreezeID       string `json:"freeze_id,omitempty"`
	AmountDueCents int64  `json:"amount_due_cents,omitempty"`

	// Message
	Message string `json:"message"`
}

// Validate valide la requête
func (r *CollectCommissionRequest) Validate() error {
	if r.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if r.CollectedBy == "" {
		return fmt.Errorf("collected_by is required")
	}
	return nil
}

// CollectCommissionUsecase collecte la commission GoShop sur une vente COD
type CollectCommissionUsecase struct {
	codProofRepo repository.CODProofRepository
	orderRepo    repository.OrderRepository
	walletUC     *walletusecase.DebitWalletUsecase
	freezeUC     *walletusecase.FreezeAccountUsecase
	txManager    repository.TxManager
}

// NewCollectCommissionUsecase crée une nouvelle instance
func NewCollectCommissionUsecase(
	codProofRepo repository.CODProofRepository,
	orderRepo repository.OrderRepository,
	walletUC *walletusecase.DebitWalletUsecase,
	freezeUC *walletusecase.FreezeAccountUsecase,
	txManager repository.TxManager,
) *CollectCommissionUsecase {
	return &CollectCommissionUsecase{
		codProofRepo: codProofRepo,
		orderRepo:    orderRepo,
		walletUC:     walletUC,
		freezeUC:     freezeUC,
		txManager:    txManager,
	}
}

// Execute collecte la commission GoShop
func (uc *CollectCommissionUsecase) Execute(ctx context.Context, req *CollectCommissionRequest) (*CollectCommissionResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		logger.Error().Err(err).Msg("Invalid collect commission request")
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Vérifier le multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Multi-tenant error")
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Récupérer la preuve COD
	proof, err := uc.codProofRepo.WithTX(tx).FindByOrderID(ctx, req.OrderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find COD proof")
		return nil, fmt.Errorf("COD proof not found: %w", err)
	}

	// 5. Vérifier que la preuve appartient au bon shop (multi-tenant)
	if proof.ShopID != shopID {
		return nil, fmt.Errorf("access denied: proof does not belong to tenant shop")
	}

	// 6. Vérifier que les deux preuves sont présentes
	if !proof.IsComplete() {
		return nil, fmt.Errorf("cannot collect commission: both client and merchant proofs are required")
	}

	// 7. Vérifier la cohérence (sauf si force_collect)
	if !req.ForceCollect && !proof.IsCoherent() {
		return nil, fmt.Errorf("cannot collect commission: proofs are incoherent (amounts_match=%v, dates_match=%v)",
			proof.AmountsMatch, proof.DatesMatch)
	}

	// 8. Vérifier que la commission n'est pas déjà collectée
	if proof.CommissionStatus == entity.CODCommissionCollected {
		return nil, fmt.Errorf("commission already collected")
	}

	// 9. Récupérer le wallet pour connaître le solde avant
	wallet, err := uc.walletUC.GetWallet(ctx, shopID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get wallet")
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	walletBalanceBefore := wallet.BalanceCents

	// 10. Tenter de prélever la commission
	debitReq := &walletusecase.DebitWalletRequest{
		ShopID:          shopID,
		AmountCents:     proof.CommissionCents,
		TransactionType: entity.WalletTxCommissionDebit,
		AllowNegative:   true, // Commission peut créer une dette
	}

	refType := "cod_order"
	refID := proof.OrderID
	description := fmt.Sprintf("GoShop commission for COD order %s", proof.OrderID)
	debitReq.ReferenceType = &refType
	debitReq.ReferenceID = &refID
	debitReq.Description = &description

	// 11. Effectuer le débit
	debitResp, debitErr := uc.walletUC.Execute(ctx, debitReq)

	// Préparer la réponse
	response := &CollectCommissionResponse{
		ProofID:             proof.ID,
		OrderID:             proof.OrderID,
		ShopID:              proof.ShopID,
		CommissionCents:     proof.CommissionCents,
		WalletBalanceBefore: walletBalanceBefore,
		AmountsMatch:        proof.AmountsMatch != nil && *proof.AmountsMatch,
		DatesMatch:          proof.DatesMatch != nil && *proof.DatesMatch,
		IsCoherent:          proof.IsCoherent(),
	}

	// 12. Gérer le résultat du débit
	if debitErr != nil {
		// Débit échoué (wallet gelé par exemple)
		logger.Error().Err(debitErr).Msg("Failed to debit commission")

		// Marquer la commission comme "due"
		if err := proof.MarkCommissionDue(); err != nil {
			return nil, fmt.Errorf("failed to mark commission as due: %w", err)
		}

		response.CommissionStatus = entity.CODCommissionDue
		response.WalletBalanceAfter = walletBalanceBefore
		response.Message = fmt.Sprintf("Commission could not be collected: %v. Commission marked as due.", debitErr)

	} else if debitResp.ShouldFreeze {
		// Débit réussi mais wallet passé en négatif → geler le compte
		logger.Warn().
			Str("shop_id", shopID).
			Int64("new_balance", debitResp.BalanceAfterCents).
			Msg("Wallet went negative after commission debit, freezing account")

		// Marquer la commission comme collectée
		if err := proof.MarkCommissionCollected(); err != nil {
			return nil, fmt.Errorf("failed to mark commission as collected: %w", err)
		}

		response.CommissionStatus = entity.CODCommissionCollected
		response.CommissionCollectedAt = proof.CommissionCollectedAt
		response.WalletTransactionID = &debitResp.TransactionID
		response.WalletBalanceAfter = debitResp.BalanceAfterCents

		// Geler le compte automatiquement
		if uc.freezeUC != nil {
			freezeResp, freezeErr := uc.freezeUC.FreezeForNegativeBalance(ctx, shopID, -debitResp.BalanceAfterCents)
			if freezeErr != nil {
				logger.Error().Err(freezeErr).Msg("Failed to auto-freeze account")
				response.Message = "Commission collected but account freeze failed (manual intervention required)"
			} else {
				response.AccountFrozen = true
				response.FreezeID = freezeResp.FreezeID
				response.AmountDueCents = freezeResp.AmountDueCents
				response.Message = "Commission collected, account frozen due to negative balance"
			}
		}

	} else {
		// Débit réussi, wallet reste positif
		if err := proof.MarkCommissionCollected(); err != nil {
			return nil, fmt.Errorf("failed to mark commission as collected: %w", err)
		}

		response.CommissionStatus = entity.CODCommissionCollected
		response.CommissionCollectedAt = proof.CommissionCollectedAt
		response.WalletTransactionID = &debitResp.TransactionID
		response.WalletBalanceAfter = debitResp.BalanceAfterCents
		response.Message = "Commission collected successfully"
	}

	// 13. Mettre à jour la preuve dans la base
	if err := uc.codProofRepo.WithTX(tx).Update(ctx, proof); err != nil {
		logger.Error().Err(err).Msg("Failed to update COD proof")
		return nil, fmt.Errorf("failed to update COD proof: %w", err)
	}

	// 14. Commit la transaction
	if err := tx.Commit(); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 15. Logger le résultat
	logger.Info().
		Str("order_id", req.OrderID).
		Str("shop_id", shopID).
		Int64("commission_cents", proof.CommissionCents).
		Str("commission_status", string(proof.CommissionStatus)).
		Bool("account_frozen", response.AccountFrozen).
		Msg("Commission collection processed")

	return response, nil
}

// ============================================================
// MÉTHODES UTILITAIRES
// ============================================================

// CollectCommissionForced force la collecte même si incohérent (admin only)
func (uc *CollectCommissionUsecase) CollectCommissionForced(
	ctx context.Context,
	orderID string,
	collectedBy string,
) (*CollectCommissionResponse, error) {
	req := &CollectCommissionRequest{
		OrderID:      orderID,
		CollectedBy:  collectedBy,
		ForceCollect: true,
	}

	return uc.Execute(ctx, req)
}

// RetryCommissionDue retente la collecte d'une commission due
func (uc *CollectCommissionUsecase) RetryCommissionDue(
	ctx context.Context,
	orderID string,
	collectedBy string,
) (*CollectCommissionResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer la preuve
	proof, err := uc.codProofRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("COD proof not found: %w", err)
	}

	// 2. Vérifier que la commission est due
	if proof.CommissionStatus != entity.CODCommissionDue {
		return nil, fmt.Errorf("commission is not in 'due' status, current status: %s", proof.CommissionStatus)
	}

	// 3. Réinitialiser le statut pour permettre la collecte
	proof.CommissionStatus = entity.CODCommissionPending
	proof.Status = entity.CODProofConfirmed

	if err := uc.codProofRepo.Update(ctx, proof); err != nil {
		return nil, fmt.Errorf("failed to reset commission status: %w", err)
	}

	// 4. Logger
	logger.Info().
		Str("order_id", orderID).
		Str("collected_by", collectedBy).
		Msg("Retrying commission collection")

	// 5. Retenter la collecte
	req := &CollectCommissionRequest{
		OrderID:     orderID,
		CollectedBy: collectedBy,
	}

	return uc.Execute(ctx, req)
}

// ListDueCommissions retourne toutes les commissions dues (pour admin)
func (uc *CollectCommissionUsecase) ListDueCommissions(ctx context.Context) ([]*entity.CODProof, error) {
	return uc.codProofRepo.FindCommissionDue(ctx)
}

// SumDueCommissionsByShop somme des commissions dues pour une boutique
func (uc *CollectCommissionUsecase) SumDueCommissionsByShop(ctx context.Context, shopID string) (int64, error) {
	return uc.codProofRepo.SumCommissionDueByShopID(ctx, shopID)
}

// SumTotalDueCommissions somme totale des commissions dues (toutes boutiques)
func (uc *CollectCommissionUsecase) SumTotalDueCommissions(ctx context.Context) (int64, error) {
	return uc.codProofRepo.SumTotalCommissionDue(ctx)
}
