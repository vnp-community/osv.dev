FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy shared module first for cache efficiency
COPY services/shared/ ./services/shared/
COPY services/sla-service/go.mod services/sla-service/go.sum ./
RUN go mod download

COPY services/sla-service/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sla-service ./cmd/server

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /sla-service .
COPY --from=builder /app/migrations /migrations

EXPOSE 8086 9006
USER nonroot:nonroot
ENTRYPOINT ["/sla-service"]
