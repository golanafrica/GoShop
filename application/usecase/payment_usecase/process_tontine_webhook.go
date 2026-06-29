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
}

// NewProcessTontineWebhookUsecase crée une nouvelle instance
func NewProcessTontineWebhookUsecase(
	tontinePaymentRepo repository.TontinePaymentRepository,
	tontineGroupRepo repository.TontineGroupRepository,
	tontineParticipantRepo repository.TontineParticipantRepository,
	tontineVoucherRepo repository.TontineVoucherRepository,
	shopRepo repository.ShopRepository,
) *ProcessTontineWebhookUsecase {
	return &ProcessTontineWebhookUsecase{
		tontinePaymentRepo:     tontinePaymentRepo,
		tontineGroupRepo:       tontineGroupRepo,
		tontineParticipantRepo: tontineParticipantRepo,
		tontineVoucherRepo:     tontineVoucherRepo,
		shopRepo:               shopRepo,
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

		// 2. Trouver le paiement tontine par préfixe de référence
		// ✅ APRÈS (match exact, plus simple)
	payment, err := uc.tontinePaymentRepo.FindByReference(ctx, reference)
	if err != nil {
		return fmt.Errorf("tontine payment not found for reference %s: %w", reference, err)
	}

	// 3. Vérifier que le paiement n'est pas déjà dans un état terminal
	if payment.IsDone() {
		logger.Info().
			Str("payment_id", payment.ID).
			Msg("Tontine payment already done, ignoring webhook")
		return nil
	}

	// 4. Récupérer le groupe pour injecter le tenant
	group, err := uc.tontineGroupRepo.FindByID(ctx, payment.GroupID)
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

	// Injecter le tenant dans le contexte
	ctx = tenant.WithTenant(ctx, shop)

	// 5. Transition de statut selon l'événement
	switch status {
	case entity.PaymentStatusSuccess:
		if err := uc.tontinePaymentRepo.MarkDone(ctx, payment.ID, transactionID); err != nil {
			return fmt.Errorf("mark tontine payment done: %w", err)
		}
		logger.Info().
			Str("payment_id", payment.ID).
			Str("transaction_id", transactionID).
			Msg("Tontine payment marked as DONE")

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

	// 6. Si succès, vérifier si tous les participants ont payé pour ce cycle
	if status == entity.PaymentStatusSuccess {
		return uc.checkAndCompleteCycle(ctx, group.ID, cycleNumber)
	}

	return nil
}

// checkAndCompleteCycle vérifie si tous les participants ont payé et complète le cycle
func (uc *ProcessTontineWebhookUsecase) checkAndCompleteCycle(
	ctx context.Context,
	groupID string,
	cycleNumber int,
) error {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le groupe
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

	// 2. Compter les paiements DONE pour ce cycle
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

	// 3. Pas encore tous payés → on attend
	if doneCount < group.TotalCycles {
		logger.Info().
			Int("remaining", group.TotalCycles-doneCount).
			Msg("Waiting for remaining participants to pay")
		return nil
	}

	// 4. Tous ont payé ! Générer le voucher pour le bénéficiaire du cycle
	logger.Info().
		Str("group_id", groupID).
		Int("cycle", cycleNumber).
		Msg("All participants paid, generating voucher")

	// Trouver le bénéficiaire du cycle (position = cycleNumber)
	beneficiary, err := uc.tontineParticipantRepo.FindByPosition(ctx, groupID, cycleNumber)
	if err != nil {
		return fmt.Errorf("beneficiary not found for cycle %d: %w", cycleNumber, err)
	}

	// Générer un code voucher unique
	voucherCode, err := generateVoucherCode(12)
	if err != nil {
		return fmt.Errorf("generate voucher code: %w", err)
	}

	// Créer le voucher
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

	// 5. Si c'est le dernier cycle, compléter le groupe
	if group.IsLastCycle() {
		if err := uc.tontineGroupRepo.Complete(ctx, groupID); err != nil {
			return fmt.Errorf("complete group: %w", err)
		}
		logger.Info().
			Str("group_id", groupID).
			Msg("🎉 Tontine group completed!")
		return nil
	}

	// 6. Sinon, passer au cycle suivant
	if err := uc.tontineGroupRepo.IncrementCycle(ctx, groupID); err != nil {
		return fmt.Errorf("increment cycle: %w", err)
	}

	logger.Info().
		Str("group_id", groupID).
		Int("new_cycle", cycleNumber+1).
		Msg("Moved to next cycle")

	return nil
}

// generateVoucherCode génère un code alphanumérique unique de 12 caractères
func generateVoucherCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes))[:length], nil
}
