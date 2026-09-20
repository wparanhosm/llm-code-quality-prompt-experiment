package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()
	if _, err := NewHTTPServer(Config{}); err == nil {
		t.Fatal("expected error")
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv, err := NewHTTPServer(Config{
		DB:     db,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Addr != ":8080" || srv.Handler == nil {
		t.Fatalf("unexpected server %+v", srv)
	}
	_ = mock
}

func TestNewHTTPServerNilLoggerCustomAddr(t *testing.T) {
	t.Parallel()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv, err := NewHTTPServer(Config{DB: db, HTTPAddr: ":9999"})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Addr != ":9999" {
		t.Fatalf("addr %s", srv.Addr)
	}
}

func TestMigrate(t *testing.T) {
	t.Parallel()
	if err := Migrate(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS wallets").WillReturnError(errors.New("fail"))
	if err := Migrate(context.Background(), db); err == nil {
		t.Fatal("expected error")
	}

	db2, mock2, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	mock2.ExpectExec("CREATE TABLE IF NOT EXISTS wallets").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := Migrate(context.Background(), db2); err != nil {
		t.Fatal(err)
	}
}
