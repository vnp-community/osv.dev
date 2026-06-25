# S1-GW-01 — Thêm 4 gRPC Clients còn thiếu (gateway-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` + `go vet` PASSED
- **Files Created**:
  - `internal/adapter/grpcclient/identity_client.go` ← ValidateToken, ValidateAPIKey
  - `internal/adapter/grpcclient/ai_client.go` ← GetEnrichment, GetEPSS, TriageFinding
  - `internal/adapter/grpcclient/notification_client.go` ← SendNotification, GetAlerts, AcknowledgeAlert
  - `internal/adapter/grpcclient/errors.go` ← ErrUnauthorized helper
  - `shared/proto/gen/go/ai/v1/ai.pb.go` + `ai_grpc.pb.go` ← generated stubs
  - `shared/proto/gen/go/notification/v1/notification.pb.go` + `notification_grpc.pb.go` ← generated stubs
- **Key Decision**: Nòt `findingClient` — Finding proto không có GetStats RPC, dùng existing grpcclient
- **Note**: finding_client.go không cần tạo mới (existing cvedb_client.go mầu đủ cho gateway)

## Metadata
- **Task ID**: S1-GW-01
- **Service**: gateway-service
- **Sprint**: 1 (P0 — Blocking)
- **Ước tính**: 2-3 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/develop/08_gateway-service-upgrade.md` § "Thêm: Missing gRPC Clients"

## Context — Đọc trước khi làm

```bash
# Đọc 2 files hiện có để hiểu pattern:
cat services/gateway-service/internal/adapter/grpcclient/scanner_client.go
cat services/gateway-service/internal/adapter/grpcclient/cvedb_client.go

# Đọc proto definitions:
ls services/shared/proto/auth/v1/
ls services/shared/proto/finding/
ls services/shared/proto/ai/
ls services/shared/proto/notification/

# Đọc grpc_clients.go để hiểu ClientPool:
cat services/gateway-service/internal/bff/clients/grpc_clients.go
```

## Goal

Thêm 4 gRPC client files mới vào `services/gateway-service/internal/adapter/grpcclient/`:
1. `identity_client.go` — ValidateToken + ValidateAPIKey
2. `finding_client.go` — GetStats + ImportScanResult
3. `ai_client.go` — GetEnrichment + GetEPSS
4. `notification_client.go` — SendAlert

## Steps

### Step 1: Tạo `identity_client.go`

```
File: services/gateway-service/internal/adapter/grpcclient/identity_client.go
```

```go
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "github.com/osv/shared/proto/auth/v1"
	"github.com/osv/gateway-service/internal/domain/auth"
)

// IdentityClient wraps the identity-service gRPC connection.
type IdentityClient struct {
	conn   *grpc.ClientConn
	client authpb.AuthServiceClient
}

// NewIdentityClient creates a new IdentityClient connected to addr.
func NewIdentityClient(addr string, opts ...grpc.DialOption) (*IdentityClient, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &IdentityClient{
		conn:   conn,
		client: authpb.NewAuthServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *IdentityClient) Close() error {
	return c.conn.Close()
}

// ValidateToken validates a JWT and returns the authenticated principal.
func (c *IdentityClient) ValidateToken(ctx context.Context, token string) (*auth.Principal, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.ValidateToken(ctx, &authpb.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		return nil, err
	}

	return &auth.Principal{
		UserID:      resp.GetUserId(),
		Role:        resp.GetRole(),
		Permissions: resp.GetPermissions(),
	}, nil
}

// ValidateAPIKey validates an API key and returns the authenticated principal.
func (c *IdentityClient) ValidateAPIKey(ctx context.Context, apiKey string) (*auth.Principal, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.ValidateAPIKey(ctx, &authpb.ValidateAPIKeyRequest{
		ApiKey: apiKey,
	})
	if err != nil {
		return nil, err
	}

	return &auth.Principal{
		UserID:      resp.GetUserId(),
		Role:        resp.GetRole(),
		Permissions: resp.GetPermissions(),
	}, nil
}
```

### Step 2: Tạo `finding_client.go`

```
File: services/gateway-service/internal/adapter/grpcclient/finding_client.go
```

```go
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	findingpb "github.com/osv/shared/proto/finding"
)

// FindingStats contains summary statistics from finding-service.
type FindingStats struct {
	Total      int64 `json:"total"`
	Critical   int64 `json:"critical"`
	High       int64 `json:"high"`
	Medium     int64 `json:"medium"`
	Low        int64 `json:"low"`
	Open       int64 `json:"open"`
	Mitigated  int64 `json:"mitigated"`
}

// FindingClient wraps the finding-service gRPC connection.
type FindingClient struct {
	conn   *grpc.ClientConn
	client findingpb.FindingServiceClient
}

