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

// PayInstallmentRequest représente la requête pour payer une échéance
type PayInstallmentRequest struct {
	InstallmentID string `json:"installment_id"`
	PhoneNumber   string `json:"phone_number"`
	Operator      string `json:"operator"`      // ex: "ORANGE", "MOOV", "TELECEL"
	OTP           string `json:"otp,omitempty"` // Requis pour ONE_STEP (Orange/Telecel)
}

// PayInstallmentResponse représente la réponse après initiation du paiement
type PayInstallmentResponse struct {
	PaymentID   string `json:"payment_id"`
	ProviderRef string `json:"provider_ref,omitempty"`
	Status      string `json:"status"`
	USSDCode    string `json:"ussd_code,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
	Message     string `json:"message"`
}

// Validate valide la requête
func (r *PayInstallmentRequest) Validate() error {
	if r.InstallmentID == "" {
		return fmt.Errorf("installment_id is required")
	}
	if r.PhoneNumber == "" {
		return fmt.Errorf("phone_number is required")
	}
	if r.Operator == "" {
		return fmt.Errorf("operator is required")
	}
	return nil
}

// PayInstallmentUsecase gère le paiement d'une échéance de crédit
type PayInstallmentUsecase struct {
	installmentRepo repository.CreditInstallmentRepository
	contractRepo    repository.CreditContractRepository
	paymentRepo     repository.PaymentRepository
	registry        PaymentRegistry // Interface vers les providers
	txManager       repository.TxManager
}

// PaymentRegistry est une interface locale pour éviter les dépendances circulaires
type PaymentRegistry interface {
	GetAvailable(ctx context.Context, providerCode entity.PaymentProvider) (payment.Provider, error)
}

// NewPayInstallmentUsecase crée une nouvelle instance
func NewPayInstallmentUsecase(
	installmentRepo repository.CreditInstallmentRepository,
	contractRepo repository.CreditContractRepository,
	paymentRepo repository.PaymentRepository,
	registry PaymentRegistry,
	txManager repository.TxManager,
) *PayInstallmentUsecase {
	return &PayInstallmentUsecase{
		installmentRepo: installmentRepo,
		contractRepo:    contractRepo,
		paymentRepo:     paymentRepo,
		registry:        registry,
		txManager:       txManager,
	}
}

// Execute initie le paiement d'une échéance via le provider (YengaPay)
func (uc *PayInstallmentUsecase) Execute(ctx context.Context, req *PayInstallmentRequest) (*PayInstallmentResponse, error) {
	logger := zerolog.Ctx(ctx)

	// 1. Valider la requête
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// 2. Vérifier le multi-tenant
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Récupérer l'échéance (Le repo attend un string)
	installment, err := uc.installmentRepo.WithTX(tx).FindByID(ctx, req.InstallmentID)
	if err != nil {
		return nil, fmt.Errorf("installment not found: %w", err)
	}

	// 5. Récupérer le contrat pour vérifier l'appartenance au shop
	contract, err := uc.contractRepo.WithTX(tx).FindByID(ctx, installment.ContractID)
	if err != nil {
		return nil, fmt.Errorf("contract not found: %w", err)
	}

	// 6. Vérifier que l'échéance est payable et appartient au bon shop
	if contract.ShopID != shopID {
		return nil, fmt.Errorf("access denied: installment does not belong to tenant shop")
	}

	if installment.Status != entity.InstallmentPending && installment.Status != entity.InstallmentLate {
		return nil, fmt.Errorf("installment is not payable (current status: %s)", installment.Status)
	}

	// 7. Créer l'entité Payment polymorphe
	// On utilise uuid.Nil pour OrderID car c'est un paiement de crédit, pas de commande
	newPayment, err := entity.NewPayment(shop.ID, uuid.Nil, entity.PaymentProvider(req.Operator), installment.AmountCents)
	if err != nil {
		return nil, fmt.Errorf("create payment entity: %w", err)
	}

	// Lier à l'échéance (polymorphisme)
	refType := "credit_installment"
	refID := installment.ID
	newPayment.ReferenceType = &refType
	newPayment.ReferenceID = &refID
	newPayment.CustomerPhone = &req.PhoneNumber

	// 8. Sauvegarder en statut PENDING
	if err := uc.paymentRepo.WithTX(tx).Create(ctx, newPayment); err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}

	// 9. Récupérer le provider (YengaPay)
	provider, err := uc.registry.GetAvailable(ctx, entity.PaymentProvider(req.Operator))
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}

	// 10. Préparer la requête pour le provider (YengaPay init-and-pay)
	providerReq := &payment.PaymentRequest{
		PaymentID:   newPayment.ID.String(),
		AmountCents: installment.AmountCents,
		Currency:    entity.CurrencyXOF,
		PhoneNumber: req.PhoneNumber,
		CustomerRef: contract.CustomerID,
		Description: fmt.Sprintf("Paiement échéance crédit #%d", installment.InstallmentNumber),
		Metadata: map[string]interface{}{
			"installment_id": installment.ID,
			"contract_id":    contract.ID,
			"shop_id":        shopID,
		},
	}

	// Si c'est un opérateur ONE_STEP (Orange/Telecel) et qu'on a l'OTP
	if (req.Operator == "ORANGE" || req.Operator == "TELECEL") && req.OTP != "" {
		providerReq.Metadata["otp"] = req.OTP
	}

	// 11. Initier le paiement auprès du provider
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

	// 12. Mettre à jour le paiement avec la référence provider
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

	// 13. Commit la transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Str("payment_id", newPayment.ID.String()).
		Str("installment_id", installment.ID).
		Str("provider_ref", providerResp.ProviderRef).
		Msg("Installment payment initiated successfully")

	// 14. Construire la réponse
	message := "Paiement initié. Suivez les instructions sur votre téléphone."
	if req.OTP != "" {
		message = "Paiement en cours de validation..."
	}

	return &PayInstallmentResponse{
		PaymentID:   newPayment.ID.String(),
		ProviderRef: providerResp.ProviderRef,
		Status:      string(newPayment.Status),
		USSDCode:    providerResp.USSDCode,
		RedirectURL: providerResp.RedirectURL,
		Message:     message,
	}, nil
}
