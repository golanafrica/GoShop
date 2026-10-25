package payment

import "fmt"

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
