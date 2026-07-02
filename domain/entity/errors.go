package entity

import "errors"

// ============================================================
// ERREURS DE VALIDATION (Commission)
// ============================================================

var (
	// CommissionRate validation errors
	ErrInvalidShopID          = errors.New("invalid shop ID")
	ErrInvalidTransactionType = errors.New("invalid transaction type")
	ErrInvalidCommissionRate  = errors.New("commission rate must be between 0 and 1500 bps (15%)")
	ErrInvalidMinCommission   = errors.New("min commission must be positive")
	ErrInvalidMaxCommission   = errors.New("max commission must be positive")
	ErrMinGreaterThanMax      = errors.New("min commission cannot be greater than max")
)

// ============================================================
// ERREURS MÉTIER GÉNÉRIQUES
// ============================================================

var (
	ErrNotFound          = errors.New("resource not found")
	ErrInvalidID         = errors.New("invalid ID")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrAlreadyExists     = errors.New("resource already exists")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInsufficientFunds = errors.New("insufficient funds")
)
