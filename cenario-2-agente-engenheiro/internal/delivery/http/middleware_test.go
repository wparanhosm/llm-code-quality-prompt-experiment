package http_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	delivery "p2p-wallet/internal/delivery/http"
)

func TestCORSMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := delivery.CORSMiddleware(inner)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected CORS header")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCORSMiddleware_Options(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach inner handler on OPTIONS")
	})
	h := delivery.CORSMiddleware(inner)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestCorrelationMiddleware_GeneratesID(t *testing.T) {
	gen := func() string { return "gen-123" }
	var gotHeader string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = w.Header().Get("X-Correlation-ID")
	})

	h := delivery.CorrelationMiddleware(gen)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if gotHeader != "gen-123" {
		t.Fatalf("expected gen-123, got %s", gotHeader)
	}
}

func TestCorrelationMiddleware_UsesExisting(t *testing.T) {
	gen := func() string { return "should-not-use" }
	var gotHeader string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = w.Header().Get("X-Correlation-ID")
	})

	h := delivery.CorrelationMiddleware(gen)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Correlation-ID", "existing-456")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if gotHeader != "existing-456" {
		t.Fatalf("expected existing-456, got %s", gotHeader)
	}
}

func TestRecoverMiddleware(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	h := delivery.RecoverMiddleware(log)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestChain(t *testing.T) {
	var order []string
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	h := delivery.Chain(inner, m1, m2)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if len(order) != 3 || order[0] != "m1" || order[1] != "m2" || order[2] != "handler" {
		t.Fatalf("expected [m1, m2, handler], got %v", order)
	}
}
