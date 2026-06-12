FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /bin/search-service ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/search-service /search-service
EXPOSE 8080 9090
ENTRYPOINT ["/search-service"]
