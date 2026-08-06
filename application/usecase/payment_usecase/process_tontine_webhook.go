package paymentusecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TontineWalletCreditor abstrait le crédit wallet (évite import circulaire usecase→usecase)
// La signature correspond exactement à celle de walletusecase.CreditWalletUsecase.CreditFromTontine
type TontineWalletCreditor interface {
	CreditFromTontine(ctx context.Context, shopID string, amountCents int64, groupID string) (*walletusecase.CreditWalletResponse, error)
}

// ProcessTontineWebhookUsecase traite les webhooks YengaPay pour les paiements de tontine
type ProcessTontineWebhookUsecase struct {
	tontinePaymentRepo     repository.TontinePaymentRepository
	tontineGroupRepo       repository.TontineGroupRepository
	tontineParticipantRepo repository.TontineParticipantRepository
	tontineVoucherRepo     repository.TontineVoucherRepository
	shopRepo               repository.ShopRepository

	notificationSvc service.NotificationService
	customerRepo    repository.CustomerRepositoryInterface
	walletCreditor  TontineWalletCreditor // optionnel (nil-safe)
}

// NewProcessTontineWebhookUsecase crée une nouvelle instance
// La signature reste inchangée pour ne pas casser app.go et les tests existants.
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

// WithWalletCreditor injecte le crédit wallet via un pattern fluent (après New)
func (uc *ProcessTontineWebhookUsecase) WithWalletCreditor(c TontineWalletCreditor) *ProcessTontineWebhookUsecase {
	uc.walletCreditor = c
	return uc
}

// IsTontineReference vérifie si une référence est un paiement tontine
func IsTontineReference(reference string) bool {
	return strings.HasPrefix(reference, "TONTINE:")
}

// ParseTontineReference parse TONTINE:{groupID[:8]}:{cycle}:{participantID[:8]}
func ParseTontineReference(reference string) (string, int, string, error) {
	if !IsTontineReference(reference) {
		return "", 0, "", fmt.Errorf("not a tontine reference: %s", reference)
	}

	parts := strings.Split(reference, ":")
	if len(parts) != 4 {
		return "", 0, "", fmt.Errorf("invalid tontine reference format: %s", reference)
	}

	groupPrefix := parts[1]
	cycleNumber, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid cycle number: %s", parts[2])
	}

	return groupPrefix, cycleNumber, parts[3], nil
}

// tontineCycleRateBps mappe le type de cercle vers le taux plateforme (bps).
// Aligné sur domain/entity/commission_rate.go + migration 044.
func tontineCycleRateBps(circleType string) int {
	switch circleType {
	case entity.TontineCircleCommercial:
		return entity.DefaultRateTontineCommercial // 200 = 2.0%
	case entity.TontineCircleCorporate:
		return entity.DefaultRateTontineCorporate // 150 = 1.5%
	case entity.TontineCircleFamily:
		return entity.DefaultRateTontineFamily // 150 = 1.5%
	default:
		return entity.DefaultRateTontineGroup // 150
	}
}

// Execute traite un webhook tontine
func (uc *ProcessTontineWebhookUsecase) Execute(
	ctx context.Context,
	reference string,
	transactionID string,
	status entity.PaymentStatus,
) error {
	logger := zerolog.Ctx(ctx)

	_, cycleNumber, _, err := ParseTontineReference(reference)
	if err != nil {
		return fmt.Errorf("parse tontine reference: %w", err)
	}

	logger.Info().
		Str("reference", reference).
		Int("cycle", cycleNumber).
		Str("status", string(status)).
		Msg("Processing tontine webhook")

	// 1. Lookup SANS tenant (référence globale unique)
	payment, err := uc.tontinePaymentRepo.FindByReferenceUnscoped(ctx, reference)
	if err != nil {
		return fmt.Errorf("tontine payment not found for reference %s: %w", reference, err)
	}

	// 2. Déjà payé → idempotent
	if payment.IsDone() {
		logger.Info().Str("payment_id", payment.ID).Msg("Tontine payment already done, ignoring webhook")
		return nil
	}

	// 3. Récupérer le groupe SANS tenant
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

	// 4. Injecter le tenant pour le reste des opérations
	ctx = tenant.WithTenant(ctx, shop)

	// 5. Transition de statut
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
		logger.Info().Str("payment_id", payment.ID).Msg("Tontine payment marked as FAILED")
		return nil

	default:
		logger.Warn().Str("status", string(status)).Msg("Unknown tontine webhook status, ignoring")
		return nil
	}

	// 6. Si succès → vérifier complétion du cycle
	if status == entity.PaymentStatusSuccess {
		return uc.checkAndCompleteCycle(ctx, group, cycleNumber, shop)
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
		if err := uc.notificationSvc.NotifyTontineCyclePaid(ctx, customer.UserID, payerName, groupName, amountStr); err != nil {
			logger.Warn().Err(err).Str("user_id", customer.UserID).Msg("Failed to send tontine notification")
		}
	}

	logger.Info().Int("notified_count", len(participants)).Str("payer", payerName).Msg("Tontine cycle paid notifications dispatched")
}

