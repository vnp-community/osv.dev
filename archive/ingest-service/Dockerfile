FROM golang:1.26-alpine AS builder
WORKDIR /build
RUN apk add --no-cache ca-certificates git
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY ingest-service/go.mod ingest-service/go.sum ./ingest-service/
WORKDIR /build/ingest-service
RUN go mod download
WORKDIR /build
COPY pkg/ ./pkg/
COPY ingest-service/ ./ingest-service/
WORKDIR /build/ingest-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /ingest-service ./cmd/server/
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /ingest-service /ingest-service
EXPOSE 8005
USER nonroot:nonroot
ENTRYPOINT ["/ingest-service"]
