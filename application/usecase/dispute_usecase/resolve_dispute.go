package disputeusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// PaymentRegistry interface pour éviter les dépendances circulaires
type PaymentRegistry interface {
	Get(providerCode entity.PaymentProvider) (payment.Provider, error)
}

type ResolveDisputeUsecase struct {
	disputeRepo     repository.DisputeRepository
	escrowRepo      repository.EscrowAccountRepository
	walletRepo      repository.MerchantWalletRepository
	walletTxnRepo   repository.WalletTransactionRepository
	txManager       repository.TxManager
	paymentRepo     repository.PaymentRepository // 🆕 AJOUTÉ
	paymentRegistry PaymentRegistry              // 🆕 AJOUTÉ
}

func NewResolveDisputeUsecase(
	disputeRepo repository.DisputeRepository,
	escrowRepo repository.EscrowAccountRepository,
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
	txManager repository.TxManager,
	paymentRepo repository.PaymentRepository, // 🆕 AJOUTÉ
	paymentRegistry PaymentRegistry, // 🆕 AJOUTÉ
) *ResolveDisputeUsecase {
	return &ResolveDisputeUsecase{
		disputeRepo:     disputeRepo,
		escrowRepo:      escrowRepo,
		walletRepo:      walletRepo,
		walletTxnRepo:   walletTxnRepo,
		txManager:       txManager,
		paymentRepo:     paymentRepo,
		paymentRegistry: paymentRegistry,
	}
}

type ResolveDisputeRequest struct {
	DisputeID  uuid.UUID
	Resolution string // "merchant_wins" ou "customer_wins"
	Notes      string
	ResolverID uuid.UUID
}

