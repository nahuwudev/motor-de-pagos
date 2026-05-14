-- name: GetAccountForUpdate :one
-- Bloqueamos la fila para evitar race conditions al modificar el saldo
SELECT id, owner_id, balance, currency, created_at, updated_at
FROM payments.accounts
WHERE id = $1
FOR NO KEY UPDATE;

-- name: GetAccountForRead :one
-- Lectura simple no bloqueante.
SELECT id, owner_id, balance, currency, created_at, updated_at
FROM payments.accounts
WHERE id = $1;

-- name: UpdateAccountBalance :exec
-- Actualizamos el saldo y el timestamp
UPDATE payments.accounts
SET balance = balance + sqlc.arg(delta), 
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: CreateTransaction :one
-- Registramos el movimiento inmutable
INSERT INTO payments.transactions (
    from_account_id, 
    to_account_id, 
    amount, 
    currency, 
    status
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, from_account_id, to_account_id, amount, currency, status, created_at;

-- name: GetIdempotencyKey :one
-- Buscamos si la clave ya existe para evitar doble procesamiento
SELECT key, status, response_code, response_body, created_at, locked_at
FROM payments.idempotency_keys
WHERE key = $1;

-- name: CreateIdempotencyKey :one
INSERT INTO payments.idempotency_keys (
    -- lockd_At responde a "hace cuanto esta pasando"
    -- status responde a "que esta pasando"
    key, status, locked_at 
) VALUES (
    $1, $2, NOW()
)
ON CONFLICT (key) DO NOTHING
RETURNING key, status, response_code, response_body, created_at, locked_at;

-- name: UpdateIdempotencyKey :exec
-- Actualizamos el resultado de la operación idempotente
UPDATE payments.idempotency_keys
SET status = $2, 
    response_code = $3, 
    response_body = $4, 
    locked_at = NULL
WHERE key = $1;

-- name: ListAccountTransactions :many
-- Listamos las transacciones donde la cuenta es origen o destino, con paginación
SELECT id, from_account_id, to_account_id, amount, currency, status, created_at
FROM payments.transactions
WHERE from_account_id = $1 OR to_account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateAccount :one
-- Creamos una cuenta
INSERT INTO payments.accounts (
    owner_id,
    balance,
    currency
) VALUES (
    $1, $2, $3
)
RETURNING id, owner_id, balance, currency, created_at, updated_at;