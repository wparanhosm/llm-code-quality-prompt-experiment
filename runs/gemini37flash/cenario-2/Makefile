.PHONY: all build test test-coverage lint run clean

APP_NAME=wallet-p2p-api
MAIN_SRC=cmd/api/main.go

all: test build

build:
	@echo "==> Building binary..."
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(APP_NAME) $(MAIN_SRC)

test:
	@echo "==> Running unit and race tests..."
	go test -v -race -cover ./internal/... ./pkg/...

test-coverage:
	@echo "==> Running tests with coverage report..."
	go test -v -race -coverprofile=coverage.out ./internal/... ./pkg/...
	go tool cover -func=coverage.out

lint:
	@echo "==> Checking Go formatting and vet..."
	go vet ./...
	test -z "$$(gofmt -s -l cmd internal pkg)"

run:
	@echo "==> Running server..."
	go run $(MAIN_SRC)

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin coverage.out
