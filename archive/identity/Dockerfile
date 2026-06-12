FROM golang:1.26-alpine AS builder

WORKDIR /build

# Install ca-certs for HTTPS calls (OAuth2 etc.)
RUN apk add --no-cache ca-certificates git

# Copy go.work and all modules it references
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY auth-service/go.mod auth-service/go.sum ./auth-service/

# Download dependencies
WORKDIR /build/auth-service
RUN go mod download

# Copy source
WORKDIR /build
COPY pkg/ ./pkg/
COPY auth-service/ ./auth-service/

# Build the binary
WORKDIR /build/auth-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /auth-service ./cmd/server/main.go

# ── Runtime image ──────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /auth-service /auth-service

# Ports: HTTP :9101, gRPC :9001
EXPOSE 9101 9001

USER nonroot:nonroot
ENTRYPOINT ["/auth-service"]
