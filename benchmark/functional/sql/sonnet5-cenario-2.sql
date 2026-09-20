DELETE FROM transactions;
DELETE FROM wallets;
INSERT INTO wallets(id,owner_id,balance,status) VALUES ('bench-a','owner-a',10000,'ACTIVE'),('bench-b','owner-b',2000,'ACTIVE');
