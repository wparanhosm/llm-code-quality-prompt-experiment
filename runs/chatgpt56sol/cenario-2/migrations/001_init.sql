CREATE TABLE wallets (
    id TEXT PRIMARY KEY,
    balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transfers (
    id TEXT PRIMARY KEY,
    source_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    destination_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    created_at TIMESTAMPTZ NOT NULL,
    CHECK (source_wallet_id <> destination_wallet_id)
);
CREATE INDEX transfers_source_created_idx ON transfers(source_wallet_id, created_at DESC);
CREATE INDEX transfers_destination_created_idx ON transfers(destination_wallet_id, created_at DESC);