func (uc *ResolveDisputeUsecase) Execute(ctx context.Context, req *ResolveDisputeRequest) (*entity.Dispute, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le litige
	dispute, err := uc.disputeRepo.FindByID(ctx, req.DisputeID)
	if err != nil {
		return nil, fmt.Errorf("dispute not found: %w", err)
	}

	if dispute.Status != entity.DisputeStatusPending && dispute.Status != entity.DisputeStatusUnderReview {
		return nil, fmt.Errorf("dispute is already resolved or cancelled")
	}

	// 2. Récupérer l'Escrow associé
	orderIDStr := dispute.OrderID.String()
	escrow, err := uc.escrowRepo.FindByOrderID(ctx, orderIDStr) // ✅ CORRIGÉ : utilisation de orderIDStr (string)
	if err != nil {
		return nil, fmt.Errorf("escrow not found for dispute order: %w", err)
	}

	// 3. Démarrer une transaction pour garantir l'atomicité des opérations financières
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	escrowRepoTx := uc.escrowRepo.WithTX(tx)
	walletRepoTx := uc.walletRepo.WithTX(tx)
	walletTxnRepoTx := uc.walletTxnRepo.WithTX(tx)

	// 4. Traiter la résolution
	switch req.Resolution {
	case "merchant_wins":
		dispute.Status = entity.DisputeStatusResolvedMerchant

		// a. Libérer les fonds de l'Escrow vers le marchand
		if err := escrow.ReleaseFunds(); err != nil {
			return nil, fmt.Errorf("failed to release escrow: %w", err)
		}
		if err := escrowRepoTx.Update(ctx, escrow); err != nil {
			return nil, fmt.Errorf("failed to update escrow: %w", err)
		}

		// b. Créditer le wallet du marchand
		merchantAmount := escrow.GetMerchantAmount()
		shopIDStr := dispute.ShopID.String()

		wallet, err := walletRepoTx.FindByShopIDForUpdateAdmin(ctx, shopIDStr)
		if err != nil {
			if err.Error() == "merchant wallet not found" {
				wallet = entity.NewMerchantWallet(shopIDStr)
				if err := walletRepoTx.CreateAdmin(ctx, wallet); err != nil {
					return nil, fmt.Errorf("failed to create missing merchant wallet: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to find merchant wallet: %w", err)
			}
		}

		if err := wallet.Credit(merchantAmount); err != nil {
			return nil, fmt.Errorf("failed to credit wallet: %w", err)
		}
		if err := walletRepoTx.UpdateAdmin(ctx, wallet); err != nil {
			return nil, fmt.Errorf("failed to update wallet: %w", err)
		}

		// c. Enregistrer la transaction dans l'historique du wallet
		txnID := uuid.New().String()
		refType := "dispute_resolution"
		desc := fmt.Sprintf("Litige résolu en faveur du marchand (Order: %s)", orderIDStr)

		txn := &entity.WalletTransaction{
			ID:                txnID,
			ShopID:            shopIDStr,
			TransactionType:   entity.WalletTxSaleCredit,
			AmountCents:       merchantAmount,
			BalanceAfterCents: wallet.BalanceCents,
			ReferenceType:     &refType,
			ReferenceID:       func() *string { s := dispute.ID.String(); return &s }(),
			Description:       &desc,
			Status:            entity.WalletTxCompleted,
			CreatedAt:         time.Now().UTC(),
		}

		if err := walletTxnRepoTx.CreateAdmin(ctx, txn); err != nil {
			return nil, fmt.Errorf("failed to create wallet transaction: %w", err)
		}

	case "customer_wins":
		dispute.Status = entity.DisputeStatusResolvedCustomer

		// 🆕 1. Trouver le paiement associé à la commande (unscoped car on est en contexte admin)
		payments, err := uc.paymentRepo.FindByOrderIDUnscoped(ctx, dispute.OrderID)
		if err != nil {
			return nil, fmt.Errorf("failed to find payments for order: %w", err)
		}

		// Trouver le paiement réussi avec une référence provider
		var successPayment *entity.Payment
		for _, p := range payments {
			if p.Status == entity.PaymentStatusSuccess && p.ProviderRef != nil {
				successPayment = p
				break
			}
		}

		if successPayment == nil || successPayment.ProviderRef == nil {
			return nil, fmt.Errorf("no successful payment with provider reference found for this order")
		}

		// 🆕 2. Appeler le provider pour le remboursement (Cash-Out vers le client)
		provider, err := uc.paymentRegistry.Get(successPayment.Provider)
		if err != nil {
			return nil, fmt.Errorf("payment provider not found: %w", err)
		}

		refundAmount := escrow.TotalAmountCents

		// Récupérer le numéro de téléphone du client pour le cash-out
		customerPhone := ""
		if successPayment.CustomerPhone != nil {
			customerPhone = *successPayment.CustomerPhone
		}

		// 🆕 Récupérer l'opérateur utilisé pour le paiement initial (pour le cash-out dynamique)
		operator := ""
		if successPayment.Metadata != nil {
			if op, ok := successPayment.Metadata["operator"].(string); ok {
				operator = op
			}
		}

		// Appel du provider avec l'opérateur dynamique
		if err := provider.Refund(ctx, *successPayment.ProviderRef, refundAmount, customerPhone, operator); err != nil {
			logger.Error().Err(err).Msg("Failed to process refund with provider")
			return nil, fmt.Errorf("failed to process refund with provider: %w", err)
		}

		// 🆕 3. Marquer l'escrow comme remboursé
		escrow.Status = "refunded"
		if err := escrowRepoTx.Update(ctx, escrow); err != nil {
			return nil, fmt.Errorf("failed to update escrow to refunded: %w", err)
		}

		logger.Info().
			Str("order_id", orderIDStr).
			Str("dispute_id", dispute.ID.String()).
			Str("provider_ref", *successPayment.ProviderRef).
			Str("customer_phone", customerPhone).
			Str("operator", operator).
			Int64("refunded_amount", refundAmount).
			Msg("✅ Funds successfully refunded to customer via YengaPay Cash-Out")

	default:
		return nil, fmt.Errorf("invalid resolution: must be 'merchant_wins' or 'customer_wins'")
	}

	// 5. Finaliser le litige
	dispute.ResolutionNotes = &req.Notes
	dispute.UpdatedAt = time.Now().UTC()

	if err := uc.disputeRepo.Update(ctx, dispute); err != nil {
		return nil, fmt.Errorf("failed to update dispute: %w", err)
	}

	// 6. Commit de la transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("dispute_id", dispute.ID.String()).
		Str("resolution", req.Resolution).
		Msg("Dispute resolved successfully")

	return dispute, nil
}
