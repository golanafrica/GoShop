package reportusecase

import (
	"context"
	"fmt"
	"time"

	reportingdto "Goshop/application/dto/reporting_dto"
	"Goshop/domain/repository"
)

type PlatformRevenueReportUsecase struct {
	revenueRepo repository.PlatformRevenueRepository
}

func NewPlatformRevenueReportUsecase(revenueRepo repository.PlatformRevenueRepository) *PlatformRevenueReportUsecase {
	return &PlatformRevenueReportUsecase{revenueRepo: revenueRepo}
}

func (uc *PlatformRevenueReportUsecase) GetBalance(ctx context.Context) (*reportingdto.PlatformRevenueBalanceResponse, error) {
	bal, err := uc.revenueRepo.GetBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("get platform balance: %w", err)
	}
	return &reportingdto.PlatformRevenueBalanceResponse{
		BalanceCents:        bal.BalanceCents,
		TotalCollectedCents: bal.TotalCollectedCents,
		UpdatedAt:           bal.UpdatedAt,
	}, nil
}

func (uc *PlatformRevenueReportUsecase) ListCommissions(
	ctx context.Context,
	from, to time.Time,
	limit, offset int,
) (*reportingdto.PlatformRevenueListResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	total, err := uc.revenueRepo.CountTransactions(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("count commissions: %w", err)
	}

	rows, err := uc.revenueRepo.ListTransactions(ctx, from, to, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list commissions: %w", err)
	}

	items := make([]reportingdto.PlatformRevenueTransactionItem, 0, len(rows))
	for _, t := range rows {
		items = append(items, reportingdto.PlatformRevenueTransactionItem{
			ID:              t.ID,
			TransactionType: t.TransactionType,
			AmountCents:     t.AmountCents,
			ReferenceType:   t.ReferenceType,
			ReferenceID:     t.ReferenceID,
			Description:     t.Description,
			CreatedAt:       t.CreatedAt,
		})
	}

	return &reportingdto.PlatformRevenueListResponse{
		Transactions: items,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
	}, nil
}
