package payments

import (
	"context"

	"github.com/google/uuid"
)

type Currency string

const (
	CurrencyARS Currency = "ARS"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
)

type TransferParams struct {
	FromAccountID uuid.UUID
	ToAccountID   uuid.UUID
	Amount        int64
	Currency      Currency
}

type TransferResult struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Status        string    `json:"status"`
}

type PaymentRepository interface {
	// ExecuteTransferTx ejecuta todo el bloque ACID de la transferencia
	ExecuteTransferTx(ctx context.Context, arg TransferParams) (TransferResult, error)

	// A futuro podría añadir: GetBalance obtiene el balance usando la Query no bloqueante (GetAccountForRead)
	// GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
}
