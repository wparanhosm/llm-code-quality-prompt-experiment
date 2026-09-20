package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wallet/p2p-transfer/internal/domain"
	"github.com/wallet/p2p-transfer/internal/usecase"
)

type stubUC struct {
	fn func(ctx context.Context, in usecase.TransferInput) (*domain.Transfer, error)
}

func (s stubUC) Execute(ctx context.Context, in usecase.TransferInput) (*domain.Transfer, error) {
	return s.fn(ctx, in)
}

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHealth(t *testing.T) {
	t.Parallel()
	h := NewHandler(stubUC{}, logger())
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if res.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing request id")
	}
}

func TestCreateTransfer(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	h := NewHandler(stubUC{fn: func(ctx context.Context, in usecase.TransferInput) (*domain.Transfer, error) {
		return &domain.Transfer{
			ID: "t1", FromWalletID: in.FromWalletID, ToWalletID: in.ToWalletID,
			Amount: in.Amount, CreatedAt: now,
		}, nil
	}}, logger())
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	body := `{"from_wallet_id":"a","to_wallet_id":"b","amount":150}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/transfers", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "rid-1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", res.StatusCode)
	}
	if res.Header.Get("X-Request-ID") != "rid-1" {
		t.Fatalf("rid %s", res.Header.Get("X-Request-ID"))
	}
	var got transferResponse
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "t1" || got.Amount != 150 {
		t.Fatalf("%+v", got)
	}
}

func TestCreateTransferErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		body   string
		ucErr  error
		status int
	}{
		{"bad json", `{`, nil, http.StatusBadRequest},
		{"empty", "", nil, http.StatusBadRequest},
		{"same wallet", `{"from_wallet_id":"a","to_wallet_id":"b","amount":1}`, domain.ErrSameWalletTransfer, http.StatusBadRequest},
		{"not found", `{"from_wallet_id":"a","to_wallet_id":"b","amount":1}`, domain.ErrWalletNotFound, http.StatusNotFound},
		{"funds", `{"from_wallet_id":"a","to_wallet_id":"b","amount":1}`, domain.ErrInsufficientFunds, http.StatusConflict},
		{"invalid amount", `{"from_wallet_id":"a","to_wallet_id":"b","amount":1}`, domain.ErrInvalidAmount, http.StatusBadRequest},
		{"invalid id", `{"from_wallet_id":"a","to_wallet_id":"b","amount":1}`, domain.ErrInvalidWalletID, http.StatusBadRequest},
		{"internal", `{"from_wallet_id":"a","to_wallet_id":"b","amount":1}`, errorsSentinel, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(stubUC{fn: func(context.Context, usecase.TransferInput) (*domain.Transfer, error) {
				if tc.ucErr != nil {
					return nil, tc.ucErr
				}
				return &domain.Transfer{ID: "x"}, nil
			}}, logger())
			srv := httptest.NewServer(h.Routes())
			defer srv.Close()

			res, err := http.Post(srv.URL+"/transfers", "application/json", bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status {
				t.Fatalf("status %d want %d", res.StatusCode, tc.status)
			}
		})
	}
}

var errorsSentinel = errSentinel("boom")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestNewHandlerNilLogger(t *testing.T) {
	t.Parallel()
	h := NewHandler(stubUC{}, nil)
	if h.logger == nil {
		t.Fatal("logger")
	}
}
