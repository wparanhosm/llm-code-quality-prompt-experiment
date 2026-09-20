.PHONY: build test coverage lint run docker-up docker-down test-integration tidy vet clean

BINARY := bin/wallet-p2p-api
PKG    := ./...

## build: compila o binário do serviço
build:
	go build -o $(BINARY) ./cmd/wallet-p2p-api

## test: executa toda a suíte de testes unitários com race detector
test:
	go test $(PKG) -race -count=1

## coverage: executa testes e exibe a cobertura total e por função
coverage:
	go test $(PKG) -race -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

## vet: análise estática padrão do Go
vet:
	go vet $(PKG)

## lint: golangci-lint se disponível; caso contrário, fallback para go vet
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint não encontrado; executando go vet"; \
		go vet $(PKG); \
	fi

## run: inicia o serviço localmente (exige postgres rodando — veja docker-up)
run: build
	./$(BINARY)

## docker-up: sobe o PostgreSQL com migrations + seed aplicados
docker-up:
	docker compose up -d --wait

## docker-down: derruba o PostgreSQL e remove volumes
docker-down:
	docker compose down -v

## test-integration: sobe o banco e executa os testes de integração (-tags=integration)
test-integration: docker-up
	DATABASE_DSN_TEST="postgres://wallet:wallet@localhost:5432/wallet_p2p?sslmode=disable" \
		go test -tags=integration ./internal/adapter/postgres/... -race -count=1 -v

## tidy: sincroniza go.mod/go.sum
tidy:
	go mod tidy

## clean: remove artefatos de build e cobertura
clean:
	rm -rf bin coverage.out
