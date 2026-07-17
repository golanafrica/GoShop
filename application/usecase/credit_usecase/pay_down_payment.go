package creditusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
	"Goshop/infrastructure/payment"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// PayDownPaymentRequest représente la requête pour initier le paiement de l'apport
type PayDownPaymentRequest struct {
	ContractID  string `json:"contract_id"`
	PhoneNumber string `json:"phone_number"`
	Operator    string `json:"operator"`      // ex: "ORANGE", "MOOV", "TELECEL"
	OTP         string `json:"otp,omitempty"` // Requis pour ONE_STEP
}

// PayDownPaymentResponse représente la réponse après initiation du paiement
type PayDownPaymentResponse struct {
	PaymentID   string `json:"payment_id"`
	ProviderRef string `json:"provider_ref,omitempty"`
	Status      string `json:"status"`
	USSDCode    string `json:"ussd_code,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
	Message     string `json:"message"`
}

// Validate valide la requête
func (r *PayDownPaymentRequest) Validate() error {
	if r.ContractID == "" {
		return fmt.Errorf("contract_id is required")
	}
	if r.PhoneNumber == "" {
		return fmt.Errorf("phone_number is required")
	}
	if r.Operator == "" {
		return fmt.Errorf("operator is required")
	}
	return nil
}

// PayDownPaymentUsecase gère l'initiation sécurisée du paiement de l'apport initial
type PayDownPaymentUsecase struct {
	contractRepo repository.CreditContractRepository
	paymentRepo  repository.PaymentRepository
	registry     PaymentRegistry
	txManager    repository.TxManager
}

// NewPayDownPaymentUsecase crée une nouvelle instance
func NewPayDownPaymentUsecase(
	contractRepo repository.CreditContractRepository,
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry,
	txManager repository.TxManager,
) *PayDownPaymentUsecase {
	return &PayDownPaymentUsecase{
		contractRepo: contractRepo,
		paymentRepo:  paymentRepo,
		registry:     registry,
		txManager:    txManager,
	}
}

// Execute initie le paiement de l'apport initial via le provider (YengaPay)
func (uc *PayDownPaymentUsecase) Execute(ctx context.Context, req *PayDownPaymentRequest) (*PayDownPaymentResponse, error) {
	logger := zerolog.Ctx(ctx)

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	contract, err := uc.contractRepo.WithTX(tx).FindByID(ctx, req.ContractID)
	if err != nil {
		return nil, fmt.Errorf("contract not found: %w", err)
	}

	if contract.ShopID != shopID {
		return nil, fmt.Errorf("access denied: contract does not belong to tenant shop")
	}

	if contract.Status != entity.CreditContractActive {
		return nil, fmt.Errorf("contract is not in active status (current: %s)", contract.Status)
	}

	if contract.DownPaymentPaidAt != nil {
		return nil, fmt.Errorf("down payment already paid at %v", contract.DownPaymentPaidAt)
	}

	// Créer l'entité Payment polymorphe
	newPayment, err := entity.NewPayment(shop.ID, uuid.Nil, entity.PaymentProvider(req.Operator), contract.DownPaymentCents)
	if err != nil {
		return nil, fmt.Errorf("create payment entity: %w", err)
	}

	refType := "credit_down_payment"
	refID := contract.ID
	newPayment.ReferenceType = &refType
	newPayment.ReferenceID = &refID
	newPayment.CustomerPhone = &req.PhoneNumber

	if err := uc.paymentRepo.WithTX(tx).Create(ctx, newPayment); err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}

	provider, err := uc.registry.GetAvailable(ctx, entity.PaymentProvider(req.Operator))
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}

	providerReq := &payment.PaymentRequest{
		PaymentID:   newPayment.ID.String(),
		AmountCents: contract.DownPaymentCents,
		Currency:    entity.CurrencyXOF,
		PhoneNumber: req.PhoneNumber,
		CustomerRef: contract.CustomerID,
		Description: fmt.Sprintf("Apport initial crédit contrat #%s", contract.ID[:8]),
		Metadata: map[string]interface{}{
			"contract_id": contract.ID,
			"shop_id":     shopID,
		},
	}

	if (req.Operator == "ORANGE" || req.Operator == "TELECEL") && req.OTP != "" {
		providerReq.Metadata["otp"] = req.OTP
	}

	providerResp, err := provider.InitiatePayment(ctx, providerReq)
	if err != nil {
		if markErr := newPayment.MarkFailed(err.Error()); markErr != nil {
			logger.Error().Err(markErr).Msg("Failed to mark payment as failed")
		}
		if updateErr := uc.paymentRepo.WithTX(tx).Update(ctx, newPayment); updateErr != nil {
			logger.Error().Err(updateErr).Msg("Failed to update payment")
		}
		return nil, fmt.Errorf("initiate payment with provider: %w", err)
	}

	if providerResp.ProviderRef != "" {
		newPayment.ProviderRef = &providerResp.ProviderRef
		if err := newPayment.MarkProcessing(); err != nil {
			return nil, fmt.Errorf("mark payment as processing: %w", err)
		}
	}

	if providerResp.Metadata != nil {
		newPayment.Metadata = providerResp.Metadata
	}

	if err := uc.paymentRepo.WithTX(tx).Update(ctx, newPayment); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("payment_id", newPayment.ID.String()).
		Str("contract_id", contract.ID).
		Str("provider_ref", providerResp.ProviderRef).
		Msg("Down payment initiated successfully")

	message := "Paiement de l'apport initié. Suivez les instructions sur votre téléphone."
	if req.OTP != "" {
		message = "Paiement de l'apport en cours de validation..."
	}

	return &PayDownPaymentResponse{
		PaymentID:   newPayment.ID.String(),
		ProviderRef: providerResp.ProviderRef,
		Status:      string(newPayment.Status),
		USSDCode:    providerResp.USSDCode,
		RedirectURL: providerResp.RedirectURL,
		Message:     message,
	}, nil
}
