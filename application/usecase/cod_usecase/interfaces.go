package codusecase

import (
	"context"

	walletusecase "Goshop/application/usecase/wallet_usecase"
	"Goshop/domain/entity"
)

//go:generate mockgen -destination=../../../mocks/usecase/mock_wallet_debiter.go -package=usecase . WalletDebiter
//go:generate mockgen -destination=../../../mocks/usecase/mock_account_freezer.go -package=usecase . AccountFreezer

// WalletDebiter définit l'interface pour débiter un wallet
type WalletDebiter interface {
	Execute(ctx context.Context, req *walletusecase.DebitWalletRequest) (*walletusecase.DebitWalletResponse, error)
	GetWallet(ctx context.Context, shopID string) (*entity.MerchantWallet, error)
}

// AccountFreezer définit l'interface pour geler un compte
type AccountFreezer interface {
	FreezeForNegativeBalance(ctx context.Context, shopID string, amountDueCents int64) (*walletusecase.FreezeAccountResponse, error)
}