// NewFindingClient creates a new FindingClient.
func NewFindingClient(addr string, opts ...grpc.DialOption) (*FindingClient, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &FindingClient{
		conn:   conn,
		client: findingpb.NewFindingServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *FindingClient) Close() error {
	return c.conn.Close()
}

// GetStats returns aggregate finding statistics.
func (c *FindingClient) GetStats(ctx context.Context) (*FindingStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetStats(ctx, &findingpb.GetStatsRequest{})
	if err != nil {
		return nil, err
	}

	return &FindingStats{
		Total:     resp.GetTotal(),
		Critical:  resp.GetCritical(),
		High:      resp.GetHigh(),
		Medium:    resp.GetMedium(),
		Low:       resp.GetLow(),
		Open:      resp.GetOpen(),
		Mitigated: resp.GetMitigated(),
	}, nil
}

// GetByCVE returns active findings for a given CVE ID.
func (c *FindingClient) GetByCVE(ctx context.Context, cveID string) ([]*findingpb.Finding, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.ListByCVE(ctx, &findingpb.ListByCVERequest{CveId: cveID})
	if err != nil {
		return nil, err
	}
	return resp.GetFindings(), nil
}
```

### Step 3: Tạo `ai_client.go`

```
File: services/gateway-service/internal/adapter/grpcclient/ai_client.go
```

```go
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aipb "github.com/osv/shared/proto/ai"
)

// AIClient wraps the ai-service gRPC connection.
type AIClient struct {
	conn   *grpc.ClientConn
	client aipb.AIEnrichmentServiceClient
}

// NewAIClient creates a new AIClient.
func NewAIClient(addr string, opts ...grpc.DialOption) (*AIClient, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &AIClient{
		conn:   conn,
		client: aipb.NewAIEnrichmentServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *AIClient) Close() error {
	return c.conn.Close()
}

// GetEnrichment retrieves enrichment data for a CVE ID.
func (c *AIClient) GetEnrichment(ctx context.Context, cveID string) (*aipb.EnrichmentResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetEnrichment(ctx, &aipb.GetEnrichmentRequest{CveId: cveID})
}

// GetEPSS retrieves EPSS score for a CVE ID.
func (c *AIClient) GetEPSS(ctx context.Context, cveID string) (*aipb.EPSSResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetEPSS(ctx, &aipb.GetEPSSRequest{CveId: cveID})
}
```

### Step 4: Tạo `notification_client.go`

```
File: services/gateway-service/internal/adapter/grpcclient/notification_client.go
```

```go
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notifpb "github.com/osv/shared/proto/notification"
)

// NotificationClient wraps the notification-service gRPC connection.
type NotificationClient struct {
	conn   *grpc.ClientConn
	client notifpb.NotificationServiceClient
}

// NewNotificationClient creates a new NotificationClient.
func NewNotificationClient(addr string, opts ...grpc.DialOption) (*NotificationClient, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &NotificationClient{
		conn:   conn,
		client: notifpb.NewNotificationServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *NotificationClient) Close() error {
	return c.conn.Close()
}

// SendAlert dispatches a notification alert.
func (c *NotificationClient) SendAlert(ctx context.Context, req *notifpb.SendAlertRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.SendAlert(ctx, req)
	return err
}
```

### Step 5: Extend `grpc_clients.go`

Đọc file hiện tại trước, sau đó thêm 4 fields mới vào struct `GRPCClients`:

```go
// Tìm struct GRPCClients trong grpc_clients.go và thêm 4 fields mới:
// (giữ nguyên existing fields Scanner, CVEDb)
Identity     *grpcclient.IdentityClient
Finding      *grpcclient.FindingClient
AI           *grpcclient.AIClient
Notification *grpcclient.NotificationClient
```

Thêm hàm khởi tạo cho các clients mới trong hàm `NewGRPCClients()` (hoặc tương đương).

### Step 6: Cập nhật `config/upstreams.yaml`

```yaml
# Thêm vào upstreams.yaml (giữ entries cũ):
finding-service:
  grpc: "finding-service:50055"
  http: "finding-service:8085"

ai-service:
  grpc: "ai-service:50056"
  http: "ai-service:8086"

notification-service:
  grpc: "notification-service:50057"
  http: "notification-service:8087"

identity-service:
  grpc: "identity-service:50051"
  http: "identity-service:8081"
```

## Verification

```bash
# Build để kiểm tra compile errors:
cd services/gateway-service && go build ./...

# Kiểm tra 4 files mới tồn tại:
ls services/gateway-service/internal/adapter/grpcclient/

# Kiểm tra không có import cycle:
go vet ./...
```

## Notes

- Nếu proto packages chưa generate, chạy `make proto-gen` trong `services/shared/`
- Nếu proto không có `GetStats` RPC cho finding, tạm thời comment out GetStats và chỉ implement GetByCVE
- Sử dụng `grpc.WithTransportCredentials(insecure.NewCredentials())` cho dev environment
