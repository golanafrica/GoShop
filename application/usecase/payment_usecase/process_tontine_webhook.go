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

// TontineWalletCreditor abstrait le crédit wallet
type TontineWalletCreditor interface {
	CreditFromTontine(ctx context.Context, shopID string, amountCents int64, groupID string, cycleNumber int) (*walletusecase.CreditWalletResponse, error)
}

// ProcessTontineWebhookUsecase traite les webhooks YengaPay pour les paiements de tontine
type ProcessTontineWebhookUsecase struct {
	tontinePaymentRepo     repository.TontinePaymentRepository
	tontineGroupRepo       repository.TontineGroupRepository
	tontineParticipantRepo repository.TontineParticipantRepository
	tontineVoucherRepo     repository.TontineVoucherRepository
	shopRepo               repository.ShopRepository
	txManager              repository.TxManager // 🛡️ v4.11.0 : Pour FOR UPDATE

	notificationSvc    service.NotificationService
	customerRepo       repository.CustomerRepositoryInterface
	walletCreditor     TontineWalletCreditor
	commissionRateRepo repository.CommissionRateRepository
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
	txManager repository.TxManager, // 🛡️ v4.11.0 : Ajouté
) *ProcessTontineWebhookUsecase {
	return &ProcessTontineWebhookUsecase{
		tontinePaymentRepo:     tontinePaymentRepo,
		tontineGroupRepo:       tontineGroupRepo,
		tontineParticipantRepo: tontineParticipantRepo,
		tontineVoucherRepo:     tontineVoucherRepo,
		shopRepo:               shopRepo,
		txManager:              txManager,
		notificationSvc:        notificationSvc,
		customerRepo:           customerRepo,
	}
}

func (uc *ProcessTontineWebhookUsecase) WithWalletCreditor(c TontineWalletCreditor) *ProcessTontineWebhookUsecase {
	uc.walletCreditor = c
	return uc
}

func (uc *ProcessTontineWebhookUsecase) WithCommissionRateRepo(r repository.CommissionRateRepository) *ProcessTontineWebhookUsecase {
	uc.commissionRateRepo = r
	return uc
}

func IsTontineReference(reference string) bool {
	return strings.HasPrefix(reference, "TONTINE:")
}

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

func tontineCircleToTransactionType(circleType string) string {
	switch circleType {
	case entity.TontineCircleCommercial:
		return entity.TransactionTypeTontineCommercial
	case entity.TontineCircleCorporate:
		return entity.TransactionTypeTontineCorporate
	case entity.TontineCircleFamily:
		return entity.TransactionTypeTontineFamily
	default:
		return entity.TransactionTypeTontineGroup
	}
}

func tontineCycleRateBps(circleType string) int {
	switch circleType {
	case entity.TontineCircleCommercial:
		return entity.DefaultRateTontineCommercial
	case entity.TontineCircleCorporate:
		return entity.DefaultRateTontineCorporate
	case entity.TontineCircleFamily:
		return entity.DefaultRateTontineFamily
	default:
		return entity.DefaultRateTontineGroup
	}
}

func (uc *ProcessTontineWebhookUsecase) resolveTontineRateBps(
	ctx context.Context,
	shopID string,
	circleType string,
	logger *zerolog.Logger,
) int {
	fallback := tontineCycleRateBps(circleType)
	if uc.commissionRateRepo == nil {
		return fallback
	}

	txType := tontineCircleToTransactionType(circleType)
	rate, err := uc.commissionRateRepo.FindByShopAndType(ctx, shopID, txType)
	if err != nil || rate == nil || !rate.IsActive {
		logger.Debug().
			Str("shop_id", shopID).
			Str("tx_type", txType).
			Int("fallback_bps", fallback).
			Msg("Phase 4: no active shop commission rate — using platform default")
		return fallback
	}

	logger.Info().
		Str("shop_id", shopID).
		Str("tx_type", txType).
		Int("rate_bps", rate.RateBps).
		Msg("Phase 4: using shop-specific commission rate")
	return rate.RateBps
}

