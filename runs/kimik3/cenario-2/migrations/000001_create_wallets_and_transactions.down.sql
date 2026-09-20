-- Migration 000001 (down): remove as tabelas criadas na migração up.

DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS wallets;
