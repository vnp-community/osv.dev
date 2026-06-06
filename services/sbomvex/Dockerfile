FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
    go build -ldflags="-s -w" -o /sbomvex ./cmd/sbomvex/main.go

FROM scratch
COPY --from=builder /sbomvex /sbomvex
EXPOSE 50055
ENTRYPOINT ["/sbomvex"]
