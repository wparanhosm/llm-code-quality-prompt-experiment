-- Tabela de carteiras digitais. O saldo é armazenado em centavos (BIGINT)
-- para evitar problemas de precisão de ponto flutuante em operações
-- financeiras.
CREATE TABLE IF NOT EXISTS wallets (
    id          VARCHAR(64)  PRIMARY KEY,
    owner_id    VARCHAR(64)  NOT NULL,
    balance     BIGINT       NOT NULL CHECK (balance >= 0),
    status      VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'INACTIVE')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wallets_owner_id ON wallets (owner_id);

-- Tabela de histórico de transações de transferência P2P. Registro
-- imutável (apenas INSERT) para fins de auditoria e rastreabilidade.
CREATE TABLE IF NOT EXISTS transactions (
    id              VARCHAR(64)  PRIMARY KEY,
    from_wallet_id  VARCHAR(64)  NOT NULL REFERENCES wallets (id),
    to_wallet_id    VARCHAR(64)  NOT NULL REFERENCES wallets (id),
    amount          BIGINT       NOT NULL CHECK (amount > 0),
    status          VARCHAR(16)  NOT NULL CHECK (status IN ('COMPLETED', 'FAILED')),
    failure_reason  TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transactions_from_wallet_id ON transactions (from_wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_to_wallet_id ON transactions (to_wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at);
