-- Habilita extensão para gerar UUIDs (opcional, se preferir gerar no banco)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Tabela de carteiras digitais.
-- balance_cents armazena o saldo em CENTAVOS (inteiro) para evitar
-- problemas de arredondamento com ponto flutuante em dinheiro.
CREATE TABLE IF NOT EXISTS wallets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL UNIQUE,
    balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tabela de transferências (ledger / histórico), útil para auditoria,
-- idempotência e reconciliação.
CREATE TABLE IF NOT EXISTS transfers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key   UUID NOT NULL UNIQUE,
    from_user_id      UUID NOT NULL,
    to_user_id        UUID NOT NULL,
    amount_cents      BIGINT NOT NULL CHECK (amount_cents > 0),
    status            TEXT NOT NULL CHECK (status IN ('COMPLETED', 'FAILED')),
    failure_reason    TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transfers_from_user ON transfers(from_user_id);
CREATE INDEX IF NOT EXISTS idx_transfers_to_user ON transfers(to_user_id);

-- Dados de exemplo para testes locais
-- INSERT INTO wallets (user_id, balance_cents) VALUES
--     ('11111111-1111-1111-1111-111111111111', 10000),
--     ('22222222-2222-2222-2222-222222222222', 0);
