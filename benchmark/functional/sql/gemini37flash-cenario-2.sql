DELETE FROM transfers;
DELETE FROM accounts;
INSERT INTO accounts(id,holder,balance_cents,status) VALUES (101,'Benchmark A',10000,'ACTIVE'),(102,'Benchmark B',2000,'ACTIVE');
