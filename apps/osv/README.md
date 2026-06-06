# apps/osv — OSV.dev Web Server

Web server application cho OSV.dev. Tích hợp các microservices:

- **api-gateway** — gRPC-Gateway cho HTTP/REST API
- **vulnerability-query** — Vulnerability lookup service
- **search** — Full-text search service  
- **web-bff** — Backend-For-Frontend (Next.js BFF)

## Cấu trúc

```
apps/osv/
├── cmd/
│   ├── server/          # Main entrypoint (tổng hợp tất cả services)
│   ├── api/             # Legacy API server (từ go/cmd/api/)
│   └── api-devserver/   # Dev server (từ go/cmd/api-devserver/)
├── internal/            # App-specific logic (thin layer)
└── go.mod
```

## Dependencies

Dùng shared library từ `services/pkg` qua `replace` directives trong `go.mod`.
Legacy code từ `/go` được include tạm thời qua `github.com/google/osv.dev/go` (xem TODO trong go.mod).

## Build

```bash
go build ./cmd/server/...
go build ./cmd/api/...
```
