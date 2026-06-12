module github.com/osv/apps/openvulnscan

go 1.26.3

require (
	// Service modules (replaced via replace directive below)
	github.com/defectdojo/finding-service v0.0.0-00010101000000-000000000000

	// Database
	github.com/jackc/pgx/v5 v5.10.0

	// NATS
	github.com/nats-io/nats.go v1.52.0
	github.com/osv/scan-service v0.0.0-00010101000000-000000000000
	github.com/osv/shared/pkg v0.0.0
	github.com/osv/vulnerability-service v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.7.3

	// Utilities
	github.com/rs/zerolog v1.35.1
	github.com/spf13/viper v1.20.1

	// gRPC + protobuf
	google.golang.org/grpc v1.81.1
)

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/osv/shared/proto v0.0.0
	go.mongodb.org/mongo-driver v1.17.9
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-chi/cors v1.2.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.3.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/osv/query-service v0.0.0-00010101000000-000000000000 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260523011958-0a33c5d7ca68 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/defectdojo/finding-service => ../../services/finding-service
	github.com/defectdojo/product-service => ../../services/product-service
	github.com/osv/auth-service => ../../services/auth-service
	github.com/osv/ingestion-service => ../../services/ingestion-service
	github.com/osv/notification-service => ../../services/notification-service
	github.com/osv/query-service => ../../services/query-service
	github.com/osv/report-service => ../../services/report-service
	github.com/osv/scan-service => ../../services/scan-service
	github.com/osv/shared/pkg => ../../services/shared/pkg
	github.com/osv/shared/proto => ../../services/shared/proto
	github.com/osv/vulnerability-service => ../../services/vulnerability-service
)
