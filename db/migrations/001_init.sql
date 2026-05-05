-- Migración Goose: 001_init_schema.sql

-- +goose Up

-- Crear esquema
CREATE SCHEMA IF NOT EXISTS payments;

-- Tabla de Cuentas
CREATE TABLE payments.accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- No permitir saldos negativos a nivel DB
    CONSTRAINT positive_balance CHECK (balance >= 0)
);

-- Tabla de Transacciones (Inmutable)
CREATE TABLE payments.transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_account_id UUID NOT NULL REFERENCES payments.accounts(id),
    to_account_id UUID NOT NULL REFERENCES payments.accounts(id),
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tabla de Idempotencia
CREATE TABLE payments.idempotency_keys (
    key TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('in_progress', 'completed', 'failed')),
    response_code INTEGER,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ
);

-- Índices para performance
CREATE INDEX idx_transactions_from_account ON payments.transactions(from_account_id);
CREATE INDEX idx_transactions_to_account ON payments.transactions(to_account_id);


-- +goose Down
DROP TABLE payments.idempotency_keys, payments.transactions, payments.accounts;
