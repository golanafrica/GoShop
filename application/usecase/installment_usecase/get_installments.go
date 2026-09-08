package installmentusecase

import (
	"context"
	"fmt"

	"Goshop/domain/entity"
	"Goshop/domain/repository"
)

type InstallmentsResponse struct {
	Installments   []*entity.OrderInstallment `json:"installments"`
	TotalPaid      int64                      `json:"total_paid"`
	TotalRemaining int64                      `json:"total_remaining"`
}

type GetInstallmentsUsecase struct {
	installmentRepo repository.OrderInstallmentRepository
}

func NewGetInstallmentsUsecase(installmentRepo repository.OrderInstallmentRepository) *GetInstallmentsUsecase {
	return &GetInstallmentsUsecase{installmentRepo: installmentRepo}
}

func (uc *GetInstallmentsUsecase) Execute(ctx context.Context, orderID string) (*InstallmentsResponse, error) {
	installments, err := uc.installmentRepo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("échec de récupération des tranches: %w", err)
	}

	var totalPaid, totalRemaining int64
	for _, inst := range installments {
		if inst.IsPaid() {
			totalPaid += inst.AmountCents
		} else {
			totalRemaining += inst.AmountCents
		}
	}

	return &InstallmentsResponse{
		Installments:   installments,
		TotalPaid:      totalPaid,
		TotalRemaining: totalRemaining,
	}, nil
}
