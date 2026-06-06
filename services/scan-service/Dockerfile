# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /workspace

# Copy go.work to resolve local modules
COPY go.work go.work.sum* ./

# Copy modules
COPY pkg/ pkg/
COPY proto/ proto/
COPY scan-service/ scan-service/

WORKDIR /workspace/scan-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /scan-service ./cmd/server/

# ── Runtime ─────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /scan-service /scan-service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 9102 9002
USER nonroot:nonroot
ENTRYPOINT ["/scan-service"]
