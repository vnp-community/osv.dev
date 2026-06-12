# syntax=docker/dockerfile:1
# ────────────────────────────────────────────────────────────────────────────
# Stage 1: Builder
# ────────────────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

# Copy shared modules first (cache-friendly)
COPY services/shared/ ./services/shared/
COPY services/auth-service/ ./services/auth-service/
COPY services/scan-service/ ./services/scan-service/
COPY services/finding-service/ ./services/finding-service/
COPY services/vulnerability-service/ ./services/vulnerability-service/
COPY services/ingestion-service/ ./services/ingestion-service/
COPY services/notification-service/ ./services/notification-service/
COPY services/report-service/ ./services/report-service/
COPY services/product-service/ ./services/product-service/
COPY services/query-service/ ./services/query-service/

# Copy app
COPY apps/OpenVulnScan/ ./apps/OpenVulnScan/

# Download dependencies
WORKDIR /workspace/apps/OpenVulnScan
RUN go mod download

# Build binary with version injection
ARG VERSION=dev
ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o /bin/openvulnscan ./cmd/server/

# ────────────────────────────────────────────────────────────────────────────
# Stage 2: Runtime (minimal Alpine)
# ────────────────────────────────────────────────────────────────────────────
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    nmap \
    ca-certificates \
    tzdata \
    wget \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1001 -S openvulnscan && \
    adduser -u 1001 -S openvulnscan -G openvulnscan

# Copy binary and static assets
COPY --from=builder /bin/openvulnscan /usr/local/bin/openvulnscan
COPY apps/OpenVulnScan/configs/ /app/configs/
COPY apps/OpenVulnScan/api/ /app/api/

# Create necessary directories
RUN mkdir -p /app/secrets /tmp/openvulnscan-reports && \
    chown -R openvulnscan:openvulnscan /app /tmp/openvulnscan-reports

WORKDIR /app

# Switch to non-root user
USER openvulnscan

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/openvulnscan"]
CMD ["--config", "/app/configs/config.docker.yaml"]
