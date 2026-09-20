CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS wallets (
    id TEXT PRIMARY KEY,
    balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    destination_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (source_wallet_id <> destination_wallet_id)
);

CREATE INDEX IF NOT EXISTS transfers_source_wallet_id_idx
    ON transfers (source_wallet_id, created_at DESC);

CREATE INDEX IF NOT EXISTS transfers_destination_wallet_id_idx
    ON transfers (destination_wallet_id, created_at DESC);

INSERT INTO wallets (id, balance_cents) VALUES
    ('user-1', 100000),
    ('user-2', 25000)
ON CONFLICT (id) DO NOTHING;
