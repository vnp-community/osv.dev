FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /bin/web-bff ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/web-bff /web-bff
EXPOSE 8080
ENTRYPOINT ["/web-bff"]
