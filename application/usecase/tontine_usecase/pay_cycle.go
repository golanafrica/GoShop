package tontineusecase

import (
	"context"
	"fmt"
	"time"

	paymentusecase "Goshop/application/usecase/payment_usecase"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// PayCycleUsecase permet à un client de payer une cotisation de tontine via un fournisseur de paiement
type PayCycleUsecase struct {
	groupRepo       repository.TontineGroupRepository
	participantRepo repository.TontineParticipantRepository
	paymentRepo     repository.TontinePaymentRepository
	shopRepo        repository.ShopRepository
	txManager       repository.TxManager
	paymentRegistry paymentusecase.PaymentRegistry // 🆕 Interface réutilisée de payment_usecase
}

// NewPayCycleUsecase crée une nouvelle instance
func NewPayCycleUsecase(
	groupRepo repository.TontineGroupRepository,
	participantRepo repository.TontineParticipantRepository,
	paymentRepo repository.TontinePaymentRepository,
	shopRepo repository.ShopRepository,
	txManager repository.TxManager,
	paymentRegistry paymentusecase.PaymentRegistry, // 🆕 AJOUT
) *PayCycleUsecase {
	return &PayCycleUsecase{
		groupRepo:       groupRepo,
		participantRepo: participantRepo,
		paymentRepo:     paymentRepo,
		shopRepo:        shopRepo,
		txManager:       txManager,
		paymentRegistry: paymentRegistry, // 🆕 AJOUT
	}
}

// PayCycleRequest représente la requête de paiement d'une cotisation
type PayCycleRequest struct {
	GroupID     string `json:"group_id"`
	CustomerID  string `json:"customer_id"`
	Operator    string `json:"operator"`     // orange_money, moov_money, telecel, etc.
	PhoneNumber string `json:"phone_number"` // +226...
	Flow        string `json:"flow"`         // indirect (défaut) ou direct
}

// Validate valide la requête
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
	return nil
}

