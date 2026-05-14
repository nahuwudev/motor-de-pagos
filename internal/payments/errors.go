package payments

import "errors"

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientFunds   = errors.New("insufficient funds in source account")
	ErrCurrencyMismatch    = errors.New("currency mismatch between source and destination accounts")
	ErrInvalidAmount       = errors.New("transaction amount must be greater than zero")
	ErrIdempotencyConflict = errors.New("idempotency key conflict: request already in progress or completed")
	ErrSameAccountTransfer = errors.New("source and destination accounts must be different")
	ErrCurrencyRequired    = errors.New("Invalid currency used")
	ErrInvalidCurrency     = errors.New("Invalid currency")
)
