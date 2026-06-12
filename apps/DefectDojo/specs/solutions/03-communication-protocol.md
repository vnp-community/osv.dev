# Communication Protocols

## Tổng quan Protocol

| Protocol | Khi nào dùng | Ưu điểm | Nhược điểm |
|---|---|---|---|
| **gRPC (bufconn)** | In-process service calls | Zero latency, type-safe, streaming | Chỉ trong cùng process |
| **gRPC (TCP)** | External clients, cross-host | Standard, widespread | Network overhead |
| **NATS JetStream** | Async events, fan-out | Durability, pub/sub, replay | Eventually consistent |
| **REST/HTTP** | External API consumers | Ubiquitous, human readable | No streaming |

## 1. gRPC In-Process (bufconn)

### Thiết lập bufconn

```go
// apps/DefectDojo/internal/transport/bufconn.go

package transport

import (
    "context"
    "net"
    
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
)

const bufSize = 1 * 1024 * 1024 // 1MB

// BufConnPair holds a server listener and client dialer for in-process gRPC.
type BufConnPair struct {
    Listener *bufconn.Listener
}

func NewBufConnPair() *BufConnPair {
    return &BufConnPair{
        Listener: bufconn.Listen(bufSize),
    }
}

// Dial creates a gRPC client connection to the in-process server.
func (b *BufConnPair) Dial(ctx context.Context) (*grpc.ClientConn, error) {
    return grpc.DialContext(ctx, "passthrough://bufnet",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return b.Listener.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
}
```

### Kết nối từ report-service đến finding-service

```go
// Khởi tạo trong app.go
findingBuf := transport.NewBufConnPair()

// finding-service expose server trên buffer
findingRunner := runners.NewFindingServiceRunner(cfg, findingBuf)

// report-service nhận dialer để connect
reportRunner := runners.NewReportServiceRunner(cfg, findingBuf.Dial)

// Trong report-service:
conn, err := r.dialFinding(ctx)
findingClient := findingpb.NewFindingServiceClient(conn)

// Stream findings
stream, err := findingClient.ListFindingsForReport(ctx, &findingpb.ListFindingsForReportRequest{
    ProductId:   productID,
    ActiveOnly:  true,
})
for {
    finding, err := stream.Recv()
    if err == io.EOF {
        break
    }
    // process finding
}
```

## 2. NATS JetStream — Event Bus

### Stream & Subject Design

```
JetStream Stream: DD_EVENTS
  Subjects:
    dd.scan.submitted          → scan-service consume
    dd.scan.completed          → finding-service, notification-service, ai-service consume
    dd.finding.created         → notification-service, integration-service consume
    dd.finding.severity.critical → notification-service (urgent), ai-service consume
    dd.finding.closed          → notification-service consume
    dd.finding.sla.breach      → notification-service (alert), integration-service consume
    dd.report.requested        → report-service consume
    dd.report.completed        → notification-service consume
    dd.vuln.ingested           → search-service (index), impact-service consume
    dd.ai.triage.request       → ai-service consume
    dd.ai.triage.completed     → finding-service (update priority) consume
    dd.integration.jira.create → integration-service consume
    dd.integration.github.issue → integration-service consume
```

### NATS Publisher Pattern

```go
// apps/DefectDojo/internal/events/publisher.go

package events

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/nats-io/nats.go"
)

type Publisher struct {
    js nats.JetStreamContext
}

func NewPublisher(nc *nats.Conn) (*Publisher, error) {
    js, err := nc.JetStream()
    if err != nil {
        return nil, err
    }
    return &Publisher{js: js}, nil
}

// ScanCompleted publishes scan completion event.
func (p *Publisher) ScanCompleted(ctx context.Context, evt ScanCompletedEvent) error {
    data, err := json.Marshal(evt)
    if err != nil {
        return fmt.Errorf("marshal ScanCompleted: %w", err)
    }
    _, err = p.js.Publish("dd.scan.completed", data)
    return err
}

// FindingCreated publishes finding created event.
func (p *Publisher) FindingCreated(ctx context.Context, evt FindingCreatedEvent) error {
    data, _ := json.Marshal(evt)
    _, err := p.js.Publish("dd.finding.created", data)
    return err
}

// SLABreach publishes SLA breach event.
func (p *Publisher) SLABreach(ctx context.Context, evt SLABreachEvent) error {
    data, _ := json.Marshal(evt)
    _, err := p.js.Publish("dd.finding.sla.breach", data)
    return err
}
```

### NATS Consumer Pattern