// PayCycleResponse représente la réponse après initiation du paiement
type PayCycleResponse struct {
	TontinePaymentID string `json:"tontine_payment_id"`
	AmountCents      int64  `json:"amount_cents"`
	CommissionCents  int64  `json:"commission_cents"`
	NetAmountCents   int64  `json:"net_amount_cents"`
	CycleNumber      int    `json:"cycle_number"`
	Status           string `json:"status"`

	// Infos Provider (YengaPay, Orange Money, etc.)
	ProviderRef string `json:"provider_ref,omitempty"`
	USSDCode    string `json:"ussd_code,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Execute initie le paiement d'une cotisation de tontine
func (uc *PayCycleUsecase) Execute(ctx context.Context, req *PayCycleRequest) (*PayCycleResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Récupérer le shop du contexte (multi-tenant)
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 3. Récupérer le groupe
	group, err := uc.groupRepo.FindByID(ctx, req.GroupID)
	if err != nil {
		return nil, fmt.Errorf("group not found: %w", err)
	}

	// 4. Vérifier que le groupe appartient à la boutique
	if group.ShopID != shop.ID.String() {
		return nil, fmt.Errorf("group does not belong to this shop")
	}

	// 5. Vérifier que le groupe est actif
	if !group.IsActive() {
		return nil, fmt.Errorf("group is not active (status: %s)", group.Status)
	}

	// 6. Récupérer le participant
	participant, err := uc.participantRepo.FindByGroupAndCustomer(ctx, group.ID, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("you are not a participant of this group")
	}

	// 7. Vérifier que le participant est actif
	if !participant.IsActive() {
		return nil, fmt.Errorf("your participant status is not active (status: %s)", participant.Status)
	}

	// 8. Vérifier que le client n'a pas déjà payé pour ce cycle
	existingPayment, _ := uc.paymentRepo.FindByParticipantAndCycle(ctx, participant.ID, group.CurrentCycle)
	if existingPayment != nil && existingPayment.IsDone() {
		return nil, fmt.Errorf("you have already paid for cycle %d", group.CurrentCycle)
	}

	// 9. Récupérer la configuration de commission de la boutique
	settings, err := uc.shopRepo.GetPaymentSettings(ctx, shop.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop settings: %w", err)
	}

	commissionRate := settings.GetTontineCommissionRate()
	commissionCents := entity.CalculateCommission(group.AmountPerCycleCents, commissionRate)

	// 10. Générer la référence unique pour ce paiement tontine
	yengapayReference := fmt.Sprintf("TONTINE:%s:%d:%s",
		group.ID[:8],
		group.CurrentCycle,
		participant.ID[:8],
	)

	// 11. Créer l'entité de paiement en statut PENDING
	dueDate := time.Now().UTC().Add(24 * time.Hour) // 24h pour payer
	paymentEntity, err := entity.NewTontinePayment(
		group.ID,
		participant.ID,
		req.CustomerID,
		group.CurrentCycle,
		group.AmountPerCycleCents,
		commissionCents,
		dueDate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment entity: %w", err)
	}

	// Marquer comme PROCESSING avec la référence (cela définit YengaPayReference)
	if err := paymentEntity.MarkProcessing(yengapayReference); err != nil {
		return nil, fmt.Errorf("failed to mark payment processing: %w", err)
	}

	// 12. Sauvegarder en base et committer IMMÉDIATEMENT (transaction courte)
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	paymentRepoTx := uc.paymentRepo.WithTX(tx)
	if err = paymentRepoTx.Create(ctx, paymentEntity); err != nil {
		_ = tx.Rollback() // Rollback explicite uniquement en cas d'échec du Create
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	// 🆕 Committer immédiatement pour libérer la connexion DB avant l'appel réseau
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 13. 🆕 Appeler le fournisseur de paiement HORS TRANSACTION
	providerName := entity.PaymentProvider(req.Operator)
	provider, err := uc.paymentRegistry.GetAvailable(ctx, providerName)
	if err != nil {
		// Marquer le paiement comme FAILED (pas de transaction ici)
		_ = uc.paymentRepo.UpdateStatus(ctx, paymentEntity.ID, entity.TontinePaymentFailed)
		return nil, fmt.Errorf("payment provider not available: %w", err)
	}

	providerReq := &payment.PaymentRequest{
		PaymentID:   paymentEntity.ID,
		AmountCents: paymentEntity.AmountCents,
		Currency:    "XOF",
		PhoneNumber: req.PhoneNumber,
		CustomerRef: req.CustomerID, // 🆕 ID du client qui paie, pas du marchand
		Description: fmt.Sprintf("Tontine Cycle %d - %s", group.CurrentCycle, group.ID[:8]),
		Metadata: map[string]interface{}{
			"group_id":     group.ID,
			"cycle_number": group.CurrentCycle,
			"flow":         req.Flow,
		},
	}

	providerResp, err := provider.InitiatePayment(ctx, providerReq)
	if err != nil {
		// Marquer le paiement comme FAILED (pas de transaction ici)
		_ = uc.paymentRepo.UpdateStatus(ctx, paymentEntity.ID, entity.TontinePaymentFailed)
		return nil, fmt.Errorf("payment initiation failed: %w", err)
	}

	logger.Info().
		Str("group_id", group.ID).
		Str("customer_id", req.CustomerID).
		Int("cycle", group.CurrentCycle).
		Int64("amount_cents", group.AmountPerCycleCents).
		Str("provider_ref", yengapayReference).
		Msg("Tontine payment successfully initiated with provider")

	// 14. Retourner la réponse au client (qui contiendra le code USSD ou l'URL de redirection)
	return &PayCycleResponse{
		TontinePaymentID: paymentEntity.ID,
		AmountCents:      paymentEntity.AmountCents,
		CommissionCents:  paymentEntity.CommissionCents,
		NetAmountCents:   paymentEntity.NetAmountCents(),
		CycleNumber:      paymentEntity.CycleNumber,
		Status:           paymentEntity.Status,
		ProviderRef:      yengapayReference,
		USSDCode:         providerResp.USSDCode,
		RedirectURL:      providerResp.RedirectURL,
		Message: fmt.Sprintf(
			"Paiement initié pour le cycle %d. Montant: %d FCFA. Suivez les instructions sur votre téléphone.",
			group.CurrentCycle,
			paymentEntity.AmountCents/100,
		),
	}, nil
}

// ============================================================
// ListCustomerPaymentsUsecase
// ============================================================

type ListCustomerPaymentsUsecase struct {
	paymentRepo repository.TontinePaymentRepository
	groupRepo   repository.TontineGroupRepository
}

func NewListCustomerPaymentsUsecase(
	paymentRepo repository.TontinePaymentRepository,
	groupRepo repository.TontineGroupRepository,
) *ListCustomerPaymentsUsecase {
	return &ListCustomerPaymentsUsecase{
		paymentRepo: paymentRepo,
		groupRepo:   groupRepo,
	}
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
		return nil, fmt.Errorf("failed to fetch payments: %w", err)
	}

	return payments, nil
}

// ============================================================
// Utilitaire : Générer la référence YengaPay pour un paiement tontine
// ============================================================

// GenerateTontineReference génère la référence YengaPay pour un paiement tontine
// Format : "TONTINE:{groupID_short}:{cycleNumber}:{participantID_short}"
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
		return fmt.Sprintf("TONTINE:%s:%d:%s",
			groupID[:8],
			cycleNumber,
			participantID[:8],
		)
	}

	return fmt.Sprintf("TONTINE:%s:%d:%s",
		groupUUID.String()[:8],
		cycleNumber,
		participantUUID.String()[:8],
	)
}

// ParseTontineReference parse une référence YengaPay tontine
// Retourne (groupIDPrefix, cycleNumber, participantIDPrefix, error)
func ParseTontineReference(reference string) (string, int, string, error) {
	var groupPrefix, participantPrefix string
	var cycleNumber int

	n, err := fmt.Sscanf(reference, "TONTINE:%8s:%d:%8s", &groupPrefix, &cycleNumber, &participantPrefix)
	if err != nil || n != 3 {
		return "", 0, "", fmt.Errorf("invalid tontine reference format: %s", reference)
	}

	return groupPrefix, cycleNumber, participantPrefix, nil
}
