CREATE TABLE IF NOT EXISTS wallets (
    id         VARCHAR(36) PRIMARY KEY,
    balance    BIGINT      NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transactions (
    id              VARCHAR(36) PRIMARY KEY,
    from_wallet_id  VARCHAR(36) NOT NULL REFERENCES wallets(id),
    to_wallet_id    VARCHAR(36) NOT NULL REFERENCES wallets(id),
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    status          VARCHAR(20) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_from ON transactions(from_wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_to   ON transactions(to_wallet_id);