func (uc *ProcessTontineWebhookUsecase) checkAndCompleteCycle(
	ctx context.Context,
	group *entity.TontineGroup,
	cycleNumber int,
	shop *entity.Shop,
) error {
	logger := zerolog.Ctx(ctx)
	groupID := group.ID

	// Recharger le groupe sous tenant
	freshGroup, err := uc.tontineGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}
	group = freshGroup

	if !group.IsActive() {
		logger.Warn().Str("group_id", groupID).Str("status", group.Status).Msg("Group is not active, skipping cycle completion")
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
		logger.Info().Int("remaining", group.TotalCycles-doneCount).Msg("Waiting for remaining participants to pay")
		return nil
	}

	logger.Info().Str("group_id", groupID).Int("cycle", cycleNumber).Msg("All participants paid, generating voucher")

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

	// ── Phase 1.1 : crédit NET (commission 1× sur le TOTAL du cycle) ─────────
	// gross = somme des cotisations du cycle = prix du bien (modèle N membres = N cycles)
	grossCents := group.AmountPerCycleCents * int64(group.TotalCycles)
	rateBps := tontineCycleRateBps(group.CircleType)
	commissionCents := entity.CalculateCommission(grossCents, rateBps)
	netCents := grossCents - commissionCents
	if netCents < 0 {
		netCents = 0
	}

	logger.Info().
		Str("group_id", groupID).
		Int("cycle", cycleNumber).
		Str("circle_type", group.CircleType).
		Int("rate_bps", rateBps).
		Int64("gross_cents", grossCents).
		Int64("commission_cents", commissionCents).
		Int64("net_cents", netCents).
		Msg("Tontine cycle settlement amounts (Phase 1: net credit)")

	if uc.walletCreditor != nil && netCents > 0 {
		if _, err := uc.walletCreditor.CreditFromTontine(ctx, group.ShopID, netCents, groupID); err != nil {
			logger.Error().Err(err).
				Str("shop_id", group.ShopID).
				Int64("net_cents", netCents).
				Msg("Failed to credit merchant wallet from tontine cycle (net)")
			// On ne bloque pas le cycle : le voucher est déjà créé.
		} else {
			logger.Info().
				Str("shop_id", group.ShopID).
				Int64("gross_cents", grossCents).
				Int64("commission_cents", commissionCents).
				Int64("net_cents", netCents).
				Msg("💰 Merchant wallet credited NET from tontine cycle (platform commission withheld)")
		}
	}

	// Marquer les cotisations du cycle comme commission déjà prise (évite double débit scheduler)
	// Si la méthode n'existe pas encore / échoue : Phase 1.2 (skip scheduler) reste obligatoire.
	uc.markCycleCommissionsCollected(ctx, groupID, cycleNumber, commissionCents, logger)

	// Notifs : montant net crédité marchand (FCFA)
	uc.notifyBeneficiaryAndMerchant(ctx, group, beneficiary, voucherCode, netCents, cycleNumber, shop, logger)

	if group.IsLastCycle() {
		if err := uc.tontineGroupRepo.Complete(ctx, groupID); err != nil {
			return fmt.Errorf("complete group: %w", err)
		}
		logger.Info().Str("group_id", groupID).Msg("🎉 Tontine group completed!")
		return nil
	}

	if err := uc.tontineGroupRepo.IncrementCycle(ctx, groupID); err != nil {
		return fmt.Errorf("increment cycle: %w", err)
	}

	logger.Info().Str("group_id", groupID).Int("new_cycle", cycleNumber+1).Msg("Moved to next cycle")
	return nil
}

