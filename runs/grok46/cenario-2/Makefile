.PHONY: test lint build coverage tidy

tidy:
	go mod tidy

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

lint:
	go vet ./...

build:
	go build -o bin/p2p-transfer ./cmd/p2p-transfer
