package tontineusecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type PayCycleUsecase struct {
	groupRepo       repository.TontineGroupRepository
	participantRepo repository.TontineParticipantRepository
	paymentRepo     repository.TontinePaymentRepository
	rateRepo        repository.CommissionRateRepository // 🆕 Pour le taux exact par type de cercle
	txManager       repository.TxManager
	paymentRegistry paymentusecase.PaymentRegistry
}

func NewPayCycleUsecase(
	groupRepo repository.TontineGroupRepository,
	participantRepo repository.TontineParticipantRepository,
	paymentRepo repository.TontinePaymentRepository,
	rateRepo repository.CommissionRateRepository, // 🆕 Injection du rateRepo
	txManager repository.TxManager,
	paymentRegistry paymentusecase.PaymentRegistry,
) *PayCycleUsecase {
	return &PayCycleUsecase{
		groupRepo:       groupRepo,
		participantRepo: participantRepo,
		paymentRepo:     paymentRepo,
		rateRepo:        rateRepo,
		txManager:       txManager,
		paymentRegistry: paymentRegistry,
	}
}

type PayCycleRequest struct {
	GroupID     string `json:"group_id"`
	CustomerID  string `json:"customer_id"`
	Operator    string `json:"operator"`
	PhoneNumber string `json:"phone_number"`
	Flow        string `json:"flow"`
	OTP         string `json:"otp,omitempty"`
}

func (r *PayCycleRequest) Validate() error {
	if r.GroupID == "" {
		return fmt.Errorf("group_id is required")
	}
	if r.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if r.Operator == "" {
		return fmt.Errorf("operator is required")
	}
	if r.PhoneNumber == "" {
		return fmt.Errorf("phone_number is required")
	}
	if r.Flow == "" {
		r.Flow = "indirect"
	}
	if r.Flow != "indirect" && r.Flow != "direct" {
		return fmt.Errorf("flow must be 'indirect' or 'direct'")
	}

	op := strings.ToLower(strings.TrimSpace(r.Operator))
	if op == "yenga_pay" || op == "yengapay" {
		return fmt.Errorf("operator must be a mobile money channel, not yenga_pay")
	}

	if r.OTP != "" {
		otp := strings.TrimSpace(r.OTP)
		if len(otp) < 4 || len(otp) > 10 {
			return fmt.Errorf("otp must be between 4 and 10 characters")
		}
		r.OTP = otp
	}
	return nil
}

type PayCycleResponse struct {
	TontinePaymentID string `json:"tontine_payment_id"`
	AmountCents      int64  `json:"amount_cents"`
	CommissionCents  int64  `json:"commission_cents"`
	NetAmountCents   int64  `json:"net_amount_cents"`
	CycleNumber      int    `json:"cycle_number"`
	Status           string `json:"status"`
	ProviderRef      string `json:"provider_ref,omitempty"`
	YengaIntentID    string `json:"yenga_intent_id,omitempty"`
	USSDCode         string `json:"ussd_code,omitempty"`
	RedirectURL      string `json:"redirect_url,omitempty"`
	Message          string `json:"message,omitempty"`
	RequiresOTP      bool   `json:"requires_otp,omitempty"`
}

