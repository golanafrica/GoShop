package paymentusecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ProcessTontineWebhookUsecase traite les webhooks YengaPay pour les paiements de tontine
type ProcessTontineWebhookUsecase struct {
	tontinePaymentRepo     repository.TontinePaymentRepository
	tontineGroupRepo       repository.TontineGroupRepository
	tontineParticipantRepo repository.TontineParticipantRepository
	tontineVoucherRepo     repository.TontineVoucherRepository
	shopRepo               repository.ShopRepository

	notificationSvc service.NotificationService
	customerRepo    repository.CustomerRepositoryInterface
}

// NewProcessTontineWebhookUsecase crée une nouvelle instance
func NewProcessTontineWebhookUsecase(
	tontinePaymentRepo repository.TontinePaymentRepository,
	tontineGroupRepo repository.TontineGroupRepository,
	tontineParticipantRepo repository.TontineParticipantRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	shopRepo repository.ShopRepository,
	notificationSvc service.NotificationService,
	customerRepo repository.CustomerRepositoryInterface,
) *ProcessTontineWebhookUsecase {
	return &ProcessTontineWebhookUsecase{
		tontinePaymentRepo:     tontinePaymentRepo,
		tontineGroupRepo:       tontineGroupRepo,
		tontineParticipantRepo: tontineParticipantRepo,
		tontineVoucherRepo:     tontineVoucherRepo,
		shopRepo:               shopRepo,
		notificationSvc:        notificationSvc,
		customerRepo:           customerRepo,
	}
}

// IsTontineReference vérifie si une référence est un paiement tontine
func IsTontineReference(reference string) bool {
	return strings.HasPrefix(reference, "TONTINE:")
}

// ParseTontineReference parse une référence TONTINE:{groupID[:8]}:{cycle}:{participantID[:8]}
func ParseTontineReference(reference string) (string, int, string, error) {
	if !IsTontineReference(reference) {
		return "", 0, "", fmt.Errorf("not a tontine reference: %s", reference)
	}

	parts := strings.Split(reference, ":")
	if len(parts) != 4 {
		return "", 0, "", fmt.Errorf("invalid tontine reference format: %s", reference)
	}

	groupPrefix := parts[1]
	cycleStr := parts[2]
	participantPrefix := parts[3]

	cycleNumber, err := strconv.Atoi(cycleStr)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid cycle number: %s", cycleStr)
	}

	return groupPrefix, cycleNumber, participantPrefix, nil
}

// Execute traite un webhook tontine
func (uc *ProcessTontineWebhookUsecase) Execute(
	ctx context.Context,
	reference string,
	transactionID string,
	status entity.PaymentStatus,
) error {
	logger := zerolog.Ctx(ctx)

	// 1. Parser la référence
	groupPrefix, cycleNumber, participantPrefix, err := ParseTontineReference(reference)
	if err != nil {
		return fmt.Errorf("parse tontine reference: %w", err)
	}

	logger.Info().
		Str("reference", reference).
		Str("group_prefix", groupPrefix).
		Int("cycle", cycleNumber).
		Str("participant_prefix", participantPrefix).
		Str("status", string(status)).
		Msg("Processing tontine webhook")

	// 2. Lookup SANS tenant (référence globale unique)
	payment, err := uc.tontinePaymentRepo.FindByReferenceUnscoped(ctx, reference)
	if err != nil {
		return fmt.Errorf("tontine payment not found for reference %s: %w", reference, err)
	}

	// 3. Déjà payé → idempotent
	if payment.IsDone() {
		logger.Info().
			Str("payment_id", payment.ID).
			Msg("Tontine payment already done, ignoring webhook")
		return nil
	}

	// 4. Récupérer le groupe SANS tenant
	group, err := uc.tontineGroupRepo.FindByIDUnscoped(ctx, payment.GroupID)
	if err != nil {
		return fmt.Errorf("tontine group not found: %w", err)
	}

	shopUUID, err := uuid.Parse(group.ShopID)
	if err != nil {
		return fmt.Errorf("invalid shop ID: %w", err)
	}

	shop, err := uc.shopRepo.FindByID(ctx, shopUUID)
	if err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	// 5. Injecter le tenant pour le reste des opérations
	ctx = tenant.WithTenant(ctx, shop)

	// 6. Transition de statut
	switch status {
	case entity.PaymentStatusSuccess:
		if err := uc.tontinePaymentRepo.MarkDone(ctx, payment.ID, transactionID); err != nil {
			return fmt.Errorf("mark tontine payment done: %w", err)
		}
		logger.Info().
			Str("payment_id", payment.ID).
			Str("transaction_id", transactionID).
			Msg("Tontine payment marked as DONE")

		uc.notifyGroupMembers(ctx, group, cycleNumber, payment.CustomerID, logger)

	case entity.PaymentStatusFailed:
		if err := uc.tontinePaymentRepo.UpdateStatus(ctx, payment.ID, entity.TontinePaymentFailed); err != nil {
			return fmt.Errorf("mark tontine payment failed: %w", err)
		}
		logger.Info().
			Str("payment_id", payment.ID).
			Msg("Tontine payment marked as FAILED")
		return nil

	default:
		logger.Warn().
			Str("status", string(status)).
			Msg("Unknown tontine webhook status, ignoring")
		return nil
	}

	// 7. Si succès → vérifier complétion du cycle
	if status == entity.PaymentStatusSuccess {
		return uc.checkAndCompleteCycle(ctx, group.ID, cycleNumber)
	}

	return nil
}

