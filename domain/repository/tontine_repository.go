package repository

//go:generate mockgen -destination=../../mocks/repository/mock_tontine_group_repository.go -package=repository . TontineGroupRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_participant_repository.go -package=repository . TontineParticipantRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_payment_repository.go -package=repository . TontinePaymentRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_voucher_repository.go -package=repository . TontineVoucherRepository
//go:generate mockgen -destination=../../mocks/repository/mock_product_tontine_settings_repository.go -package=repository . ProductTontineSettingsRepository

import (
	"context"

	"Goshop/domain/entity"
)

// ============================================================
// ProductTontineSettingsRepository
// ============================================================

type ProductTontineSettingsRepository interface {
	FindByProductID(ctx context.Context, productID string) (*entity.ProductTontineSettings, error)
	FindByShopID(ctx context.Context, shopID string) ([]*entity.ProductTontineSettings, error)
	Upsert(ctx context.Context, settings *entity.ProductTontineSettings) error
	WithTX(tx Tx) ProductTontineSettingsRepository
}

// ============================================================
// TontineGroupRepository
// ============================================================

type TontineGroupRepository interface {
	Create(ctx context.Context, group *entity.TontineGroup) error
	FindByID(ctx context.Context, id string) (*entity.TontineGroup, error)
	FindByInviteCode(ctx context.Context, code string) (*entity.TontineGroup, error)
	FindByShopID(ctx context.Context, shopID string) ([]*entity.TontineGroup, error)
	FindByProductID(ctx context.Context, productID string) ([]*entity.TontineGroup, error)
	FindByCreatorCustomerID(ctx context.Context, customerID string) ([]*entity.TontineGroup, error)
	UpdateStatus(ctx context.Context, groupID string, status string) error
	IncrementCycle(ctx context.Context, groupID string) error
	Start(ctx context.Context, groupID string) error
	Complete(ctx context.Context, groupID string) error
	WithTX(tx Tx) TontineGroupRepository

	FindByIDUnscoped(ctx context.Context, id string) (*entity.TontineGroup, error)
}

// ============================================================
// TontineParticipantRepository
// ============================================================

type TontineParticipantRepository interface {
	Add(ctx context.Context, participant *entity.TontineParticipant) error
	FindByID(ctx context.Context, id string) (*entity.TontineParticipant, error)
	FindByGroupID(ctx context.Context, groupID string) ([]*entity.TontineParticipant, error)
	FindByGroupAndCustomer(ctx context.Context, groupID, customerID string) (*entity.TontineParticipant, error)
	FindByPosition(ctx context.Context, groupID string, position int) (*entity.TontineParticipant, error)
	CountByGroup(ctx context.Context, groupID string) (int, error)
	CountActiveByGroup(ctx context.Context, groupID string) (int, error)
	UpdateStatus(ctx context.Context, participantID string, status string) error
	WithTX(tx Tx) TontineParticipantRepository
}

// ============================================================
// TontinePaymentRepository
// ============================================================

type TontinePaymentRepository interface {
	Create(ctx context.Context, payment *entity.TontinePayment) error
	FindByID(ctx context.Context, id string) (*entity.TontinePayment, error)
	FindByIDUnscoped(ctx context.Context, id string) (*entity.TontinePayment, error) // 🆕 AJOUT PHASE 2
	SetProviderIntentID(ctx context.Context, id, intentID string) error              // 🆕 AJOUT PHASE 2
	FindByReference(ctx context.Context, reference string) (*entity.TontinePayment, error)
	FindByReferencePrefix(ctx context.Context, referencePrefix string) (*entity.TontinePayment, error)
	FindByGroupAndCycle(ctx context.Context, groupID string, cycle int) ([]*entity.TontinePayment, error)
	FindByCustomerAndGroup(ctx context.Context, customerID, groupID string) ([]*entity.TontinePayment, error)
	FindByParticipantAndCycle(ctx context.Context, participantID string, cycle int) (*entity.TontinePayment, error)
	CountDoneByGroupAndCycle(ctx context.Context, groupID string, cycle int) (int, error)
	UpdateStatus(ctx context.Context, paymentID string, status string) error
	MarkDone(ctx context.Context, paymentID string, transactionID string) error
	WithTX(tx Tx) TontinePaymentRepository

	FindDoneWithoutCommission(ctx context.Context, limit int) ([]*entity.TontinePayment, error)
	UpdateTontineCommissionStatus(ctx context.Context, paymentID string, status string, batchID *string) error

	FindByReferenceUnscoped(ctx context.Context, reference string) (*entity.TontinePayment, error)
}

// ============================================================
// TontineVoucherRepository
// ============================================================

type TontineVoucherRepository interface {
	Create(ctx context.Context, voucher *entity.TontineVoucher) error
	FindByID(ctx context.Context, id string) (*entity.TontineVoucher, error)
	FindByCode(ctx context.Context, code string) (*entity.TontineVoucher, error)
	FindByParticipantAndCycle(ctx context.Context, participantID string, cycle int) (*entity.TontineVoucher, error)
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.TontineVoucher, error)
	FindByShopID(ctx context.Context, shopID string) ([]*entity.TontineVoucher, error)
	FindActiveByShopID(ctx context.Context, shopID string) ([]*entity.TontineVoucher, error)
	Redeem(ctx context.Context, voucherCode string, redeemedBy string) error
	ExpireOldVouchers(ctx context.Context) (int, error)
	WithTX(tx Tx) TontineVoucherRepository
}
