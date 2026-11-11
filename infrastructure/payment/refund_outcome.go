package payment

import "fmt"

// ============================================================
// Erreurs de remboursement (Refund) - PR-D
// ============================================================

// ErrRefundDefinitive : Le prestataire a explicitement rejeté la demande (ex: 4xx).
// Action : Annuler la réservation locale immédiatement.
type ErrRefundDefinitive struct {
	HTTPStatus int
	Message    string
}

func (e *ErrRefundDefinitive) Error() string {
	return fmt.Sprintf("refund definitive rejection (status %d): %s", e.HTTPStatus, e.Message)
}

// ErrRefundUncertain : Le prestataire a échoué à répondre (ex: 5xx, timeout, réseau).
// Action : Conserver la réservation locale, passer en 'reconciliation_required'.
type ErrRefundUncertain struct {
	HTTPStatus int
	Cause      error
}

func (e *ErrRefundUncertain) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("refund outcome unknown (status %d): %v", e.HTTPStatus, e.Cause)
	}
	return fmt.Sprintf("refund outcome unknown (status %d)", e.HTTPStatus)
}

func (e *ErrRefundUncertain) Unwrap() error { return e.Cause }