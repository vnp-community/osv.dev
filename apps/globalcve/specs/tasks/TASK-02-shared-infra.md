# TASK-02 — Shared Infrastructure

## Mục Tiêu

Implement các shared infrastructure clients: PostgreSQL (pgx pool), Redis, NATS JetStream, OpenSearch. Đây là layer được inject vào tất cả các services qua Dependency Injection.

## Phụ Thuộc

- TASK-01 (Project Scaffold)

## Đầu Ra

- `internal/infra/postgres/pool.go` — pgx connection pool
- `internal/infra/redis/client.go` — go-redis client
- `internal/infra/nats/client.go` — NATS JetStream client + stream provisioning
- `internal/infra/opensearch/client.go` — OpenSearch client
- `internal/events/events.go` — NATS event type definitions

---

## Checklist

- [x] PostgreSQL connection pool với health check
- [x] Redis client với health check
- [x] NATS client + tạo 3 streams (CVE_EVENTS, KEV_EVENTS, ALERT_EVENTS)
- [x] OpenSearch client
- [x] Event type definitions

---

## 1. PostgreSQL Pool (`internal/infra/postgres/pool.go`)

```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/binhnt/globalcve/internal/config"
)

func NewPool(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
    poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
    if err != nil {
        return nil, fmt.Errorf("postgres: parse config: %w", err)
    }

    poolCfg.MaxConns = cfg.MaxConns
    poolCfg.MinConns = cfg.MinConns

    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil {
        return nil, fmt.Errorf("postgres: create pool: %w", err)
    }

    // Health check — fail-fast nếu DB không available
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("postgres: ping failed: %w", err)
    }

    return pool, nil
}
```

> **Lưu ý:** Database không available → fail-fast (app không start). Xem §5.3 của architecture-solutions.md.

---

## 2. Redis Client (`internal/infra/redis/client.go`)

```go
package redis

import (
    "context"
    "fmt"

    goredis "github.com/redis/go-redis/v9"
    "github.com/binhnt/globalcve/internal/config"
)

func NewClient(ctx context.Context, cfg config.RedisConfig) (*goredis.Client, error) {
    client := goredis.NewClient(&goredis.Options{
        Addr:     cfg.Addr,
        Password: cfg.Password,
        DB:       cfg.DB,
    })

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis: ping failed: %w", err)
    }

    return client, nil
}
```

### Cache Key Constants

```go
// cache_keys.go
package redis

import (
    "crypto/md5"
    "fmt"
)

func SearchKey(queryHash string) string {
    return fmt.Sprintf("search:%s", queryHash)
}

func CVEKey(cveID string) string {
    return fmt.Sprintf("cve:%s", cveID)
}

func RateLimitKey(ip string) string {
    return fmt.Sprintf("rl:ip:%s", ip)
}

func HashQuery(query string) string {
    return fmt.Sprintf("%x", md5.Sum([]byte(query)))
}
```

**TTL Values:**
| Key Pattern | TTL |
|-------------|-----|
| `search:{hash}` | 5 phút |
| `cve:{CVE-ID}` | 60 phút |
| `rl:ip:{ip}` | Rolling window |

---

## 3. NATS Client (`internal/infra/nats/client.go`)

