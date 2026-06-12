FROM golang:1.26-alpine AS builder
WORKDIR /build
RUN apk add --no-cache ca-certificates git
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY ranking-service/go.mod ranking-service/go.sum ./ranking-service/
WORKDIR /build/ranking-service
RUN go mod download
WORKDIR /build
COPY pkg/ ./pkg/
COPY ranking-service/ ./ranking-service/
WORKDIR /build/ranking-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /ranking-service ./cmd/server/
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /ranking-service /ranking-service
EXPOSE 8006 9051
USER nonroot:nonroot
ENTRYPOINT ["/ranking-service"]
