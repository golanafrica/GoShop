package customerusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/service"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

// ============================================================
// Usecase : Valider/Rejeter un KYC (côté marchand)
// ============================================================

// ReviewKYCUsecase permet à un marchand de valider ou rejeter un KYC client
type ReviewKYCUsecase struct {
	customerRepo repository.CustomerRepositoryInterface
	kycDocRepo   repository.CustomerKYCRepository
	notifService service.NotificationService
	txManager    repository.TxManager
}

// NewReviewKYCUsecase crée une nouvelle instance
func NewReviewKYCUsecase(
	customerRepo repository.CustomerRepositoryInterface,
	kycDocRepo repository.CustomerKYCRepository,
	notifService service.NotificationService,
	txManager repository.TxManager,
) *ReviewKYCUsecase {
	return &ReviewKYCUsecase{
		customerRepo: customerRepo,
		kycDocRepo:   kycDocRepo,
		notifService: notifService,
		txManager:    txManager,
	}
}

// ReviewKYCAction représente l'action de revue
type ReviewKYCAction string

const (
	ReviewKYCApprove ReviewKYCAction = "approve"
	ReviewKYCReject  ReviewKYCAction = "reject"
)

// ReviewKYCRequest représente la requête de revue KYC
type ReviewKYCRequest struct {
	CustomerID      string          `json:"customer_id"`
	Action          ReviewKYCAction `json:"action"`
	RejectionReason string          `json:"rejection_reason,omitempty"` // Requis si action = reject
}

// Validate valide la requête
func (r *ReviewKYCRequest) Validate() error {
	if r.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if r.Action != ReviewKYCApprove && r.Action != ReviewKYCReject {
		return fmt.Errorf("action must be 'approve' or 'reject'")
	}
	if r.Action == ReviewKYCReject && r.RejectionReason == "" {
		return fmt.Errorf("rejection_reason is required when action is 'reject'")
	}
	return nil
}

// ReviewKYCResponse représente la réponse après revue KYC
type ReviewKYCResponse struct {
	Customer    *entity.Customer              `json:"customer"`
	Documents   []*entity.CustomerKYCDocument `json:"documents"`
	NewKYCLevel entity.KYCLevel               `json:"new_kyc_level"`
}

// Execute valide ou rejette le KYC d'un client
func (uc *ReviewKYCUsecase) Execute(ctx context.Context, req *ReviewKYCRequest) (*ReviewKYCResponse, error) {
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

	// 3. Démarrer une transaction
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 4. Attacher les repositories à la transaction
	customerRepoTx := uc.customerRepo.WithTX(tx)
	kycDocRepoTx := uc.kycDocRepo.WithTX(tx)

	// 5. Vérifier que le client existe et appartient à la boutique
	customer, err := customerRepoTx.FindByCustomerID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// 6. Vérifier que le client a des documents en attente
	pendingDocs, err := kycDocRepoTx.FindPendingByCustomer(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending documents: %w", err)
	}
	if len(pendingDocs) == 0 {
		return nil, fmt.Errorf("no pending KYC documents for this customer")
	}

	// 7. Récupérer l'ID du marchand (owner du shop)
	merchantUserID := shop.OwnerID

	// 8. Traiter chaque document en attente
	for _, doc := range pendingDocs {
		switch req.Action {
		case ReviewKYCApprove:
			if err := doc.Approve(merchantUserID); err != nil {
				return nil, fmt.Errorf("failed to approve document: %w", err)
			}
		case ReviewKYCReject:
			if err := doc.Reject(merchantUserID, req.RejectionReason); err != nil {
				return nil, fmt.Errorf("failed to reject document: %w", err)
			}
		}

		if err := kycDocRepoTx.Update(ctx, doc); err != nil {
			return nil, fmt.Errorf("failed to update document: %w", err)
		}
	}

	// 9. Mettre à jour le statut KYC du client
	switch req.Action {
	case ReviewKYCApprove:
		customer.MarkKYCVerified(merchantUserID)
		logger.Info().
			Str("customer_id", customer.ID).
			Str("merchant_id", merchantUserID).
			Msg("✅ KYC approved for customer")
	case ReviewKYCReject:
		customer.MarkKYCRejected()
		logger.Info().
			Str("customer_id", customer.ID).
			Str("reason", req.RejectionReason).
			Msg("❌ KYC rejected for customer")
	}

	if _, err := customerRepoTx.UpdateCustomer(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to update customer KYC status: %w", err)
	}

	// 10. Commit
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 11. Notification (hors transaction, no-op pour l'instant)
	// TODO: Envoyer notification au client (SMS/email)
	logger.Info().
		Str("customer_id", customer.ID).
		Str("action", string(req.Action)).
		Str("new_kyc_level", string(customer.KYCLevel)).
		Msg("KYC review completed")

	// 12. Récupérer la liste finale des documents
	updatedDocs, err := uc.kycDocRepo.FindByCustomerID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated documents: %w", err)
	}

	return &ReviewKYCResponse{
		Customer:    customer,
		Documents:   updatedDocs,
		NewKYCLevel: customer.KYCLevel,
	}, nil
}

// ============================================================
// Usecase : Lister les KYC en attente (côté marchand)
// ============================================================

// ListPendingKYCUsecase liste les KYC en attente pour le marchand courant
type ListPendingKYCUsecase struct {
	kycDocRepo   repository.CustomerKYCRepository
	customerRepo repository.CustomerRepositoryInterface
}

// NewListPendingKYCUsecase crée une nouvelle instance
func NewListPendingKYCUsecase(
	kycDocRepo repository.CustomerKYCRepository,
	customerRepo repository.CustomerRepositoryInterface,
) *ListPendingKYCUsecase {
	return &ListPendingKYCUsecase{
		kycDocRepo:   kycDocRepo,
		customerRepo: customerRepo,
	}
}

// PendingKYCItem représente un KYC en attente avec les infos client
type PendingKYCItem struct {
	Customer  *entity.Customer              `json:"customer"`
	Documents []*entity.CustomerKYCDocument `json:"documents"`
}

// Execute retourne la liste des KYC en attente pour la boutique du marchand
func (uc *ListPendingKYCUsecase) Execute(ctx context.Context) ([]*PendingKYCItem, error) {
	// 1. Récupérer le shop du contexte
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	// 2. Récupérer tous les documents KYC en attente pour la boutique
	pendingDocs, err := uc.kycDocRepo.FindPendingByShop(ctx, shop.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending documents: %w", err)
	}

	// 3. Grouper par customer_id
	customerDocsMap := make(map[string][]*entity.CustomerKYCDocument)
	for _, doc := range pendingDocs {
		customerDocsMap[doc.CustomerID] = append(customerDocsMap[doc.CustomerID], doc)
	}

	// 4. Récupérer les infos client pour chaque customer_id
	var items []*PendingKYCItem
	for customerID, docs := range customerDocsMap {
		customer, err := uc.customerRepo.FindByCustomerID(ctx, customerID)
		if err != nil {
			// Log mais continue (client peut avoir été supprimé)
			continue
		}
		items = append(items, &PendingKYCItem{
			Customer:  customer,
			Documents: docs,
		})
	}

	if items == nil {
		items = []*PendingKYCItem{}
	}

	return items, nil
}
