SERVICE := notification
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_REPO := ghcr.io/osv-dev

.PHONY: build test test-v lint vet tidy docker clean

build: ## Build all binaries
	go build -ldflags="-X main.version=$(VERSION)" ./cmd/...

test: ## Run unit tests with race detector
	go test -race -count=1 -timeout=60s ./...

test-v: ## Run unit tests verbose
	go test -race -count=1 -v -timeout=60s ./...

lint: ## Run golangci-lint
	golangci-lint run --timeout=5m ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod
	go mod tidy

docker: ## Build Docker image
	docker build -t $(DOCKER_REPO)/$(SERVICE):$(VERSION) -t $(DOCKER_REPO)/$(SERVICE):latest .

clean: ## Remove build artifacts
	find . -name "*.test" -delete
	find . -name "coverage.out" -delete
