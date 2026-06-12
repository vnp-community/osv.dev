FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Copy pkg (local replace directive)
COPY pkg/ ./pkg/

# Copy kev-service source
COPY kev-service/ ./kev-service/

# Build
WORKDIR /workspace/kev-service
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kev-service ./cmd/main.go

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /kev-service .
COPY --from=builder /workspace/kev-service/config/config.yaml ./config/

EXPOSE 8083

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8083/health || exit 1

CMD ["./kev-service"]
