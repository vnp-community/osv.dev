FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /alias-relations ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /alias-relations /alias-relations
EXPOSE 50051 8080 9090
ENTRYPOINT ["/alias-relations"]
