module github.com/osv/gateway-service

go 1.26.3

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/go-chi/cors v1.2.1
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/osv/shared/pkg v0.0.0
	github.com/osv/shared/proto v0.0.0
	github.com/rs/zerolog v1.33.0
	golang.org/x/time v0.15.0
	google.golang.org/grpc v1.81.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/redis/go-redis/v9 v9.7.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260523011958-0a33c5d7ca68 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
)

replace (
	github.com/osv/shared/pkg => ../shared/pkg
	github.com/osv/shared/proto => ../shared/proto
)
