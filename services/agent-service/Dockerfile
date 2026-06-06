# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /workspace
COPY go.work go.work.sum* ./
COPY pkg/ pkg/
COPY proto/ proto/
COPY agent-service/ agent-service/

WORKDIR /workspace/agent-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /agent-service ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /agent-service /agent-service
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 9105
USER nonroot:nonroot
ENTRYPOINT ["/agent-service"]