// markCycleCommissionsCollected tente de marquer les paiements DONE du cycle
// comme commission déjà collectée (via net credit). Best-effort.
func (uc *ProcessTontineWebhookUsecase) markCycleCommissionsCollected(
	ctx context.Context,
	groupID string,
	cycleNumber int,
	_ int64, // commission cycle totale (pour logs futurs / settlement)
	logger *zerolog.Logger,
) {
	// L’interface repo expose typiquement UpdateCommissionStatus(paymentID, status, cents).
	// On itère les paiements du cycle si FindByGroupAndCycle est disponible.
	type cycleLister interface {
		FindByGroupAndCycle(ctx context.Context, groupID string, cycleNumber int) ([]*entity.TontinePayment, error)
	}
	type commissionUpdater interface {
		UpdateCommissionStatus(ctx context.Context, paymentID string, status string, commissionCents int64) error
	}

	lister, okL := uc.tontinePaymentRepo.(cycleLister)
	updater, okU := uc.tontinePaymentRepo.(commissionUpdater)
	if !okL || !okU {
		logger.Debug().Msg("Phase 1: repo does not support cycle commission mark — rely on scheduler skip (Phase 1.2)")
		return
	}

	payments, err := lister.FindByGroupAndCycle(ctx, groupID, cycleNumber)
	if err != nil {
		logger.Warn().Err(err).Msg("Phase 1: could not list cycle payments for commission mark")
		return
	}

	for _, p := range payments {
		if p == nil || !p.IsDone() {
			continue
		}
		// commission_cents ligne peut rester informatif ; statut = collected pour le scheduler
		cents := p.CommissionCents
		if err := updater.UpdateCommissionStatus(ctx, p.ID, entity.CommissionStatusCollected, cents); err != nil {
			logger.Warn().Err(err).Str("payment_id", p.ID).Msg("Phase 1: failed to mark payment commission collected")
		}
	}
}

func (uc *ProcessTontineWebhookUsecase) notifyBeneficiaryAndMerchant(
	ctx context.Context,
	group *entity.TontineGroup,
	beneficiary *entity.TontineParticipant,
	voucherCode string,
	amountCents int64,
	cycleNumber int,
	shop *entity.Shop,
	logger *zerolog.Logger,
) {
	if uc.notificationSvc == nil {
		return
	}

	amountStr := fmt.Sprintf("%d", amountCents/100)
	groupName := fmt.Sprintf("Groupe Tontine (Cycle %d/%d)", cycleNumber, group.TotalCycles)

	// 1. Bénéficiaire : voucher prêt (montant = net cycle / équivalent bien côté métier)
	if uc.customerRepo != nil {
		beneficiaryCustomer, err := uc.customerRepo.FindByCustomerID(ctx, beneficiary.CustomerID)
		if err != nil || beneficiaryCustomer == nil || beneficiaryCustomer.UserID == "" {
			logger.Warn().Err(err).Str("customer_id", beneficiary.CustomerID).Msg("Failed to resolve beneficiary for voucher notification")
		} else {
			if err := uc.notificationSvc.NotifyTontineVoucherReady(ctx, beneficiaryCustomer.UserID, groupName, voucherCode, amountStr); err != nil {
				logger.Warn().Err(err).Str("user_id", beneficiaryCustomer.UserID).Msg("Failed to send voucher-ready notification to beneficiary")
			} else {
				logger.Info().Str("user_id", beneficiaryCustomer.UserID).Str("voucher", voucherCode).Msg("Beneficiary notified (voucher issued)")
			}
		}
	}

	// 2. Marchand : cycle soldé, crédit NET
	if shop != nil && shop.OwnerID != "" {
		if err := uc.notificationSvc.NotifyTontineMerchantCycleCompleted(ctx, shop.OwnerID, groupName, amountStr, voucherCode); err != nil {
			logger.Warn().Err(err).Str("owner_id", shop.OwnerID).Msg("Failed to send cycle-completed notification to merchant")
		} else {
			logger.Info().Str("owner_id", shop.OwnerID).Msg("Merchant notified (cycle completed + net wallet credit)")
		}
	}
}

func generateVoucherCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes))[:length], nil
}
