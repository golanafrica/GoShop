package repository

import (
	"context"

	"Goshop/domain/entity"
)

//go:generate mockgen -destination=../../mocks/repository/mock_product_tontine_settings_repository.go -package=repository . ProductTontineSettingsRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_group_repository.go -package=repository . TontineGroupRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_participant_repository.go -package=repository . TontineParticipantRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_payment_repository.go -package=repository . TontinePaymentRepository
//go:generate mockgen -destination=../../mocks/repository/mock_tontine_voucher_repository.go -package=repository . TontineVoucherRepository

// ============================================================
// ProductTontineSettingsRepository
// ============================================================

// ProductTontineSettingsRepository définit les opérations sur la configuration tontine des produits
type ProductTontineSettingsRepository interface {
	// FindByProductID retourne la configuration tontine d'un produit
	FindByProductID(ctx context.Context, productID string) (*entity.ProductTontineSettings, error)

	// FindByShopID retourne toutes les configurations tontine d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.ProductTontineSettings, error)

	// Upsert crée ou met à jour la configuration tontine d'un produit
	Upsert(ctx context.Context, settings *entity.ProductTontineSettings) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) ProductTontineSettingsRepository
}

// ============================================================
// TontineGroupRepository
// ============================================================

// TontineGroupRepository définit les opérations sur les groupes de tontine
type TontineGroupRepository interface {
	// Create crée un nouveau groupe de tontine
	Create(ctx context.Context, group *entity.TontineGroup) error

	// FindByID trouve un groupe par son ID
	FindByID(ctx context.Context, id string) (*entity.TontineGroup, error)

	// FindByInviteCode trouve un groupe par son code d'invitation
	FindByInviteCode(ctx context.Context, code string) (*entity.TontineGroup, error)

	// FindByShopID retourne tous les groupes d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.TontineGroup, error)

	// FindByProductID retourne tous les groupes d'un produit
	FindByProductID(ctx context.Context, productID string) ([]*entity.TontineGroup, error)

	// FindByCreatorCustomerID retourne tous les groupes créés par un client
	FindByCreatorCustomerID(ctx context.Context, customerID string) ([]*entity.TontineGroup, error)

	// UpdateStatus met à jour le statut d'un groupe
	UpdateStatus(ctx context.Context, groupID string, status string) error

	// IncrementCycle passe le groupe au cycle suivant
	IncrementCycle(ctx context.Context, groupID string) error

	// Start démarre le groupe (passe de PENDING_MEMBERS à ACTIVE)
	Start(ctx context.Context, groupID string) error

	// Complete marque le groupe comme terminé
	Complete(ctx context.Context, groupID string) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) TontineGroupRepository
}

// ============================================================
// TontineParticipantRepository
// ============================================================

// TontineParticipantRepository définit les opérations sur les participants d'un groupe
type TontineParticipantRepository interface {
	// Add ajoute un participant à un groupe
	Add(ctx context.Context, participant *entity.TontineParticipant) error

	// FindByID trouve un participant par son ID
	FindByID(ctx context.Context, id string) (*entity.TontineParticipant, error)

	// FindByGroupID retourne tous les participants d'un groupe
	FindByGroupID(ctx context.Context, groupID string) ([]*entity.TontineParticipant, error)

	// FindByGroupAndCustomer trouve un participant par groupe et client
	FindByGroupAndCustomer(ctx context.Context, groupID, customerID string) (*entity.TontineParticipant, error)

	// FindByPosition trouve le participant à une position donnée dans un groupe
	FindByPosition(ctx context.Context, groupID string, position int) (*entity.TontineParticipant, error)

	// CountByGroup compte le nombre de participants dans un groupe
	CountByGroup(ctx context.Context, groupID string) (int, error)

	// CountActiveByGroup compte le nombre de participants actifs dans un groupe
	CountActiveByGroup(ctx context.Context, groupID string) (int, error)

	// UpdateStatus met à jour le statut d'un participant
	UpdateStatus(ctx context.Context, participantID string, status string) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) TontineParticipantRepository
}

// ============================================================
// TontinePaymentRepository
// ============================================================

