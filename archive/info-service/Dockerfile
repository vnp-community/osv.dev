FROM golang:1.26-alpine AS builder
WORKDIR /build
RUN apk add --no-cache ca-certificates git
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY info-service/go.mod info-service/go.sum ./info-service/
WORKDIR /build/info-service
RUN go mod download
WORKDIR /build
COPY pkg/ ./pkg/
COPY info-service/ ./info-service/
WORKDIR /build/info-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /info-service ./cmd/server/
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /info-service /info-service
EXPOSE 8010
USER nonroot:nonroot
ENTRYPOINT ["/info-service"]
