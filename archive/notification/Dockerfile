FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /notification ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /notification /notification
EXPOSE 50051 8080 9090
ENTRYPOINT ["/notification"]
