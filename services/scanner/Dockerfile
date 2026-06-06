FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy go.mod and go.sum for cache layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
    go build -ldflags="-s -w" -o /scanner ./cmd/scanner/main.go

# Runtime image
FROM scratch
COPY --from=builder /scanner /scanner
EXPOSE 50051
ENTRYPOINT ["/scanner"]
