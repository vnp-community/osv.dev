# TASK-01 — Project Scaffold

## Mục Tiêu

Tạo cấu trúc thư mục chuẩn, go.mod, config system và Makefile cho GlobalCVE v3.0 monolithic Go app.

## Phụ Thuộc

- Không có (task đầu tiên)

## Đầu Ra

Skeleton project có thể `go build ./...` thành công.

---

## Checklist

- [x] Khởi tạo Go module
- [x] Tạo cấu trúc thư mục
- [x] Config system (Viper)
- [x] .env.example
- [x] Makefile
- [x] cmd/main.go (stub)

---

## 1. Khởi Tạo Go Module

```bash
cd osv.dev/apps/globalcve
go mod init github.com/binhnt/globalcve
```

**go.mod dependencies cần thêm:**

```go
require (
    github.com/go-chi/chi/v5 v5.0.12
    github.com/spf13/viper v1.18.2
    github.com/rs/zerolog v1.32.0
    github.com/jackc/pgx/v5 v5.5.5
    github.com/redis/go-redis/v9 v9.5.1
    github.com/nats-io/nats.go v1.34.0
    github.com/nats-io/nats.go/jetstream v0.0.0-20240101000000-000000000000
    github.com/robfig/cron/v3 v3.0.1
    golang.org/x/sync v0.6.0
    github.com/opensearch-project/opensearch-go/v2 v2.3.0
    github.com/pgvector/pgvector-go v0.1.1
    github.com/pressly/goose/v3 v3.20.0
)
```

---

## 2. Cấu Trúc Thư Mục

Tạo đầy đủ thư mục theo spec:

```
osv.dev/apps/globalcve/
├── cmd/
│   └── main.go                    # Entry point (stub cho TASK-01)
├── config/
│   └── config.yaml                # Config file mẫu
├── internal/
│   ├── app/
│   │   └── app.go                 # (stub)
│   ├── config/
│   │   └── config.go              # Config loader
│   ├── events/
│   │   └── events.go              # NATS event type stubs
│   ├── infra/
│   │   ├── postgres/
│   │   │   └── pool.go            # (stub)
│   │   ├── redis/
│   │   │   └── client.go          # (stub)
│   │   ├── nats/
│   │   │   └── client.go          # (stub)
│   │   └── opensearch/
│   │       └── client.go          # (stub)
│   ├── cvesearch/
│   │   ├── domain/
│   │   │   ├── entity/
│   │   │   │   └── .gitkeep
│   │   │   └── repository/
│   │   │       └── .gitkeep
│   │   ├── adapter/
│   │   │   ├── postgres/
│   │   │   │   └── .gitkeep
│   │   │   └── redis/
│   │   │       └── .gitkeep
│   │   ├── usecase/
│   │   │   └── .gitkeep
│   │   ├── http/
│   │   │   └── .gitkeep
│   │   └── service.go             # (stub)
│   ├── cvesync/
│   │   ├── domain/
│   │   │   ├── entity/
│   │   │   │   └── .gitkeep
│   │   │   └── repository/
│   │   │       └── .gitkeep
│   │   ├── fetcher/
│   │   │   └── .gitkeep
│   │   ├── adapter/
│   │   │   └── postgres/
│   │   │       └── .gitkeep
│   │   ├── usecase/
│   │   │   └── .gitkeep
│   │   ├── scheduler/
│   │   │   └── .gitkeep
│   │   └── service.go             # (stub)
│   ├── kevservice/
│   │   ├── domain/
│   │   │   ├── entity/
│   │   │   │   └── .gitkeep
│   │   │   └── repository/
│   │   │       └── .gitkeep
│   │   ├── adapter/
│   │   │   ├── postgres/
│   │   │   │   └── .gitkeep
│   │   │   └── cisa/
│   │   │       └── .gitkeep
│   │   ├── usecase/
│   │   │   └── .gitkeep
│   │   └── service.go             # (stub)
│   ├── notification/
│   │   └── service.go             # (stub)
│   └── gateway/
│       └── service.go             # (stub)
├── migrations/
│   └── .gitkeep
├── docker-compose.yml             # (stub, hoàn thiện ở TASK-10)
├── .env.example
├── Makefile
├── go.mod
└── go.sum
```

