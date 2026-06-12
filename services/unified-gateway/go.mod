module github.com/osv/unified-gateway

go 1.26.3

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/httprate v0.14.1
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.37.0
	github.com/osv/shared/pkg v0.0.0
	github.com/osv/shared/proto v0.0.0
	github.com/redis/go-redis/v9 v9.7.3
	github.com/rs/zerolog v1.33.0
	google.golang.org/grpc v1.71.0
	google.golang.org/protobuf v1.36.5
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/osv/shared/pkg => ../shared/pkg
	github.com/osv/shared/proto => ../shared/proto
)
