# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /workspace
COPY go.work go.work.sum* ./
COPY pkg/ pkg/
COPY proto/ proto/
COPY schedule-service/ schedule-service/

WORKDIR /workspace/schedule-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /schedule-service ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /schedule-service /schedule-service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 9106
USER nonroot:nonroot
ENTRYPOINT ["/schedule-service"]
