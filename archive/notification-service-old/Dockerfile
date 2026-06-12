FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY pkg/ ./pkg/
COPY notification-service/ ./notification-service/
WORKDIR /workspace/notification-service
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /notification-service ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /notification-service .
COPY --from=builder /workspace/notification-service/config/config.yaml ./config/
EXPOSE 8084
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8084/health || exit 1
CMD ["./notification-service"]
