package installmentusecase

import (
	"context"
	"fmt"
	"time"

	installmentdto "Goshop/application/dto/installment_dto"
	"Goshop/domain/repository"
)

type GetMerchantDashboardUsecase struct {
	orderRepo        repository.OrderRepository
	installmentRepo  repository.OrderInstallmentRepository
	deliveryZoneRepo repository.DeliveryZoneRepository
}

func NewGetMerchantDashboardUsecase(
	orderRepo repository.OrderRepository,
	installmentRepo repository.OrderInstallmentRepository,
	deliveryZoneRepo repository.DeliveryZoneRepository,
) *GetMerchantDashboardUsecase {
	return &GetMerchantDashboardUsecase{
		orderRepo:        orderRepo,
		installmentRepo:  installmentRepo,
		deliveryZoneRepo: deliveryZoneRepo,
	}
}

// Execute récupère toutes les commandes en tranches d'un marchand et calcule les stats
func (uc *GetMerchantDashboardUsecase) Execute(ctx context.Context, shopID string) (*installmentdto.MerchantInstallmentDashboardResponse, []*installmentdto.InstallmentOrderSummary, error) {
	// 1. Récupérer toutes les commandes du marchand (à optimiser plus tard avec un filtre payment_method ou jointure)
	orders, err := uc.orderRepo.FindAll(ctx) // Note: Idéalement, ajouter FindByShopID(ctx, shopID)
	if err != nil {
		return nil, nil, fmt.Errorf("échec de la récupération des commandes: %w", err)
	}

	var dashboard installmentdto.MerchantInstallmentDashboardResponse
	var summaries []*installmentdto.InstallmentOrderSummary

	for _, order := range orders {
		if order.ShopID != shopID {
			continue // Filtrage multi-tenant
		}

		installments, err := uc.installmentRepo.GetByOrderID(ctx, order.ID)
		if err != nil || len(installments) == 0 {
			continue // Ce n'est pas une commande en tranches
		}

		// Calculer les montants
		var paidAmount, remainingAmount int64
		var nextDueDate *time.Time
		var hasOverdue bool
		var allPaid = true

		var details []*installmentdto.InstallmentDetail
		for _, inst := range installments {
			details = append(details, &installmentdto.InstallmentDetail{
				TrancheNumber: inst.TrancheNumber,
				AmountCents:   inst.AmountCents,
				DueDate:       inst.DueDate,
				Status:        string(inst.Status),
				PaidAt:        inst.PaidAt,
			})

			if inst.IsPaid() {
				paidAmount += inst.AmountCents
			} else {
				remainingAmount += inst.AmountCents
				allPaid = false
				if nextDueDate == nil || inst.DueDate.Before(*nextDueDate) {
					nextDueDate = &inst.DueDate
				}
				if inst.IsOverdue() {
					hasOverdue = true
				}
			}
		}

		// Mettre à jour les stats globales
		dashboard.TotalPendingOrders++
		dashboard.TotalAmountPending += remainingAmount
		if hasOverdue {
			dashboard.TotalOverdueOrders++
		}
		// Le montant en séquestre est le montant payé mais pas encore libéré
		dashboard.TotalHeldAmount += paidAmount

		// Déterminer le statut global
		status := "pending"
		if hasOverdue {
			status = "overdue"
		} else if allPaid {
			status = "complete"
		} else if paidAmount > 0 {
			status = "partial"
		}

		// Calculer la date de libération prévue
		var expectedReleaseDate *time.Time
		delayDays := order.InstallmentReleaseDelayDays
		if delayDays == 0 {
			delayDays = 7 // Fallback
		}

		if order.DeliveredAt != nil {
			releaseDate := order.DeliveredAt.AddDate(0, 0, delayDays)
			expectedReleaseDate = &releaseDate
		} else if allPaid {
			// Si tout est payé mais pas livré, fallback à 30 jours après création
			releaseDate := order.CreatedAt.AddDate(0, 0, 30)
			expectedReleaseDate = &releaseDate
		}

		// Récupérer le nom de la zone (optionnel, pour l'affichage)
		zoneName := "Zone inconnue"
		if order.DeliveryZoneID != nil {
			if zone, err := uc.deliveryZoneRepo.FindByID(ctx, *order.DeliveryZoneID); err == nil && zone != nil {
				zoneName = zone.ZoneName
			}
		}

		summaries = append(summaries, &installmentdto.InstallmentOrderSummary{
			OrderID:             order.ID,
			CustomerName:        "Client " + order.CustomerID[:8], // À remplacer par un jointure customer si besoin
			TotalAmount:         order.TotalCents,
			PaidAmount:          paidAmount,
			RemainingAmount:     remainingAmount,
			Status:              status,
			NextDueDate:         nextDueDate,
			DeliveryZoneName:    zoneName,
			ReleaseDelayDays:    delayDays,
			ExpectedReleaseDate: expectedReleaseDate,
			Installments:        details,
		})
	}

	return &dashboard, summaries, nil
}
