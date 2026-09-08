package installmentusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog/log"
)

type ProcessInstallmentPaymentUsecase struct {
	txManager       repository.TxManager
	installmentRepo repository.OrderInstallmentRepository
	walletRepo      repository.MerchantWalletRepository
}

func NewProcessInstallmentPaymentUsecase(
	txManager repository.TxManager,
	installmentRepo repository.OrderInstallmentRepository,
	walletRepo repository.MerchantWalletRepository,
) *ProcessInstallmentPaymentUsecase {
	return &ProcessInstallmentPaymentUsecase{
		txManager:       txManager,
		installmentRepo: installmentRepo,
		walletRepo:      walletRepo,
	}
}

type ProcessPaymentRequest struct {
	OrderID     string
	TrancheNum  int
	PaymentRef  string
	AmountCents int64
	ShopID      string
}

func (uc *ProcessInstallmentPaymentUsecase) Execute(ctx context.Context, req ProcessPaymentRequest) error {
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Récupérer la tranche spécifique
	installments, err := uc.installmentRepo.GetByOrderID(ctx, req.OrderID)
	if err != nil {
		return fmt.Errorf("échec de la récupération des tranches: %w", err)
	}

	var targetInst *entity.OrderInstallment
	for _, inst := range installments {
		if inst.TrancheNumber == req.TrancheNum {
			targetInst = inst
			break
		}
	}

	if targetInst == nil {
		return fmt.Errorf("tranche %d introuvable pour la commande %s", req.TrancheNum, req.OrderID)
	}

	if targetInst.IsPaid() {
		log.Warn().Str("order_id", req.OrderID).Msg("Installment already paid (idempotent)")
		return nil // Idempotence
	}

	if targetInst.AmountCents != req.AmountCents {
		return fmt.Errorf("montant invalide: attendu %d, reçu %d", targetInst.AmountCents, req.AmountCents)
	}

	// 2. Marquer la tranche comme payée
	if err := uc.installmentRepo.MarkAsPaid(ctx, targetInst.ID, req.PaymentRef); err != nil {
		return fmt.Errorf("échec de la mise à jour de la tranche: %w", err)
	}

	// 3. BLOQUER LES FONDS (Escrow / Held Balance)
	// On récupère le wallet avec un verrouillage FOR UPDATE
	wallet, err := uc.walletRepo.FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		return fmt.Errorf("portefeuille marchand introuvable: %w", err)
	}

	// On crédite d'abord le balance (si ce n'est pas déjà fait par le webhook principal)
	// Puis on "Hold" le montant pour le mettre en séquestre
	if err := wallet.Hold(req.AmountCents); err != nil {
		return fmt.Errorf("échec du blocage des fonds en séquestre: %w", err)
	}

	if err := uc.walletRepo.Update(ctx, wallet); err != nil {
		return fmt.Errorf("échec de la mise à jour du portefeuille: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("échec du commit de la transaction de paiement: %w", err)
	}

	log.Info().Str("order_id", req.OrderID).Int("tranche", req.TrancheNum).Int64("amount", req.AmountCents).Msg("Installment paid and funds held in escrow")
	return nil
}
