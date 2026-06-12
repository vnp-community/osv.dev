FROM golang:1.26-alpine AS builder

WORKDIR /build

# Install ca-certs for HTTPS calls (NVD API, MITRE feeds etc.)
RUN apk add --no-cache ca-certificates git

# Copy go.work and workspace modules
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY browse-service/go.mod browse-service/go.sum ./browse-service/

WORKDIR /build/browse-service
RUN go mod download

WORKDIR /build
COPY pkg/ ./pkg/
COPY browse-service/ ./browse-service/

WORKDIR /build/browse-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /browse-service ./cmd/server/

# ── Runtime ──────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /browse-service /browse-service
EXPOSE 8002
USER nonroot:nonroot
ENTRYPOINT ["/browse-service"]