// TontinePaymentRepository définit les opérations sur les paiements de cotisation
type TontinePaymentRepository interface {
	// Create crée un nouveau paiement de cotisation
	Create(ctx context.Context, payment *entity.TontinePayment) error

	// FindByID trouve un paiement par son ID
	FindByID(ctx context.Context, id string) (*entity.TontinePayment, error)

	// FindByReference trouve un paiement par sa référence YengaPay
	FindByReference(ctx context.Context, reference string) (*entity.TontinePayment, error)

	// FindByReferencePrefix trouve un paiement par préfixe de référence YengaPay
	// Utilisé par le webhook tontine qui ne connaît que les premiers caractères
	FindByReferencePrefix(ctx context.Context, referencePrefix string) (*entity.TontinePayment, error)

	// FindByGroupAndCycle retourne tous les paiements d'un groupe pour un cycle donné
	FindByGroupAndCycle(ctx context.Context, groupID string, cycle int) ([]*entity.TontinePayment, error)

	// FindByCustomerAndGroup retourne tous les paiements d'un client dans un groupe
	FindByCustomerAndGroup(ctx context.Context, customerID, groupID string) ([]*entity.TontinePayment, error)

	// FindByParticipantAndCycle trouve le paiement d'un participant pour un cycle donné
	FindByParticipantAndCycle(ctx context.Context, participantID string, cycle int) (*entity.TontinePayment, error)

	// CountDoneByGroupAndCycle compte les paiements DONE pour un groupe et un cycle
	// Utilisé pour savoir si tous les participants ont payé
	CountDoneByGroupAndCycle(ctx context.Context, groupID string, cycle int) (int, error)

	// UpdateStatus met à jour le statut d'un paiement
	UpdateStatus(ctx context.Context, paymentID string, status string) error

	// MarkDone marque un paiement comme effectué avec sa transaction ID
	MarkDone(ctx context.Context, paymentID string, transactionID string) error

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) TontinePaymentRepository

	// 🆕 v3.3.0 : Méthodes pour le scheduler de commissions
	// FindDoneWithoutCommission récupère les paiements DONE sans commission collectée
	FindDoneWithoutCommission(ctx context.Context, limit int) ([]*entity.TontinePayment, error)

	// UpdateTontineCommissionStatus met à jour le statut de commission d'un paiement tontine
	UpdateTontineCommissionStatus(ctx context.Context, paymentID string, status string, batchID *string) error
}

// ============================================================
// TontineVoucherRepository
// ============================================================

// TontineVoucherRepository définit les opérations sur les vouchers de livraison
type TontineVoucherRepository interface {
	// Create crée un nouveau voucher
	Create(ctx context.Context, voucher *entity.TontineVoucher) error

	// FindByID trouve un voucher par son ID
	FindByID(ctx context.Context, id string) (*entity.TontineVoucher, error)

	// FindByCode trouve un voucher par son code unique
	FindByCode(ctx context.Context, code string) (*entity.TontineVoucher, error)

	// FindByParticipantAndCycle trouve le voucher d'un participant pour un cycle
	FindByParticipantAndCycle(ctx context.Context, participantID string, cycle int) (*entity.TontineVoucher, error)

	// FindByCustomerID retourne tous les vouchers d'un client
	FindByCustomerID(ctx context.Context, customerID string) ([]*entity.TontineVoucher, error)

	// FindByShopID retourne tous les vouchers d'une boutique
	FindByShopID(ctx context.Context, shopID string) ([]*entity.TontineVoucher, error)

	// FindActiveByShopID retourne les vouchers actifs (generated non expirés) d'une boutique
	FindActiveByShopID(ctx context.Context, shopID string) ([]*entity.TontineVoucher, error)

	// Redeem marque un voucher comme utilisé par un marchand
	Redeem(ctx context.Context, voucherCode string, redeemedBy string) error

	// ExpireOldVouchers expire tous les vouchers dont la date est dépassée
	// Retourne le nombre de vouchers expirés
	ExpireOldVouchers(ctx context.Context) (int, error)

	// WithTX retourne le repository attaché à une transaction
	WithTX(tx Tx) TontineVoucherRepository
}
