package payments

import (
	"context"
	"errors"
	"main/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SQLStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewSQLStore(pool *pgxpool.Pool) *SQLStore {
	return &SQLStore{
		pool:    pool,
		queries: db.New(pool),
	}
}

func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAccountNotFound
	}

	// esto validaría el caso de saldo insuficiente.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return ErrInsufficientFunds
	}

	return err
}

func (store *SQLStore) ExecuteTransferTx(ctx context.Context, arg TransferParams) (TransferResult, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return TransferResult{}, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	qtx := store.queries.WithTx(tx)

	origenAccount, err := qtx.GetAccountForUpdate(ctx, arg.FromAccountID)
	if err != nil {
		return TransferResult{}, mapDBError(err)
	}

	destinoAccount, err := qtx.GetAccountForUpdate(ctx, arg.ToAccountID)
	if err != nil {
		return TransferResult{}, mapDBError(err)
	}

	origenAccountUpdate := db.UpdateAccountBalanceParams{
		ID:    origenAccount.ID,
		Delta: -arg.Amount,
	}

	destinoAccountUpdate := db.UpdateAccountBalanceParams{
		ID:    destinoAccount.ID,
		Delta: arg.Amount,
	}

	if origenAccount.Currency != destinoAccount.Currency {
		return TransferResult{}, ErrCurrencyMismatch
	}

	if err := qtx.UpdateAccountBalance(ctx, origenAccountUpdate); err != nil {
		return TransferResult{}, mapDBError(err)
	}

	if err := qtx.UpdateAccountBalance(ctx, destinoAccountUpdate); err != nil {
		return TransferResult{}, mapDBError(err)
	}

	txArg := db.CreateTransactionParams{
		FromAccountID: arg.FromAccountID,
		ToAccountID:   arg.ToAccountID,
		Amount:        arg.Amount,
		Currency:      string(arg.Currency),
		Status:        string(TransactionStatusCompleted),
	}

	txResult, err := qtx.CreateTransaction(ctx, txArg)

	if err != nil {
		return TransferResult{}, err
	}

	return TransferResult{
		TransactionID: txResult.ID,
		Status:        string(TransactionStatusCompleted),
	}, tx.Commit(ctx)
}
