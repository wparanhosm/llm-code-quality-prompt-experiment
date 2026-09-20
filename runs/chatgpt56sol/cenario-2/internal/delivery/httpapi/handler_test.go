package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/p2p-transfer/internal/application"
	"github.com/example/p2p-transfer/internal/domain"
)

type executor struct {
	output application.TransferOutput
	err    error
	input  application.TransferInput
}

func (e *executor) Execute(_ context.Context, input application.TransferInput) (application.TransferOutput, error) {
	e.input = input
	return e.output, e.err
}
func request(t *testing.T, e *executor, body, trace string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(e, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes()
	r := httptest.NewRequest(http.MethodPost, "/v1/transfers", strings.NewReader(body))
	if trace != "" {
		r.Header.Set("X-Trace-ID", trace)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestHandler(t *testing.T) {
	tests := []struct {
		name, body string
		err        error
		status     int
	}{{"success", `{"source_wallet_id":" a ","destination_wallet_id":"b","amount":"1.00"}`, nil, 201}, {"json", "{", nil, 400}, {"unknown", `{"x":1}`, nil, 400}, {"extra", `{} {}`, nil, 400}, {"invalid", `{}`, application.ErrInvalidWallet, 400}, {"missing", `{}`, application.ErrWalletNotFound, 404}, {"funds", `{}`, domain.ErrInsufficientFunds, 409}, {"internal", `{}`, errors.New("boom"), 500}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &executor{output: application.TransferOutput{ID: "id"}, err: test.err}
			w := request(t, e, test.body, "trace")
			if w.Code != test.status {
				t.Fatalf("got %d body %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("X-Trace-ID") != "trace" {
				t.Fatal("headers")
			}
			if test.name == "success" && e.input.SourceWalletID != "a" {
				t.Fatal("not trimmed")
			}
		})
	}
}
func TestMiddlewareGeneratesAndReplacesTrace(t *testing.T) {
	for _, trace := range []string{"", strings.Repeat("x", 129)} {
		e := &executor{}
		w := request(t, e, `{"source_wallet_id":"a","destination_wallet_id":"b","amount":"1"}`, trace)
		if len(w.Header().Get("X-Trace-ID")) != 32 {
			t.Fatalf("trace %q", w.Header().Get("X-Trace-ID"))
		}
	}
	if TraceID(context.Background()) != "" {
		t.Fatal("unexpected trace")
	}
}
