package cod_dto

import (
	"Goshop/domain/entity"
	"time"
)

// CODCommissionDashboardResponse représente les statistiques globales et récentes des commissions COD pour les admins
type CODCommissionDashboardResponse struct {
	// Statistiques globales (en centimes)
	TotalPendingCents   int64 `json:"total_pending_cents"`
	TotalDueCents       int64 `json:"total_due_cents"`
	TotalCollectedCents int64 `json:"total_collected_cents"`

	// Dernières preuves nécessitant une attention (dues ou en attente récente)
	RecentProofs []RecentCODProof `json:"recent_proofs"`
}

// RecentCODProof représente une preuve COD simplifiée pour l'affichage admin
type RecentCODProof struct {
	ID                string                     `json:"id"`
	OrderID           string                     `json:"order_id"`
	ShopID            string                     `json:"shop_id"`
	CustomerID        string                     `json:"customer_id"`
	Status            entity.CODProofStatus      `json:"status"`
	CommissionStatus  entity.CODCommissionStatus `json:"commission_status"`
	CommissionCents   int64                      `json:"commission_cents"`
	CreatedAt         time.Time                  `json:"created_at"`
	CommissionDueDate *time.Time                 `json:"commission_due_date,omitempty"`
}
