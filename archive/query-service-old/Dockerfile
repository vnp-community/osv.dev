FROM golang:1.26-alpine AS builder

WORKDIR /build
RUN apk add --no-cache ca-certificates git

COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY query-service/go.mod query-service/go.sum ./query-service/

WORKDIR /build/query-service
RUN go mod download

WORKDIR /build
COPY pkg/ ./pkg/
COPY query-service/ ./query-service/

WORKDIR /build/query-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /query-service ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /query-service /query-service
EXPOSE 8005
USER nonroot:nonroot
ENTRYPOINT ["/query-service"]