// Execute traite un webhook tontine avec protection anti-race condition
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

	// ============================================================
	// 🛡️ v4.11.0 : ANTI-RACE CONDITION
	// Démarrer une transaction et verrouiller le paiement avec FOR UPDATE
	// pour éviter les doubles webhooks simultanés
	// ============================================================
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	tontinePaymentRepoTx := uc.tontinePaymentRepo.WithTX(tx)

	// Verrou pessimiste : FOR UPDATE bloque les autres webhooks simultanés
	payment, err := tontinePaymentRepoTx.FindByReferenceUnscopedForUpdate(ctx, reference)
	if err != nil {
		return fmt.Errorf("tontine payment not found for reference %s: %w", reference, err)
	}

	// Double-check après acquisition du verrou (idempotence)
	if payment.IsDone() {
		logger.Info().Str("payment_id", payment.ID).Msg("Tontine payment already done, ignoring webhook")
		return nil
	}

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

	ctx = tenant.WithTenant(ctx, shop)

	switch status {
	case entity.PaymentStatusSuccess:
		if err := tontinePaymentRepoTx.MarkDone(ctx, payment.ID, transactionID); err != nil {
			return fmt.Errorf("mark tontine payment done: %w", err)
		}

		// Commit le changement de statut AVANT les opérations lourdes
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		logger.Info().
			Str("payment_id", payment.ID).
			Str("transaction_id", transactionID).
			Msg("Tontine payment marked as DONE")

		uc.notifyGroupMembers(ctx, group, cycleNumber, payment.CustomerID, logger)

	case entity.PaymentStatusFailed:
		if err := tontinePaymentRepoTx.UpdateStatus(ctx, payment.ID, entity.TontinePaymentFailed); err != nil {
			return fmt.Errorf("mark tontine payment failed: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		logger.Info().Str("payment_id", payment.ID).Msg("Tontine payment marked as FAILED")
		return nil

	default:
		logger.Warn().Str("status", string(status)).Msg("Unknown tontine webhook status, ignoring")
		return nil
	}

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

	grossCents := group.AmountPerCycleCents * int64(group.TotalCycles)
	rateBps := uc.resolveTontineRateBps(ctx, group.ShopID, group.CircleType, logger)
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
		Msg("Tontine cycle settlement amounts (Phase 5: shop-aware rate + held on voucher)")

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

	voucher.HeldAmountCents = netCents

	if err := uc.tontineVoucherRepo.Create(ctx, voucher); err != nil {
		return fmt.Errorf("save voucher: %w", err)
	}

	logger.Info().
		Str("voucher_code", voucherCode).
		Str("beneficiary_customer_id", beneficiary.CustomerID).
		Int("cycle", cycleNumber).
		Int64("held_amount_cents", voucher.HeldAmountCents).
		Msg("✅ Voucher generated for beneficiary")

	if uc.walletCreditor != nil && netCents > 0 {
		if _, err := uc.walletCreditor.CreditFromTontine(ctx, group.ShopID, netCents, groupID, cycleNumber); err != nil {
			logger.Error().Err(err).
				Str("shop_id", group.ShopID).
				Int64("net_cents", netCents).
				Msg("Failed to credit merchant wallet from tontine cycle (net)")
		} else {
			logger.Info().
				Str("shop_id", group.ShopID).
				Int64("gross_cents", grossCents).
				Int64("commission_cents", commissionCents).
				Int64("net_cents", netCents).
				Msg("💰 Merchant wallet credited NET from tontine cycle (held until redeem)")
		}
	}

	uc.markCycleCommissionsCollected(ctx, groupID, cycleNumber, commissionCents, logger)
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

func (uc *ProcessTontineWebhookUsecase) markCycleCommissionsCollected(
	ctx context.Context,
	groupID string,
	cycleNumber int,
	_ int64,
	logger *zerolog.Logger,
) {
	payments, err := uc.tontinePaymentRepo.FindByGroupAndCycle(ctx, groupID, cycleNumber)
	if err != nil {
		logger.Warn().Err(err).Msg("Phase 3: could not list cycle payments for commission mark")
		return
	}

	for _, p := range payments {
		if p == nil || !p.IsDone() {
			continue
		}
		if err := uc.tontinePaymentRepo.UpdateTontineCommissionStatus(
			ctx, p.ID, entity.CommissionStatusCollected, nil,
		); err != nil {
			logger.Warn().Err(err).Str("payment_id", p.ID).Msg("Phase 3: failed to mark payment commission collected")
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
