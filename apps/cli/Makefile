# CLI Tools Makefile
# Build all CLI binaries to bin/
BIN_DIR := bin
GO := go
GOFLAGS := -trimpath

CMDS := $(notdir $(wildcard cmd/*))

.PHONY: all build clean test lint

all: build

build: $(CMDS)

$(CMDS):
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$@ ./cmd/$@/...
	@echo "Built: $(BIN_DIR)/$@"

clean:
	rm -rf $(BIN_DIR)

test:
	$(GO) test ./...

lint:
	golangci-lint run ./...

.PHONY: $(CMDS)