```go
// apps/DefectDojo/internal/events/consumer.go

package events

// ScanCompletedConsumer — used by finding-service to process scan results
type ScanCompletedConsumer struct {
    js      nats.JetStreamContext
    handler func(ctx context.Context, evt ScanCompletedEvent) error
}

func (c *ScanCompletedConsumer) Start(ctx context.Context) error {
    sub, err := c.js.QueueSubscribe(
        "dd.scan.completed",
        "finding-service-workers",  // durable queue group
        func(msg *nats.Msg) {
            var evt ScanCompletedEvent
            if err := json.Unmarshal(msg.Data, &evt); err != nil {
                msg.Nak()
                return
            }
            if err := c.handler(ctx, evt); err != nil {
                msg.NakWithDelay(5 * time.Second)
                return
            }
            msg.Ack()
        },
        nats.Durable("finding-service-scan-completed"),
        nats.DeliverNew(),
        nats.AckExplicit(),
        nats.MaxDeliver(3),
    )
    if err != nil {
        return err
    }
    
    <-ctx.Done()
    sub.Drain()
    return nil
}
```

### NATS JetStream Setup

```go
// apps/DefectDojo/internal/events/setup.go

package events

import "github.com/nats-io/nats.go"

func SetupJetStream(js nats.JetStreamContext) error {
    _, err := js.AddStream(&nats.StreamConfig{
        Name:       "DD_EVENTS",
        Subjects:   []string{"dd.>"},
        Storage:    nats.FileStorage,
        Replicas:   1,
        MaxAge:     7 * 24 * time.Hour,  // 7 ngày retention
        MaxMsgs:    1_000_000,
        Discard:    nats.DiscardOld,
        Retention:  nats.LimitsPolicy,
    })
    if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
        return fmt.Errorf("create DD_EVENTS stream: %w", err)
    }
    return nil
}
```

## 3. gRPC Event Definitions

### Proto cho Product Service

```protobuf
// shared/proto/product/dd/v1/product.proto

syntax = "proto3";
package product.v1;
option go_package = "github.com/defectdojo/proto/product/v1;productv1";

import "google/protobuf/timestamp.proto";

service ProductService {
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse);
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
  rpc UpdateProduct(UpdateProductRequest) returns (UpdateProductResponse);
  rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse);
  
  rpc CreateEngagement(CreateEngagementRequest) returns (CreateEngagementResponse);
  rpc GetEngagement(GetEngagementRequest) returns (GetEngagementResponse);
  rpc ListEngagements(ListEngagementsRequest) returns (ListEngagementsResponse);
  rpc CloseEngagement(CloseEngagementRequest) returns (CloseEngagementResponse);
  
  rpc CreateTest(CreateTestRequest) returns (CreateTestResponse);
  rpc GetTest(GetTestRequest) returns (GetTestResponse);
}

message ProductProto {
  string id = 1;
  string name = 2;
  string description = 3;
  string product_type_id = 4;
  string business_criticality = 5;
  string lifecycle = 6;
  repeated string tags = 7;
  google.protobuf.Timestamp created_at = 8;
}
```

### Proto cho Notification Service

```protobuf
// shared/proto/notification/v1/notification.proto

syntax = "proto3";
package notification.v1;
option go_package = "github.com/defectdojo/proto/notification/v1;notificationv1";

service NotificationService {
  rpc CreateRule(CreateRuleRequest) returns (CreateRuleResponse);
  rpc ListRules(ListRulesRequest) returns (ListRulesResponse);
  rpc Subscribe(SubscribeRequest) returns (SubscribeResponse);
  rpc Unsubscribe(UnsubscribeRequest) returns (UnsubscribeResponse);
  rpc SendAlert(SendAlertRequest) returns (SendAlertResponse);
  rpc ListAlerts(ListAlertsRequest) returns (ListAlertsResponse);
}
```

## 4. REST API Routing

### Router Setup (unified-gateway)

