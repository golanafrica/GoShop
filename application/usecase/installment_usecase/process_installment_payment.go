package installmentusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"

	"github.com/rs/zerolog/log"
)

type ProcessInstallmentPaymentUsecase struct {
	txManager       repository.TxManager
	installmentRepo repository.OrderInstallmentRepository
	walletRepo      repository.MerchantWalletRepository
	orderRepo       repository.OrderRepository
	notifService    service.NotificationService
}

func NewProcessInstallmentPaymentUsecase(
	txManager repository.TxManager,
	installmentRepo repository.OrderInstallmentRepository,
	walletRepo repository.MerchantWalletRepository,
	orderRepo repository.OrderRepository,
	notifService service.NotificationService,
) *ProcessInstallmentPaymentUsecase {
	return &ProcessInstallmentPaymentUsecase{
		txManager:       txManager,
		installmentRepo: installmentRepo,
		walletRepo:      walletRepo,
		orderRepo:       orderRepo,
		notifService:    notifService,
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
		return nil
	}

	if targetInst.AmountCents != req.AmountCents {
		return fmt.Errorf("montant invalide: attendu %d, reçu %d", targetInst.AmountCents, req.AmountCents)
	}

	if err := uc.installmentRepo.MarkAsPaid(ctx, targetInst.ID, req.PaymentRef); err != nil {
		return fmt.Errorf("échec de la mise à jour de la tranche: %w", err)
	}

	wallet, err := uc.walletRepo.FindByShopIDForUpdate(ctx, req.ShopID)
	if err != nil {
		return fmt.Errorf("portefeuille marchand introuvable: %w", err)
	}

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

	// 4. 🆕 Vérifier si TOUTES les tranches sont maintenant payées
	allPaid := true
	for _, inst := range installments {
		if inst.ID == targetInst.ID {
			continue
		}
		if !inst.IsPaid() {
			allPaid = false
			break
		}
	}

	if allPaid {
		go uc.notifyMerchantAllPaid(context.Background(), req.OrderID, req.ShopID)
	}

	return nil
}

// 🆕 notifyMerchantAllPaid envoie une notification au marchand
func (uc *ProcessInstallmentPaymentUsecase) notifyMerchantAllPaid(ctx context.Context, orderID, shopID string) {
	if uc.notifService == nil {
		return
	}

	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		log.Error().Err(err).Str("order_id", orderID).Msg("Failed to fetch order for notification")
		return
	}

	delayDays := 7
	if order.InstallmentReleaseDelayDays > 0 {
		delayDays = order.InstallmentReleaseDelayDays
	}

	releaseDate := "N/A"
	if order.DeliveredAt != nil {
		releaseDate = order.DeliveredAt.AddDate(0, 0, delayDays).Format("02/01/2006")
	} else {
		releaseDate = order.CreatedAt.AddDate(0, 0, 30).Format("02/01/2006")
	}

	subject := fmt.Sprintf("🎉 Toutes les tranches payées - Commande #%s", orderID[:8])
	message := fmt.Sprintf(
		"Toutes les tranches de la commande #%s sont payées. Montant total : %d FCFA. Libération prévue le %s (dans %d jours).",
		orderID[:8], order.TotalCents, releaseDate, delayDays,
	)

	// ✅ Utilisation réelle du NotificationService via la méthode générique SendNotification
	req := &service.NotificationRequest{
		Type:    "installment_all_paid",
		ShopID:  shopID,
		OrderID: orderID,
		Data: map[string]interface{}{
			"subject": subject,
			"message": message,
		},
	}

	if err := uc.notifService.SendNotification(ctx, req); err != nil {
		log.Warn().Err(err).Str("order_id", orderID).Msg("Failed to send merchant installment notification")
	} else {
		log.Info().
			Str("order_id", orderID).
			Str("shop_id", shopID).
			Msg("📧 Merchant notification sent successfully: all installments paid")
	}
}
