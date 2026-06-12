# ============================================
# gateway-service — Multi-stage Docker Build
# ============================================

# Stage 1: Builder
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy shared libraries first (better layer caching)
COPY services/shared/pkg ./services/shared/pkg
COPY services/shared/proto ./services/shared/proto

# Copy service go.mod + go.sum for dependency caching
COPY services/gateway-service/go.mod services/gateway-service/go.sum ./services/gateway-service/

WORKDIR /build/services/gateway-service
RUN go mod download

# Copy service source code
COPY services/gateway-service/ .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o /bin/service ./cmd/server/

# Stage 2: Runtime (minimal distroless)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/service /service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/service"]
