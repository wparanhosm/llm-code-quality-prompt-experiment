-- Seed para desenvolvimento local e smoke tests.
-- Dois clientes com saldo inicial conhecido.

INSERT INTO wallets (id, balance_cents)
VALUES
    ('11111111-1111-1111-1111-111111111111', 100000), -- R$ 1.000,00
    ('22222222-2222-2222-2222-222222222222', 5000)     -- R$ 50,00
ON CONFLICT (id) DO NOTHING;