---

## 3. Config System

### `internal/config/config.go`

```go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    Server   ServerConfig
    Postgres PostgresConfig
    Redis    RedisConfig
    NATS     NATSConfig
    OpenSearch OpenSearchConfig
    Services ServicesConfig
}

type ServerConfig struct {
    Port int `mapstructure:"port"` // 8080
}

type PostgresConfig struct {
    DSN         string `mapstructure:"dsn"`
    MaxConns    int32  `mapstructure:"max_conns"`
    MinConns    int32  `mapstructure:"min_conns"`
}

type RedisConfig struct {
    Addr     string `mapstructure:"addr"`
    Password string `mapstructure:"password"`
    DB       int    `mapstructure:"db"`
}

type NATSConfig struct {
    URL string `mapstructure:"url"`
}

type OpenSearchConfig struct {
    Addresses []string `mapstructure:"addresses"`
    Username  string   `mapstructure:"username"`
    Password  string   `mapstructure:"password"`
}

type ServicesConfig struct {
    CVESearch    ServicePortConfig `mapstructure:"cve_search"`
    CVESync      ServicePortConfig `mapstructure:"cve_sync"`
    KEVService   ServicePortConfig `mapstructure:"kev_service"`
    Notification ServicePortConfig `mapstructure:"notification"`
}

type ServicePortConfig struct {
    Port int `mapstructure:"port"`
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./config")
    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }

    var cfg Config
    return &cfg, viper.Unmarshal(&cfg)
}
```

### `config/config.yaml`

```yaml
server:
  port: 8080

postgres:
  dsn: "${DATABASE_URL}"
  max_conns: 20
  min_conns: 2

redis:
  addr: "${REDIS_ADDR}"
  password: "${REDIS_PASSWORD}"
  db: 0

nats:
  url: "${NATS_URL}"

opensearch:
  addresses:
    - "${OPENSEARCH_URL}"
  username: "${OPENSEARCH_USERNAME}"
  password: "${OPENSEARCH_PASSWORD}"

services:
  cve_search:
    port: 8081
  cve_sync:
    port: 8082
  kev_service:
    port: 8083
  notification:
    port: 8084
```

---

## 4. .env.example

```dotenv
# Database
DATABASE_URL=postgres://globalcve:password@localhost:5432/globalcve?sslmode=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

# NATS
NATS_URL=nats://localhost:4222

# OpenSearch
OPENSEARCH_URL=http://localhost:9200
OPENSEARCH_USERNAME=admin
OPENSEARCH_PASSWORD=admin

# NVD API
NVD_API_KEY=your-nvd-api-key-here

# Server
APP_PORT=8080
LOG_LEVEL=info
```

---

## 5. Makefile

```makefile
.PHONY: build run dev test lint migrate-up migrate-down tidy

BINARY=globalcve
MAIN=./cmd/main.go

build:
	go build -o bin/$(BINARY) $(MAIN)

run:
	go run $(MAIN)

dev:
	air -c .air.toml

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

docker-up:
	docker compose up -d

docker-down:
	docker compose down

generate:
	go generate ./...
```

---

## 6. cmd/main.go (Stub)

```go
package main

import (
    "log"

    "github.com/binhnt/globalcve/internal/config"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }
    _ = cfg
    log.Println("GlobalCVE v3.0 starting... (stub)")
}
```

---

## Định Nghĩa Hoàn Thành

- [x] `go build ./...` không có lỗi
- [x] `go mod tidy` không có diff
- [x] Config load thành công với `.env.example`
- [x] Cấu trúc thư mục khớp với spec trong `architecture-solutions.md §8`

---

*TASK-01 | Project Scaffold | GlobalCVE v3.0*
