package reportusecase

import (
	"context"
	"fmt"
	"time"

	reportingdto "Goshop/application/dto/reporting_dto"
	"Goshop/domain/entity"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"
)

type MerchantWalletStatementUsecase struct {
	walletRepo    repository.MerchantWalletRepository
	walletTxnRepo repository.WalletTransactionRepository
}

func NewMerchantWalletStatementUsecase(
	walletRepo repository.MerchantWalletRepository,
	walletTxnRepo repository.WalletTransactionRepository,
) *MerchantWalletStatementUsecase {
	return &MerchantWalletStatementUsecase{
		walletRepo:    walletRepo,
		walletTxnRepo: walletTxnRepo,
	}
}

func (uc *MerchantWalletStatementUsecase) ListTransactions(
	ctx context.Context,
	from, to time.Time,
	limit, offset int,
) ([]reportingdto.WalletTransactionItem, int, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	total, err := uc.walletTxnRepo.CountByShopID(ctx, shopID)
	if err != nil {
		return nil, 0, fmt.Errorf("count wallet txns: %w", err)
	}

	var rows []*entity.WalletTransaction
	if !from.IsZero() || !to.IsZero() {
		start, end := from, to
		if start.IsZero() {
			start = time.Unix(0, 0).UTC()
		}
		if end.IsZero() {
			end = time.Now().UTC().Add(24 * time.Hour)
		}
		rows, err = uc.walletTxnRepo.FindByDateRange(ctx, shopID, start, end)
		if err != nil {
			return nil, 0, fmt.Errorf("find wallet txns by date: %w", err)
		}
		// pagination manuelle sur le résultat date-range
		total = len(rows)
		if offset > len(rows) {
			rows = nil
		} else {
			endIdx := offset + limit
			if endIdx > len(rows) {
				endIdx = len(rows)
			}
			rows = rows[offset:endIdx]
		}
	} else {
		rows, err = uc.walletTxnRepo.FindByShopIDPaginated(ctx, shopID, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("find wallet txns paginated: %w", err)
		}
	}

	items := mapWalletTxns(rows)
	return items, total, nil
}

func (uc *MerchantWalletStatementUsecase) GetStatement(
	ctx context.Context,
	from, to time.Time,
) (*reportingdto.MerchantStatementResponse, error) {
	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}
	shopID := shop.ID.String()

	if from.IsZero() {
		from = time.Now().UTC().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}

	credits, err := uc.walletTxnRepo.SumCreditsByDateRange(ctx, shopID, from, to)
	if err != nil {
		return nil, fmt.Errorf("sum credits: %w", err)
	}
	debits, err := uc.walletTxnRepo.SumDebitsByDateRange(ctx, shopID, from, to)
	if err != nil {
		return nil, fmt.Errorf("sum debits: %w", err)
	}

	rows, err := uc.walletTxnRepo.FindByDateRange(ctx, shopID, from, to)
	if err != nil {
		return nil, fmt.Errorf("find wallet txns: %w", err)
	}

	var closing int64
	wallet, werr := uc.walletRepo.FindByShopID(ctx, shopID)
	if werr == nil && wallet != nil {
		closing = wallet.BalanceCents
	}
	opening := closing - credits + debits

	return &reportingdto.MerchantStatementResponse{
		ShopID:           shopID,
		PeriodStart:      from,
		PeriodEnd:        to,
		OpeningBalance:   opening,
		ClosingBalance:   closing,
		TotalCredits:     credits,
		TotalDebits:      debits,
		Transactions:     mapWalletTxns(rows),
		TransactionCount: len(rows),
	}, nil
}

func mapWalletTxns(rows []*entity.WalletTransaction) []reportingdto.WalletTransactionItem {
	items := make([]reportingdto.WalletTransactionItem, 0, len(rows))
	for _, t := range rows {
		items = append(items, reportingdto.WalletTransactionItem{
			ID:                t.ID,
			ShopID:            t.ShopID,
			TransactionType:   string(t.TransactionType),
			AmountCents:       t.AmountCents,
			BalanceAfterCents: t.BalanceAfterCents,
			ReferenceType:     t.ReferenceType,
			ReferenceID:       t.ReferenceID,
			Description:       t.Description,
			Status:            string(t.Status),
			CreatedAt:         t.CreatedAt,
		})
	}
	return items
}