```go
package nats

import (
    "context"
    "fmt"
    "time"

    "github.com/nats-io/nats.go"
    "github.com/nats-io/nats.go/jetstream"
    "github.com/binhnt/globalcve/internal/config"
)

type Client struct {
    NC *nats.Conn
    JS jetstream.JetStream
}

func NewClient(ctx context.Context, cfg config.NATSConfig) (*Client, error) {
    nc, err := nats.Connect(cfg.URL,
        nats.RetryOnFailedConnect(true),
        nats.MaxReconnects(5),
        nats.ReconnectWait(2*time.Second),
    )
    if err != nil {
        return nil, fmt.Errorf("nats: connect failed: %w", err)
    }

    js, err := jetstream.New(nc)
    if err != nil {
        nc.Close()
        return nil, fmt.Errorf("nats: jetstream init: %w", err)
    }

    client := &Client{NC: nc, JS: js}

    // Provision streams
    if err := client.provisionStreams(ctx); err != nil {
        nc.Close()
        return nil, err
    }

    return client, nil
}

func (c *Client) provisionStreams(ctx context.Context) error {
    streams := []jetstream.StreamConfig{
        {
            Name:     "CVE_EVENTS",
            Subjects: []string{"cve.>"},
            MaxAge:   24 * time.Hour,
        },
        {
            Name:     "KEV_EVENTS",
            Subjects: []string{"kev.>"},
            MaxAge:   24 * time.Hour,
        },
        {
            Name:     "ALERT_EVENTS",
            Subjects: []string{"alert.>"},
            MaxAge:   48 * time.Hour,
        },
    }

    for _, s := range streams {
        _, err := c.JS.CreateOrUpdateStream(ctx, s)
        if err != nil {
            return fmt.Errorf("nats: provision stream %s: %w", s.Name, err)
        }
    }
    return nil
}

func (c *Client) Close() {
    c.NC.Drain()
}
```

> **Fail-open design:** Nếu NATS không available, trả về `nil` và warn (xem §5.3). Services kiểm tra `natsClient != nil` trước khi publish.

---

## 4. OpenSearch Client (`internal/infra/opensearch/client.go`)

```go
package opensearch

import (
    "fmt"

    opensearchgo "github.com/opensearch-project/opensearch-go/v2"
    "github.com/binhnt/globalcve/internal/config"
)

func NewClient(cfg config.OpenSearchConfig) (*opensearchgo.Client, error) {
    client, err := opensearchgo.NewClient(opensearchgo.Config{
        Addresses: cfg.Addresses,
        Username:  cfg.Username,
        Password:  cfg.Password,
    })
    if err != nil {
        return nil, fmt.Errorf("opensearch: create client: %w", err)
    }
    return client, nil
}
```

---

## 5. Event Type Definitions (`internal/events/events.go`)

```go
package events

// Subject constants
const (
    SubjectCVESynced      = "cve.synced"
    SubjectKEVUpdated     = "kev.updated"
    SubjectAlertTriggered = "alert.triggered"
)

// CVESyncedEvent published khi CVE Sync hoàn thành một source
type CVESyncedEvent struct {
    Source   string `json:"source"`
    Synced   int    `json:"synced"`
    SyncedAt string `json:"synced_at"` // RFC3339
}

// KEVUpdatedEvent published khi KEV Service sync xong
type KEVUpdatedEvent struct {
    Total     int      `json:"total"`
    Inserted  int      `json:"inserted"`
    NewKEVIDs []string `json:"new_kev_ids"`
}

// AlertTriggeredEvent published khi phát hiện CVE nghiêm trọng mới
type AlertTriggeredEvent struct {
    CVEID      string  `json:"cve_id"`
    Severity   string  `json:"severity"`
    CVSS3Score float64 `json:"cvss3_score"`
}
```

---

## 6. Wiring trong main.go (preview cho TASK-09)

```go
// Shared infra initialization order:
pool, err := infraPostgres.NewPool(ctx, cfg.Postgres)
// fail-fast if err

redisClient, err := infraRedis.NewClient(ctx, cfg.Redis)
// fail-fast if err

natsClient, err := infraNATS.NewClient(ctx, cfg.NATS)
if err != nil {
    log.Warn().Err(err).Msg("NATS unavailable, events disabled")
    natsClient = nil // fail-open
}

osClient, err := infraOS.NewClient(cfg.OpenSearch)
if err != nil {
    log.Warn().Err(err).Msg("OpenSearch unavailable")
    osClient = nil // fail-open
}
```

---

## Định Nghĩa Hoàn Thành

- [x] `go build ./internal/infra/...` thành công
- [x] Unit test: mock ping cho Postgres, Redis
- [x] NATS streams được tạo khi kết nối thành công
- [x] Event types compile không lỗi

---

*TASK-02 | Shared Infrastructure | GlobalCVE v3.0*