func (uc *ProcessTontineWebhookUsecase) notifyGroupMembers(
	ctx context.Context,
	group *entity.TontineGroup,
	cycleNumber int,
	payerCustomerID string,
	logger *zerolog.Logger,
) {
	if uc.notificationSvc == nil || uc.customerRepo == nil {
		return
	}

	payerName := "Un membre"
	payerCustomer, err := uc.customerRepo.FindByCustomerID(ctx, payerCustomerID)
	if err == nil && payerCustomer != nil && payerCustomer.FirstName != "" {
		payerName = payerCustomer.FirstName
	}

	amountStr := fmt.Sprintf("%d", group.AmountPerCycleCents/100)
	groupName := fmt.Sprintf("Groupe Tontine (Cycle %d/%d)", cycleNumber, group.TotalCycles)

	participants, err := uc.tontineParticipantRepo.FindByGroupID(ctx, group.ID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to fetch group participants for notification")
		return
	}

	for _, p := range participants {
		customer, err := uc.customerRepo.FindByCustomerID(ctx, p.CustomerID)
		if err != nil || customer == nil || customer.UserID == "" {
			continue
		}

		err = uc.notificationSvc.NotifyTontineCyclePaid(ctx, customer.UserID, payerName, groupName, amountStr)
		if err != nil {
			logger.Warn().Err(err).Str("user_id", customer.UserID).Msg("Failed to send tontine notification")
		}
	}

	logger.Info().Int("notified_count", len(participants)).Str("payer", payerName).Msg("Tontine cycle paid notifications dispatched")
}

func (uc *ProcessTontineWebhookUsecase) checkAndCompleteCycle(
	ctx context.Context,
	groupID string,
	cycleNumber int,
) error {
	logger := zerolog.Ctx(ctx)

	group, err := uc.tontineGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	if !group.IsActive() {
		logger.Warn().
			Str("group_id", groupID).
			Str("status", group.Status).
			Msg("Group is not active, skipping cycle completion")
		return nil
	}

	doneCount, err := uc.tontinePaymentRepo.CountDoneByGroupAndCycle(ctx, groupID, cycleNumber)
	if err != nil {
		return fmt.Errorf("count done payments: %w", err)
	}

	logger.Info().
		Str("group_id", groupID).
		Int("cycle", cycleNumber).
		Int("done_count", doneCount).
		Int("total_cycles", group.TotalCycles).
		Msg("Checking cycle completion")

	if doneCount < group.TotalCycles {
		logger.Info().
			Int("remaining", group.TotalCycles-doneCount).
			Msg("Waiting for remaining participants to pay")
		return nil
	}

	logger.Info().
		Str("group_id", groupID).
		Int("cycle", cycleNumber).
		Msg("All participants paid, generating voucher")

	beneficiary, err := uc.tontineParticipantRepo.FindByPosition(ctx, groupID, cycleNumber)
	if err != nil {
		return fmt.Errorf("beneficiary not found for cycle %d: %w", cycleNumber, err)
	}

	voucherCode, err := generateVoucherCode(12)
	if err != nil {
		return fmt.Errorf("generate voucher code: %w", err)
	}

	voucher, err := entity.NewTontineVoucher(
		groupID,
		beneficiary.ID,
		beneficiary.CustomerID,
		group.ProductID,
		group.ShopID,
		voucherCode,
		cycleNumber,
	)
	if err != nil {
		return fmt.Errorf("create voucher entity: %w", err)
	}

	if err := uc.tontineVoucherRepo.Create(ctx, voucher); err != nil {
		return fmt.Errorf("save voucher: %w", err)
	}

	logger.Info().
		Str("voucher_code", voucherCode).
		Str("beneficiary_customer_id", beneficiary.CustomerID).
		Int("cycle", cycleNumber).
		Msg("✅ Voucher generated for beneficiary")

	if group.IsLastCycle() {
		if err := uc.tontineGroupRepo.Complete(ctx, groupID); err != nil {
			return fmt.Errorf("complete group: %w", err)
		}
		logger.Info().
			Str("group_id", groupID).
			Msg("🎉 Tontine group completed!")
		return nil
	}

	if err := uc.tontineGroupRepo.IncrementCycle(ctx, groupID); err != nil {
		return fmt.Errorf("increment cycle: %w", err)
	}

	logger.Info().
		Str("group_id", groupID).
		Int("new_cycle", cycleNumber+1).
		Msg("Moved to next cycle")

	return nil
}

func generateVoucherCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes))[:length], nil
}
