.PHONY: build test lint migrate generate clean

SERVICE := auth-service
BINARY  := bin/$(SERVICE)
MAIN    := ./cmd/server/main.go

build:
	@mkdir -p bin
	go build -o $(BINARY) $(MAIN)

run: build
	./$(BINARY)

test:
	go test ./... -race -coverprofile=coverage.out

test-short:
	go test ./... -short

lint:
	golangci-lint run ./...

migrate:
	@echo "Running migrations..."
	psql $$DATABASE_URL -f migrations/001_initial_schema.sql

generate-key:
	@echo "Generating RSA-4096 private key..."
	@mkdir -p secrets
	openssl genrsa -out secrets/jwt_private.pem 4096
	openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
	@echo "Keys written to secrets/"

clean:
	rm -rf bin/ coverage.out

tidy:
	go mod tidy

.DEFAULT_GOAL := build
