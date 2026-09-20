.PHONY: fmt lint test test-race coverage build run
fmt:
	gofmt -w $$(find cmd internal pkg -name '*.go')
lint:
	go vet ./...
test:
	go test ./...
test-race:
	go test -race ./...
coverage:
	go test -coverprofile=coverage.out ./internal/... ./pkg/...
	./scripts/check-coverage.sh coverage.out 95
build:
	go build ./cmd/api
run:
	go run ./cmd/api
