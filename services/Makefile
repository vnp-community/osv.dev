.PHONY: proto build-all test-all lint-all up-ovs down migrate generate-jwt-keys clean

SERVICES := api-gateway auth-service scan-service asset-service cve-service \
            agent-service schedule-service report-service

# ── Proto ────────────────────────────────────────────────────────────────────
proto:
	cd proto && buf generate

# ── Build ────────────────────────────────────────────────────────────────────
build-all:
	@for svc in $(SERVICES); do \
		echo "==> Building $$svc..."; \
		(cd $$svc && go build -o bin/$$svc ./cmd/server) || exit 1; \
	done
	@echo "✅ All services built successfully"

# ── Test ─────────────────────────────────────────────────────────────────────
test-all:
	@for svc in $(SERVICES); do \
		echo "==> Testing $$svc..."; \
		(cd $$svc && go test ./... -short -count=1) || exit 1; \
	done

# ── Lint ─────────────────────────────────────────────────────────────────────
lint-all:
	@for svc in $(SERVICES); do \
		echo "==> Linting $$svc..."; \
		(cd $$svc && golangci-lint run ./...); \
	done

# ── Docker ───────────────────────────────────────────────────────────────────
up-ovs:
	docker compose -f docker-compose.ovs.yml --env-file docker-compose.ovs.env up -d
	@echo "🚀 OpenVulnScan stack started. Gateway: http://localhost:8443"

down:
	docker compose -f docker-compose.ovs.yml down

up-infra:
	docker compose -f docker-compose.ovs.yml up -d postgres redis nats minio

# ── Migrations ────────────────────────────────────────────────────────────────
migrate:
	@echo "Creating schemas..."
	psql $$DATABASE_URL -f scripts/create_schemas.sql
	@for svc in $(SERVICES); do \
		if [ -d "$$svc/migrations" ]; then \
			echo "==> Migrating $$svc..."; \
			psql $$DATABASE_URL -f $$svc/migrations/001_initial_schema.sql; \
		fi; \
	done
	@echo "✅ All migrations complete"

# ── JWT Keys ─────────────────────────────────────────────────────────────────
generate-jwt-keys:
	@mkdir -p secrets
	openssl genrsa -out secrets/jwt_private.pem 4096
	openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
	@echo "✅ JWT keys written to secrets/"

# ── Tidy ─────────────────────────────────────────────────────────────────────
tidy-all:
	@for svc in $(SERVICES) pkg; do \
		echo "==> go mod tidy $$svc..."; \
		(cd $$svc && go mod tidy); \
	done

clean:
	@for svc in $(SERVICES); do \
		rm -rf $$svc/bin; \
	done

.DEFAULT_GOAL := build-all
