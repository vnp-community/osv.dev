FROM golang:1.26-alpine AS builder
WORKDIR /build
RUN apk add --no-cache ca-certificates git
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY taxonomy-service/go.mod taxonomy-service/go.sum ./taxonomy-service/
WORKDIR /build/taxonomy-service
RUN go mod download
WORKDIR /build
COPY pkg/ ./pkg/
COPY taxonomy-service/ ./taxonomy-service/
WORKDIR /build/taxonomy-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /taxonomy-service ./cmd/server/
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /taxonomy-service /taxonomy-service
EXPOSE 8009 9052
USER nonroot:nonroot
ENTRYPOINT ["/taxonomy-service"]
