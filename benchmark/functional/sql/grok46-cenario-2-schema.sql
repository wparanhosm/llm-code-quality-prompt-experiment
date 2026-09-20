CREATE TABLE IF NOT EXISTS wallets (id TEXT PRIMARY KEY, balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0));
CREATE TABLE IF NOT EXISTS transfers (
  id TEXT PRIMARY KEY, from_wallet_id TEXT NOT NULL REFERENCES wallets(id), to_wallet_id TEXT NOT NULL REFERENCES wallets(id),
  amount_cents BIGINT NOT NULL CHECK (amount_cents > 0), created_at TIMESTAMPTZ NOT NULL
);
