# ============================================
# osv-server — Multi-stage Docker Build
# ============================================

# Stage 1: Builder
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy shared libraries and local module replacements required by go.mod
COPY services/shared/pkg ./services/shared/pkg
COPY services/shared/proto ./services/shared/proto
COPY services/api-gateway ./services/api-gateway
COPY services/search ./services/search
COPY services/vulnerability-query ./services/vulnerability-query
COPY services/web-bff ./services/web-bff

# Copy app go.mod + go.sum
COPY apps/osv/go.mod apps/osv/go.sum ./apps/osv/

WORKDIR /build/apps/osv
RUN go mod download

# Copy service source code
COPY apps/osv/ .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o /bin/osv-server ./cmd/server/

# Stage 2: Runtime (minimal distroless)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/osv-server /service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/service"]
