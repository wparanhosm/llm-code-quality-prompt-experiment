-- Dados de exemplo para facilitar testes manuais locais do endpoint de
-- transferência. Não deve ser aplicada em ambientes de produção.
INSERT INTO wallets (id, owner_id, balance, status) VALUES
    ('wallet-alice', 'user-alice', 100000, 'ACTIVE'),
    ('wallet-bob',   'user-bob',   50000,  'ACTIVE'),
    ('wallet-carol', 'user-carol', 0,      'INACTIVE')
ON CONFLICT (id) DO NOTHING;
