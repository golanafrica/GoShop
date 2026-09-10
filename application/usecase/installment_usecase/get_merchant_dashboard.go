package installmentusecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	installmentdto "Goshop/application/dto/installment_dto"
	"Goshop/domain/repository"
	"Goshop/interfaces/utils"
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

// 🆕 v5.3.0 : Structure pour la sérialisation JSON dans Redis
type dashboardCacheData struct {
	Dashboard *installmentdto.MerchantInstallmentDashboardResponse `json:"dashboard"`
	Summaries []*installmentdto.InstallmentOrderSummary            `json:"summaries"`
}

// Execute récupère toutes les commandes en tranches d'un marchand et calcule les stats
// 🆕 v5.3.0 : Intègre un cache Redis avec TTL de 5 minutes pour optimiser les performances
func (uc *GetMerchantDashboardUsecase) Execute(ctx context.Context, shopID string) (*installmentdto.MerchantInstallmentDashboardResponse, []*installmentdto.InstallmentOrderSummary, error) {
	cacheKey := fmt.Sprintf("merchant_installment_dashboard:%s", shopID)

	// 1. 🆕 Essayer le cache Redis
	if utils.Rdb != nil {
		cached, err := utils.Rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			var data dashboardCacheData
			if err := json.Unmarshal([]byte(cached), &data); err == nil {
				return data.Dashboard, data.Summaries, nil
			}
		}
	}

	// 2. Fallback sur la base de données
	orders, err := uc.orderRepo.FindAll(ctx)
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
			CustomerName:        "Client " + order.CustomerID[:8],
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

	// 3. 🆕 Mettre en cache le résultat pour 5 minutes
	if utils.Rdb != nil {
		dataToCache := dashboardCacheData{
			Dashboard: &dashboard,
			Summaries: summaries,
		}
		if jsonData, err := json.Marshal(dataToCache); err == nil {
			utils.Rdb.Set(ctx, cacheKey, jsonData, 5*time.Minute)
		}
	}

	// ✅ Correction de l'erreur de syntaxe ici
	return &dashboard, summaries, nil
}

// 🆕 v5.3.0 : InvalidateCache permet de vider le cache après un paiement ou une confirmation de livraison
func (uc *GetMerchantDashboardUsecase) InvalidateCache(ctx context.Context, shopID string) {
	if utils.Rdb != nil {
		cacheKey := fmt.Sprintf("merchant_installment_dashboard:%s", shopID)
		utils.Rdb.Del(ctx, cacheKey)
	}
}
