FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build both controller and worker binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /bin/version-index-controller ./cmd/controller/
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /bin/version-index-worker ./cmd/worker/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/version-index-controller /version-index-controller
COPY --from=builder /bin/version-index-worker /version-index-worker
EXPOSE 8080 9090
# Override with CMD in docker-compose: ["controller"] or ["worker"]
ENTRYPOINT ["/version-index-controller"]
