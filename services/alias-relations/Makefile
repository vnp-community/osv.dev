SERVICE   = alias-relations
DOCKER_IMG = gcr.io/osv-dev/$(SERVICE)

.PHONY: build test lint docker run

build:
	go build ./...

test:
	go test -race -count=1 ./...

test-unit:
	go test -race -count=1 ./internal/domain/...

lint:
	golangci-lint run ./...

docker:
	docker build -t $(DOCKER_IMG):latest .

run:
	go run ./cmd/server/main.go

proto:
	protoc --go_out=. --go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		interface/grpc/proto/alias_relations_service.proto

tidy:
	go mod tidy