```go
// apps/DefectDojo/internal/gateway/router.go

package gateway

import (
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
)

func NewRouter(svc *ServiceClients) *chi.Mux {
    r := chi.NewRouter()
    
    // Middleware stack
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins: []string{"*"},
        AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
    }))
    
    // Health & metrics
    r.Get("/health", healthHandler(svc.registry))
    r.Get("/metrics", promhttp.Handler())
    
    // DefectDojo v2 API — fully compatible
    r.Route("/api/v2", func(r chi.Router) {
        r.Use(authMiddleware(svc.authClient))
        
        // Products
        r.Route("/products", productRoutes(svc.productClient))
        r.Route("/product_types", productTypeRoutes(svc.productClient))
        
        // Engagements
        r.Route("/engagements", engagementRoutes(svc.productClient))
        
        // Tests
        r.Route("/tests", testRoutes(svc.scanClient))
        r.Route("/test_types", testTypeRoutes(svc.scanClient))
        
        // Findings
        r.Route("/findings", findingRoutes(svc.findingClient))
        r.Route("/finding_groups", findingGroupRoutes(svc.findingClient))
        
        // Import
        r.Post("/import-scan-results", importScanHandler(svc.scanClient))
        r.Post("/reimport-scan-results", reimportScanHandler(svc.scanClient))
        
        // Risk Acceptance
        r.Route("/risk_acceptances", riskAcceptanceRoutes(svc.findingClient))
        
        // Users & Auth
        r.Route("/users", userRoutes(svc.authClient))
        r.Post("/api-token-auth/", tokenAuthHandler(svc.authClient))
        r.Post("/api/v2/auth/login/", loginHandler(svc.authClient))
        
        // Reports
        r.Route("/reports", reportRoutes(svc.reportClient))
        
        // Notifications
        r.Route("/notifications", notificationRoutes(svc.notifClient))
        r.Route("/alerts", alertRoutes(svc.notifClient))
        
        // JIRA
        r.Route("/jira_projects", jiraRoutes(svc.integrationClient))
        r.Route("/jira_issues", jiraIssueRoutes(svc.integrationClient))
        
        // Search
        r.Get("/search/", searchHandler(svc.searchClient))
        
        // System settings
        r.Route("/system_settings", systemSettingsRoutes(svc))
        
        // Tool configurations
        r.Route("/tool_configurations", toolConfigRoutes(svc.integrationClient))
        r.Route("/tool_types", toolTypeRoutes(svc.integrationClient))
        
        // Endpoints
        r.Route("/endpoints", endpointRoutes(svc.findingClient))
        
        // SLA
        r.Route("/sla_configurations", slaConfigRoutes(svc.findingClient))
        
        // Tags
        r.Route("/tags", tagRoutes(svc))
        
        // Metrics (DefectDojo-style)
        r.Get("/metrics/", metricsHandler(svc.reportClient))
    })
    
    return r
}
```

## 5. Event Flow Diagrams

### Scan Import Flow

```
Client POST /api/v2/import-scan-results
    │
    ▼
unified-gateway
    │ JWT validate (auth-service via gRPC bufconn)
    │
    ▼
scan-service (gRPC: ImportScan)
    │ Validate test type, find/create test
    │ Publish: dd.scan.submitted
    │
    ▼
ingestion-service (NATS consumer: dd.scan.submitted)
    │ Parse scan file (SAST/DAST/SCA/Container)
    │ Normalize findings
    │
    ▼
finding-service (gRPC: BatchCreateFindings)
    │ Dedup check (hash-based)
    │ Create new findings
    │ Close old findings
    │ Calculate SLA dates
    │ Publish: dd.scan.completed
    │          dd.finding.created (per finding)
    │
    ├─── notification-service (NATS: dd.finding.created)
    │       Check alert rules
    │       Send email/slack/webhook
    │
    ├─── ai-service (NATS: dd.finding.severity.critical)
    │       AI triage & priority
    │       Publish: dd.ai.triage.completed
    │
    ├─── impact-service (NATS: dd.finding.created)
    │       Match CVE → business impact
    │       Update finding priority
    │
    └─── integration-service (NATS: dd.finding.severity.critical)
            Create JIRA ticket
            Create GitHub issue
```

### SLA Check Flow (Scheduled)

```
finding-service.SLAChecker (ticker: 1h)
    │ Query: findings with SLA near expiry
    │
    ├── SLA Warning (7 days before)
    │       Publish: dd.finding.sla.breach { type: "warning" }
    │           notification-service → email product owner
    │
    └── SLA Breach
            Publish: dd.finding.sla.breach { type: "breached" }
                notification-service → urgent alert
                integration-service → update JIRA priority: Critical
```

### Report Generation Flow

```
Client POST /api/v2/reports/
    │
    ▼
unified-gateway
    │
    ▼
report-service (gRPC: GenerateReport)
    │
    ├── product-service.GetProduct (gRPC bufconn)
    ├── finding-service.ListFindingsForReport (gRPC stream bufconn)
    │       Stream 10k+ findings without OOM
    │
    ├── Format: HTML/PDF/CSV/JSON
    │
    └── Publish: dd.report.completed
            notification-service → send email with attachment
```
