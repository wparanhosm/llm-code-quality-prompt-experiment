.PHONY: build run test test-race cover cover-html vet fmt fmt-check lint tidy docker-up docker-down

BINARY := bin/p2ptransfer

build:
	go build -o $(BINARY) ./cmd/p2ptransfer

run: build
	./$(BINARY)

test:
	go test ./...

test-race:
	go test ./... -race

cover:
	go test ./... -coverpkg=./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

cover-html: cover
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "arquivos não formatados:"; gofmt -l .; exit 1)

lint: fmt-check vet

tidy:
	go mod tidy

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v
