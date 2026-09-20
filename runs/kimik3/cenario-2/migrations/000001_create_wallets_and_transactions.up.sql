-- Migration 000001: criação das tabelas de carteiras e transações.
-- Valores monetários em centavos (bigint) para evitar erros de ponto flutuante.
-- Saldos NUNCA podem ser negativos (CHECK), reforçando a invariante do domínio.

CREATE TABLE IF NOT EXISTS wallets (
    id            UUID PRIMARY KEY,
    balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id              UUID PRIMARY KEY,
    from_wallet_id  UUID NOT NULL REFERENCES wallets (id),
    to_wallet_id    UUID NOT NULL REFERENCES wallets (id),
    amount_cents    BIGINT NOT NULL CHECK (amount_cents > 0),
    status          TEXT NOT NULL CHECK (status IN ('COMPLETED')),
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transactions_different_wallets CHECK (from_wallet_id <> to_wallet_id)
);

-- Chave de idempotência única quando presente (múltiplos NULLs permitidos).
CREATE UNIQUE INDEX IF NOT EXISTS transactions_idempotency_key_uidx
    ON transactions (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS transactions_from_wallet_id_idx ON transactions (from_wallet_id);
CREATE INDEX IF NOT EXISTS transactions_to_wallet_id_idx ON transactions (to_wallet_id);
