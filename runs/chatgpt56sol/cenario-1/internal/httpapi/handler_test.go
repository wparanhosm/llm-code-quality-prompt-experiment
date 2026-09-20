package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pixp2p/internal/transfer"
)

type serviceStub struct {
	result transfer.Transfer
	err    error
}

func (s serviceStub) Execute(context.Context, string, string, int64) (transfer.Transfer, error) {
	return s.result, s.err
}

func TestCreateTransfer(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		body       string
		service    serviceStub
		wantStatus int
	}{
		{
			name: "success",
			body: `{"source_user_id":"user-1","destination_user_id":"user-2","amount":"10.50"}`,
			service: serviceStub{result: transfer.Transfer{
				ID: "transfer-1", SourceID: "user-1", DestinationID: "user-2", AmountCents: 1050, CreatedAt: createdAt,
			}},
			wantStatus: http.StatusCreated,
		},
		{name: "malformed JSON", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "invalid amount", body: `{"source_user_id":"user-1","destination_user_id":"user-2","amount":"1.001"}`, wantStatus: http.StatusBadRequest},
		{name: "insufficient funds", body: `{"source_user_id":"user-1","destination_user_id":"user-2","amount":"10.00"}`, service: serviceStub{err: transfer.ErrInsufficientFunds}, wantStatus: http.StatusConflict},
		{name: "wallet missing", body: `{"source_user_id":"user-1","destination_user_id":"missing","amount":"10.00"}`, service: serviceStub{err: transfer.ErrWalletNotFound}, wantStatus: http.StatusNotFound},
		{name: "internal error", body: `{"source_user_id":"user-1","destination_user_id":"user-2","amount":"10.00"}`, service: serviceStub{err: errors.New("database unavailable")}, wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/transfers", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			NewHandler(test.service).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
