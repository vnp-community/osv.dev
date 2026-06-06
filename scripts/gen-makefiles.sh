#!/usr/bin/env bash
# scripts/gen-makefiles.sh — Generate per-service Makefiles
# Run: bash scripts/gen-makefiles.sh

SERVICES=(
  api-gateway
  vulnerability-query
  ingestion
  search
  impact-analysis
  version-index
  alias-relations
  notification
  source-sync
  web-bff
)

DOCKER_REPO="ghcr.io/osv-dev"

for SVC in "${SERVICES[@]}"; do
  MAKEFILE="services/$SVC/Makefile"
  if [ -f "$MAKEFILE" ]; then
    echo "Skipping $SVC (Makefile exists)"
    continue
  fi
  cat > "$MAKEFILE" << EOF
SERVICE := $SVC
VERSION ?= \$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_REPO := $DOCKER_REPO

.PHONY: build test test-v lint vet tidy docker clean

build: ## Build all binaries
	go build -ldflags="-X main.version=\$(VERSION)" ./cmd/...

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
	docker build -t \$(DOCKER_REPO)/\$(SERVICE):\$(VERSION) -t \$(DOCKER_REPO)/\$(SERVICE):latest .

clean: ## Remove build artifacts
	find . -name "*.test" -delete
	find . -name "coverage.out" -delete
EOF
  echo "Created $MAKEFILE"
done

echo "Done."
