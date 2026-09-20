DELETE FROM transfers;
DELETE FROM wallets;
INSERT INTO wallets(id,balance_cents) VALUES ('bench-a',10000),('bench-b',2000);
