package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_foreign_keys=on&_busy_timeout=5000", path)
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
CREATE TABLE IF NOT EXISTS wallets (
    user_id INTEGER PRIMARY KEY,
    balance INTEGER NOT NULL CHECK (balance >= 0),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS transfers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    amount INTEGER NOT NULL CHECK (amount > 0),
    created_at DATETIME NOT NULL,
    FOREIGN KEY (from_user_id) REFERENCES wallets(user_id),
    FOREIGN KEY (to_user_id) REFERENCES wallets(user_id)
);
`)
	return err
}

func Seed(conn *sql.DB) error {
	_, err := conn.Exec(`
INSERT OR IGNORE INTO wallets (user_id, balance) VALUES (1, 100000), (2, 25000);
`)
	return err
}
