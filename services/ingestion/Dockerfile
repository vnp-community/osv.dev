# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -o /bin/ingestion-service ./cmd/server/

# Final stage — distroless
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/ingestion-service /ingestion-service
COPY --from=builder /app/config/config.yaml /config/config.yaml

EXPOSE 8080 9090

ENTRYPOINT ["/ingestion-service"]
