package payments

import (
	"context"
	"main/internal/db"

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

func (store *SQLStore) ExecuteTransferTx(ctx context.Context, arg TransferParams) error {
	// 1. Iniciamos la transacción de Postgres
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // Si algo explota, se deshace todo por defecto

	// 2. Le inyectamos la transacción a las queries de sqlc
	qtx := store.queries.WithTx(tx)

	// 3. ACA VA TODA TU LÓGICA DE NEGOCIO EN LA DB:
	// - qtx.GetAccountForUpdate(origen)
	// - Validar saldo
	// - qtx.GetAccountForUpdate(destino)
	// - qtx.UpdateAccountBalance(...)
	// - qtx.CreateTransaction(...)

	// Si todo salió bien, guardamos los cambios de forma definitiva
	return tx.Commit(ctx)
}