func (uc *PayCycleUsecase) Execute(ctx context.Context, req *PayCycleRequest) (*PayCycleResponse, error) {
	logger := zerolog.Ctx(ctx)

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	group, err := uc.groupRepo.FindByID(ctx, req.GroupID)
	if err != nil {
		return nil, fmt.Errorf("group not found: %w", err)
	}
	if group.ShopID != shop.ID.String() {
		return nil, fmt.Errorf("group does not belong to this shop")
	}
	if !group.IsActive() {
		return nil, fmt.Errorf("group is not active (status: %s)", group.Status)
	}

	participant, err := uc.participantRepo.FindByGroupAndCustomer(ctx, group.ID, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("you are not a participant of this group")
	}
	if !participant.IsActive() {
		return nil, fmt.Errorf("your participant status is not active")
	}

	yengapayReference := fmt.Sprintf("TONTINE:%s:%d:%s", group.ID[:8], group.CurrentCycle, participant.ID[:8])

	existingPayment, _ := uc.paymentRepo.FindByParticipantAndCycle(ctx, participant.ID, group.CurrentCycle)
	if existingPayment != nil && existingPayment.IsDone() {
		return nil, fmt.Errorf("you have already paid for cycle %d", group.CurrentCycle)
	}

	var paymentEntity *entity.TontinePayment

	if existingPayment != nil && (existingPayment.Status == entity.TontinePaymentProcessing || existingPayment.Status == entity.TontinePaymentFailed) {
		paymentEntity = existingPayment

		if paymentEntity.Status == entity.TontinePaymentFailed {
			paymentEntity.Status = entity.TontinePaymentProcessing
			paymentEntity.YengaPayReference = &yengapayReference
			paymentEntity.UpdatedAt = time.Now().UTC()
			if err := uc.paymentRepo.UpdateStatus(ctx, paymentEntity.ID, entity.TontinePaymentProcessing); err != nil {
				return nil, fmt.Errorf("failed to reset failed payment: %w", err)
			}
		}
		logger.Info().Str("payment_id", paymentEntity.ID).Msg("Reusing existing processing/failed payment (Idempotence)")
	} else {
		// 🆕 FIX C5 & Audit Taux : Récupérer le taux exact basé sur le type de cercle
		transactionType := getTontineTransactionType(group.CircleType)
		rate, err := uc.rateRepo.GetDefaultRate(ctx, shop.ID.String(), transactionType)
		if err != nil {
			return nil, fmt.Errorf("failed to get commission rate: %w", err) // 🆕 Retourner l'erreur au lieu de continuer
		}

		commissionCents := entity.CalculateCommission(group.AmountPerCycleCents, rate.RateBps)

		dueDate := time.Now().UTC().Add(24 * time.Hour)
		paymentEntity, err = entity.NewTontinePayment(group.ID, participant.ID, req.CustomerID, group.CurrentCycle, group.AmountPerCycleCents, commissionCents, dueDate)
		if err != nil {
			return nil, fmt.Errorf("failed to create payment entity: %w", err)
		}
		if err := paymentEntity.MarkProcessing(yengapayReference); err != nil {
			return nil, fmt.Errorf("failed to mark payment processing: %w", err)
		}

		tx, err := uc.txManager.BeginTx(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to start transaction: %w", err)
		}
		paymentRepoTx := uc.paymentRepo.WithTX(tx)
		if err = paymentRepoTx.Create(ctx, paymentEntity); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to save payment: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	provider, err := uc.paymentRegistry.GetAvailable(ctx, entity.ProviderYengaPay)
	if err != nil {
		_ = uc.paymentRepo.UpdateStatus(ctx, paymentEntity.ID, entity.TontinePaymentFailed)
		return nil, fmt.Errorf("payment provider not available: %w", err)
	}

	mmOperator := normalizeMobileOperator(req.Operator)
	metadata := map[string]interface{}{
		"group_id":     group.ID,
		"cycle_number": group.CurrentCycle,
		"flow":         req.Flow,
		"operator":     mmOperator,
	}
	if req.OTP != "" {
		metadata["otp"] = req.OTP
	}

	providerReq := &payment.PaymentRequest{
		PaymentID:   yengapayReference,
		AmountCents: paymentEntity.AmountCents,
		Currency:    "XOF",
		PhoneNumber: req.PhoneNumber,
		CustomerRef: req.CustomerID,
		Description: fmt.Sprintf("Tontine Cycle %d - %s", group.CurrentCycle, group.ID[:8]),
		Metadata:    metadata,
	}

	providerResp, err := provider.InitiatePayment(ctx, providerReq)
	if err != nil {
		_ = uc.paymentRepo.UpdateStatus(ctx, paymentEntity.ID, entity.TontinePaymentFailed)
		return nil, fmt.Errorf("payment initiation failed: %w", err)
	}

	intentToSave := providerResp.ProviderRef
	if intentToSave == "" && req.Flow == "direct" && req.OTP == "" {
		intentToSave = yengapayReference
	}

	if intentToSave != "" {
		if err := uc.paymentRepo.SetProviderIntentID(ctx, paymentEntity.ID, intentToSave); err != nil {
			logger.Warn().Err(err).Msg("Failed to save provider intent ID")
		}
	}

	logger.Info().Str("group_id", group.ID).Int("cycle", group.CurrentCycle).Str("provider_ref", providerResp.ProviderRef).Msg("Tontine payment initiated")

	msg := fmt.Sprintf("Paiement initié pour le cycle %d. Montant: %d FCFA.", group.CurrentCycle, paymentEntity.AmountCents/100)
	if providerResp.Metadata != nil {
		if n, ok := providerResp.Metadata["notification"].(string); ok && n != "" {
			msg = n
		}
	}
	requiresOTP := providerResp.USSDCode != "" && req.OTP == "" && req.Flow == "direct"

	return &PayCycleResponse{
		TontinePaymentID: paymentEntity.ID,
		AmountCents:      paymentEntity.AmountCents,
		CommissionCents:  paymentEntity.CommissionCents,
		NetAmountCents:   paymentEntity.NetAmountCents(),
		CycleNumber:      paymentEntity.CycleNumber,
		Status:           paymentEntity.Status,
		ProviderRef:      yengapayReference,
		YengaIntentID:    intentToSave,
		USSDCode:         providerResp.USSDCode,
		RedirectURL:      providerResp.RedirectURL,
		Message:          msg,
		RequiresOTP:      requiresOTP,
	}, nil
}

func getTontineTransactionType(circleType string) string {
	switch circleType {
	case entity.TontineCircleCommercial:
		return "tontine_commercial"
	case entity.TontineCircleCorporate:
		return "tontine_corporate"
	case entity.TontineCircleFamily:
		return "tontine_family"
	default:
		return "tontine_group"
	}
}

func normalizeMobileOperator(clientOperator string) string {
	op := strings.ToLower(strings.TrimSpace(clientOperator))
	switch op {
	case "orange", "orange_money":
		return "orange_money"
	case "moov", "moov_money":
		return "moov_money"
	case "telecel":
		return "telecel"
	case "coris", "coris_money", "corism":
		return "coris_money"
	case "sank", "sank_money", "sankm":
		return "sank_money"
	case "mtn":
		return "mtn"
	default:
		return op
	}
}

func GenerateTontineReference(groupID string, cycleNumber int, participantID string) string {
	groupUUID, err1 := uuid.Parse(groupID)
	participantUUID, err2 := uuid.Parse(participantID)
	if err1 != nil || err2 != nil {
		if len(groupID) < 8 {
			groupID = groupID + "00000000"
		}
		if len(participantID) < 8 {
			participantID = participantID + "00000000"
		}
		return fmt.Sprintf("TONTINE:%s:%d:%s", groupID[:8], cycleNumber, participantID[:8])
	}
	return fmt.Sprintf("TONTINE:%s:%d:%s", groupUUID.String()[:8], cycleNumber, participantUUID.String()[:8])
}

func ParseTontineReference(reference string) (string, int, string, error) {
	var groupPrefix, participantPrefix string
	var cycleNumber int
	n, err := fmt.Sscanf(reference, "TONTINE:%8s:%d:%8s", &groupPrefix, &cycleNumber, &participantPrefix)
	if err != nil || n != 3 {
		return "", 0, "", fmt.Errorf("invalid tontine reference format: %s", reference)
	}
	return groupPrefix, cycleNumber, participantPrefix, nil
}

type ListCustomerPaymentsUsecase struct {
	paymentRepo repository.TontinePaymentRepository
	groupRepo   repository.TontineGroupRepository
}

func NewListCustomerPaymentsUsecase(paymentRepo repository.TontinePaymentRepository, groupRepo repository.TontineGroupRepository) *ListCustomerPaymentsUsecase {
	return &ListCustomerPaymentsUsecase{paymentRepo: paymentRepo, groupRepo: groupRepo}
}

func (uc *ListCustomerPaymentsUsecase) Execute(ctx context.Context, groupID, customerID string) ([]*entity.TontinePayment, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	group, err := uc.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("group not found: %w", err)
	}
	if group.ShopID != shop.ID.String() {
		return nil, fmt.Errorf("group does not belong to this shop")
	}
	payments, err := uc.paymentRepo.FindByCustomerAndGroup(ctx, customerID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch payments: %w", err) // 🆕 Wrap l'erreur pour que le test passe
	}
	return payments, nil
}
