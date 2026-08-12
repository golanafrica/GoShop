package tontineusecase

import (
	"context"
	"fmt"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/repository"
	"Goshop/domain/tenant"

	"github.com/rs/zerolog"
)

type RedeemTontineVoucherRequest struct {
	VoucherCode string `json:"voucher_code"`
	RedeemedBy  string `json:"redeemed_by"` // user_id ou customer_id
}

func (r *RedeemTontineVoucherRequest) Validate() error {
	if r.VoucherCode == "" {
		return fmt.Errorf("voucher_code is required")
	}
	if r.RedeemedBy == "" {
		return fmt.Errorf("redeemed_by is required")
	}
	return nil
}

type RedeemTontineVoucherResponse struct {
	VoucherCode    string `json:"voucher_code"`
	Status         string `json:"status"`
	ReleasedCents  int64  `json:"released_cents"`
	AvailableCents int64  `json:"available_cents"`
	HeldCents      int64  `json:"held_cents"`
}

type RedeemTontineVoucherUsecase struct {
	voucherRepo   repository.TontineVoucherRepository
	releaseHeldUC *walletusecase.ReleaseHeldWalletUsecase
}

func NewRedeemTontineVoucherUsecase(
	voucherRepo repository.TontineVoucherRepository,
	releaseHeldUC *walletusecase.ReleaseHeldWalletUsecase,
) *RedeemTontineVoucherUsecase {
	return &RedeemTontineVoucherUsecase{
		voucherRepo:   voucherRepo,
		releaseHeldUC: releaseHeldUC,
	}
}

func (uc *RedeemTontineVoucherUsecase) Execute(ctx context.Context, req *RedeemTontineVoucherRequest) (*RedeemTontineVoucherResponse, error) {
	logger := zerolog.Ctx(ctx)

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	shop, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-tenant: %w", err)
	}

	voucher, err := uc.voucherRepo.FindByCode(ctx, req.VoucherCode)
	if err != nil {
		return nil, fmt.Errorf("voucher not found: %w", err)
	}

	if !voucher.IsRedeemableInShop(shop.ID.String()) {
		return nil, fmt.Errorf("voucher does not belong to this shop")
	}
	if !voucher.IsValid() {
		return nil, fmt.Errorf("voucher is not redeemable (status=%s)", voucher.Status)
	}

	// 1. Marquer redeemed en DB (atomique côté repo)
	if err := uc.voucherRepo.Redeem(ctx, req.VoucherCode, req.RedeemedBy); err != nil {
		return nil, fmt.Errorf("redeem voucher: %w", err)
	}

	released := int64(0)
	available := int64(0)
	held := int64(0)

	// 2. Libérer le held si montant > 0
	if voucher.HeldAmountCents > 0 && uc.releaseHeldUC != nil {
		refType := "tontine_voucher"
		refID := voucher.ID
		desc := fmt.Sprintf("Release held after redeem voucher %s", voucher.VoucherCode)

		resp, err := uc.releaseHeldUC.Execute(ctx, &walletusecase.ReleaseHeldWalletRequest{
			ShopID:        shop.ID.String(),
			AmountCents:   voucher.HeldAmountCents,
			ReferenceType: &refType,
			ReferenceID:   &refID,
			Description:   &desc,
		})
		if err != nil {
			// Voucher déjà redeemed : log critique, ne pas rollback le redeem
			// (idempotence métier : un 2e redeem échouera ; held peut être corrigé admin)
			logger.Error().Err(err).
				Str("voucher", voucher.VoucherCode).
				Int64("held_amount", voucher.HeldAmountCents).
				Msg("❌ Voucher redeemed but failed to release held — manual reconciliation needed")
			return nil, fmt.Errorf("voucher redeemed but release held failed: %w", err)
		}
		released = resp.ReleasedCents
		available = resp.AvailableCents
		held = resp.HeldCents
	}

	logger.Info().
		Str("voucher", voucher.VoucherCode).
		Int64("released", released).
		Msg("✅ Tontine voucher redeemed + held released")

	return &RedeemTontineVoucherResponse{
		VoucherCode:    voucher.VoucherCode,
		Status:         "redeemed",
		ReleasedCents:  released,
		AvailableCents: available,
		HeldCents:      held,
	}, nil
}
