package payments

import (
	"context"

	"github.com/google/uuid"
)

type TransferParams struct {
	FromAccountID uuid.UUID
	ToAccountID   uuid.UUID
	Amount        int64
	Currency      string
}

type PaymentRepository interface {
	// ExecuteTransferTx ejecuta todo el bloque ACID de la transferencia
	ExecuteTransferTx(ctx context.Context, arg TransferParams) error

	// GetBalance obtiene el balance usando la Query no bloqueante (GetAccountForRead)
	GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
}
