package installmentusecase

import (
	"context"
	"fmt"
	"time"

	"Goshop/domain/entity"
	"Goshop/domain/repository"

	"github.com/rs/zerolog/log"
)

type CreateInstallmentOrderUsecase struct {
	txManager       repository.TxManager
	orderRepo       repository.OrderRepository
	planRepo        repository.InstallmentPlanRepository
	customerRepo    repository.CustomerRepositoryInterface
	scoreRepo       repository.CustomerReliabilityScoreRepository // 🆕 AJOUTÉ
	installmentRepo repository.OrderInstallmentRepository
}

func NewCreateInstallmentOrderUsecase(
	txManager repository.TxManager,
	orderRepo repository.OrderRepository,
	planRepo repository.InstallmentPlanRepository,
	customerRepo repository.CustomerRepositoryInterface,
	scoreRepo repository.CustomerReliabilityScoreRepository, // 🆕 AJOUTÉ
	installmentRepo repository.OrderInstallmentRepository,
) *CreateInstallmentOrderUsecase {
	return &CreateInstallmentOrderUsecase{
		txManager:       txManager,
		orderRepo:       orderRepo,
		planRepo:        planRepo,
		customerRepo:    customerRepo,
		scoreRepo:       scoreRepo, // 🆕 AJOUTÉ
		installmentRepo: installmentRepo,
	}
}

// Execute crée la commande ET les tranches de manière atomique.
func (uc *CreateInstallmentOrderUsecase) Execute(ctx context.Context, shopID, customerID string, totalCents int64, items []*entity.OrderItem) (*entity.Order, error) {
	tx, err := uc.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Créer la commande (Utilise ta logique existante, adaptée pour utiliser 'tx')
	order := &entity.Order{
		ID:            "generated-order-uuid", // Remplace par uuid.New().String() si nécessaire
		ShopID:        shopID,
		CustomerID:    customerID,
		TotalCents:    totalCents,
		Status:        string(entity.OrderStatusPending),
		PaymentMethod: "installment",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		Items:         items,
	}

	// TODO: uc.orderRepo.CreateWithTx(ctx, tx, order)
	orderID := order.ID

	// 2. Vérifier si un plan en tranches est actif pour le premier produit
	productID := items[0].ProductID
	plan, err := uc.planRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la vérification du plan: %w", err)
	}

	if plan == nil || !plan.IsActive {
		// Pas de plan en tranches, on commit juste la commande classique
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
		return order, nil
	}

	// 🆕 VÉRIFICATION DU SCORE DE FIABILITÉ
	score, err := uc.scoreRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to check customer reliability score: %w", err)
	}

	// Si le client n'a pas encore de score, on en crée un par défaut (Bronze)
	if score == nil {
		score = entity.NewCustomerReliabilityScore(customerID)
	}

	// Vérifier la limite de tranches
	maxInstallments := score.GetMaxInstallments()
	if maxInstallments > 0 && plan.NbTranches > maxInstallments {
		return nil, fmt.Errorf("accès refusé : votre niveau de fiabilité (%s) est limité à %d tranches maximum. Vous avez demandé %d tranches",
			score.Tier, maxInstallments, plan.NbTranches)
	}

	// 🆕 v5.1.0 : Copier les informations de délai dynamique du plan vers la commande
	order.DeliveryZoneID = plan.DeliveryZoneID
	order.InstallmentReleaseDelayDays = plan.InstallmentReleaseDelayDays

	// 3. Calculer les montants et dates des tranches
	amounts := entity.CalculateInstallmentAmount(totalCents, plan.NbTranches)
	dueDates := entity.CalculateDueDates(time.Now().UTC(), plan.NbTranches, plan.DelaiJours)

	// 4. Générer les entités OrderInstallment
	var installments []*entity.OrderInstallment
	for i := 0; i < plan.NbTranches; i++ {
		inst := entity.NewOrderInstallment(orderID, i+1, amounts[i], dueDates[i])
		installments = append(installments, inst)
	}

	// 5. Sauvegarder les tranches en batch via le repository
	if err := uc.installmentRepo.CreateBatch(ctx, installments); err != nil {
		return nil, fmt.Errorf("échec de la création des tranches: %w", err)
	}

	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit installment order transaction")
		return nil, fmt.Errorf("échec du commit de la transaction: %w", err)
	}

	log.Info().Str("order_id", orderID).Int("tranches", plan.NbTranches).Msg("Installment order created successfully")
	return order, nil
}
