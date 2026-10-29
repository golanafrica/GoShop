package payment

import "fmt"

// ============================================================
// Erreurs d'initiation de paiement (Pay-in)
// ============================================================

// ErrPaymentInitDefinitive : refus métier explicite (4xx). Le paiement n'a pas été initié.
type ErrPaymentInitDefinitive struct {
	HTTPStatus int
	Message    string
}

func (e *ErrPaymentInitDefinitive) Error() string {
	return fmt.Sprintf("payment initiation definitive rejection (status %d): %s", e.HTTPStatus, e.Message)
}

// ErrPaymentInitUnknown : timeout, réseau, 5xx. Le prestataire a peut-être initié le paiement.
type ErrPaymentInitUnknown struct {
	HTTPStatus int
	Cause      error
}

func (e *ErrPaymentInitUnknown) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("payment initiation outcome unknown (status %d): %v", e.HTTPStatus, e.Cause)
	}
	return fmt.Sprintf("payment initiation outcome unknown (status %d)", e.HTTPStatus)
}

func (e *ErrPaymentInitUnknown) Unwrap() error { return e.Cause }

// ============================================================
// Erreurs de retrait (Cash-out)
// ============================================================

// ErrCashOutDefinitive : refus métier explicite (4xx). Reverse portefeuille autorisé.
type ErrCashOutDefinitive struct {
	HTTPStatus int
	Message    string
}

func (e *ErrCashOutDefinitive) Error() string {
	return fmt.Sprintf("cash-out definitive rejection (status %d): %s", e.HTTPStatus, e.Message)
}

// ErrCashOutUnknown : timeout, réseau, 5xx, parse. Ne jamais reverse automatiquement.
type ErrCashOutUnknown struct {
	HTTPStatus int
	Cause      error
}

func (e *ErrCashOutUnknown) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cash-out outcome unknown (status %d): %v", e.HTTPStatus, e.Cause)
	}
	return fmt.Sprintf("cash-out outcome unknown (status %d)", e.HTTPStatus)
}

func (e *ErrCashOutUnknown) Unwrap() error { return e.Cause }
