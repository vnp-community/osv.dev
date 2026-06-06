# Task T09 — Alias & Relations Service

> **Priority:** P1 | **Phase:** 3 | **Spec:** `specs/services/11-alias-relations-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, Firestore)  
> **Consumed by:** T02-vulnerability-query (alias resolution)

## Mục Tiêu
Quản lý alias groups (multiple IDs → same vuln) và upstream/downstream relationships. Thay thế Alias Worker Python.

## Trách Nhiệm
- Group vulnerability IDs từ different sources: `CVE-2021-12345 ↔ GHSA-xxx ↔ PYSEC-2021-xxx`
- Manage `AliasGroup` aggregates
- Track `UpstreamGroup`, `RelatedGroup`
- Detect new aliases via embedding similarity (AI feature)
- Expose alias resolution API: any ID → canonical group
- Notify cache invalidation khi alias groups change

## Cấu Trúc File

```
services/alias-relations/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   ├── alias_group/{alias_group,alias_group_test}.go
│   │   │   ├── upstream_group/upstream_group.go
│   │   │   └── related_group/related_group.go
│   │   ├── valueobject/
│   │   │   ├── vuln_id.go
│   │   │   └── relationship_type.go  # ALIAS | UPSTREAM | RELATED
│   │   ├── service/
│   │   │   ├── alias_merger.go           # Merge overlapping groups
│   │   │   └── similarity_detector.go   # Detect aliases via embeddings
│   │   └── repository/
│   │       ├── alias_group_repo.go
│   │       └── upstream_group_repo.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── merge_alias_group/{command,handler}.go
│   │   │   ├── add_alias/{command,handler}.go
│   │   │   └── detect_new_aliases/{command,handler}.go
│   │   └── query/
│   │       ├── resolve_alias/{query,handler}.go
│   │       └── get_alias_group/{query,handler}.go
│   └── infra/
│       ├── persistence/firestore/
│       │   ├── alias_group_repo.go
│       │   └── upstream_group_repo.go
│       ├── messaging/nats/
│       │   ├── consumer.go   # VulnImported, AIEnrichmentCompleted
│       │   └── publisher.go  # AliasGroupUpdated
│       └── client/
│           ├── ai_enrichment_client.go  # Get embeddings for similarity
│           └── query_service_client.go
├── interface/
│   ├── grpc/
│   │   ├── handler/alias_relations_handler.go
│   │   └── proto/alias_relations_service.proto
│   └── http/handler/health_handler.go
└── config/config.go
```

## AliasGroup Aggregate (Core Domain)

```go
// domain/aggregate/alias_group/alias_group.go
type AliasGroup struct {
    id              string
    bugIDs          map[string]struct{}  // Set of vuln IDs
    lastModified    time.Time
    detectionMethod DetectionMethod  // MANUAL | SOURCE_DECLARED | AI_DETECTED
    events          []domain.Event
}

func NewAliasGroup(ids []string, method DetectionMethod) *AliasGroup
func (g *AliasGroup) AddID(id string)  // Idempotent: no-op nếu đã tồn tại
func (g *AliasGroup) Merge(other *AliasGroup)  // Union of bug IDs
// Appends AliasGroupUpdated event sau mỗi thay đổi

func (g *AliasGroup) CanonicalID() string:
  // Priority: CVE- > GHSA- > OSV- > alphabetical first
  for _, prefix := range []string{"CVE-", "GHSA-", "OSV-"} {
    for id := range g.bugIDs { if HasPrefix(id, prefix) { return id } }
  }
  return sortedIDs[0]

func (g *AliasGroup) BugIDs() []string  // sorted
```

## Alias Detection via OSV `aliases[]` Field

```go
// domain/service/alias_merger.go
// Primary method: read declared aliases[] from OSV schema
func (m *AliasMerger) ProcessVulnerability(ctx, vulnID string, declaredAliases []string) error:
  allIDs := append([]string{vulnID}, declaredAliases...)
  
  // 1. Find all existing groups containing any of these IDs
  affectedGroups := []*AliasGroup{}
  for _, id := range allIDs {
    group, _ := groupRepo.GetByMemberID(ctx, id)
    if group != nil { affectedGroups = append(affectedGroups, group) }
  }
  
  // 2. Merge all into one group
  if len(affectedGroups) == 0 {
    merged = NewAliasGroup(allIDs, SourceDeclared)
  } else {
    merged = affectedGroups[0]
    for _, g := range affectedGroups[1:] { merged.Merge(g) }
    for _, id := range allIDs { merged.AddID(id) }
  }
  
  // 3. Save (delete old groups if merged)
  return groupRepo.Save(ctx, merged)
```

## AI-Powered Alias Detection (Optional Feature)

```go
// domain/service/similarity_detector.go
// Secondary method: embedding cosine similarity >= 0.95
type SimilarityDetector struct {
    aiClient  port.AIEnrichmentClient
    threshold float32  // Default: 0.95
}

func (d *SimilarityDetector) FindPotentialAliases(ctx, vulnID string, embedding []float32) ([]string, error):
  similar, _ := aiClient.FindSimilar(ctx, &SimilaritySearchRequest{
    Embedding: embedding, TopK: 20, MinScore: d.threshold, ExcludeID: vulnID,
  })
  // Return IDs with score >= threshold
```

## Proto

```protobuf
service AliasRelationsService {
  rpc ResolveAlias(ResolveAliasRequest) returns (ResolveAliasResponse);
  rpc GetAliasGroup(GetAliasGroupRequest) returns (AliasGroup);
  rpc AddAlias(AddAliasRequest) returns (AddAliasResponse);
  rpc GetUpstreamRelations(GetUpstreamRequest) returns (UpstreamRelations);
}
message ResolveAliasRequest { string vuln_id = 1; }
message ResolveAliasResponse {
  string canonical_id = 1; repeated string all_ids = 2;
  string group_id = 3; string last_modified = 4;
}
message AliasGroup {
  string group_id = 1; repeated string bug_ids = 2;
  string last_modified = 3;
  string detection_method = 4;  // MANUAL | SOURCE_DECLARED | AI_DETECTED
}
message UpstreamRelations {
  string vuln_id = 1;
  repeated string upstream = 2;    // this vuln is downstream of...
  repeated string downstream = 3;  // this vuln is upstream of...
}
```

## Events

```go
// Inbound (from NATS):
// "osv.vuln.imported" → extract aliases[] → AliasMerger.ProcessVulnerability
// "osv.ai.enrichment.completed" → if has embedding → SimilarityDetector.FindPotentialAliases

// Outbound:
// Topic: "osv.alias.group.updated"
type AliasGroupUpdated struct {
    EventID      string    `json:"event_id"`
    GroupID      string    `json:"group_id"`
    BugIDs       []string  `json:"bug_ids"`
    CanonicalID  string    `json:"canonical_id"`
    LastModified time.Time `json:"last_modified"`
    OccurredAt   time.Time `json:"occurred_at"`
}
// Consumers: Query Service (cache invalidation), Notification Service
```

## Firestore Schema

```
alias-groups/{group_id}:
  bug_ids: []string         # All IDs in group
  canonical_id: string      # Primary ID
  last_modified: timestamp
  detection_method: string

# Index: bug_id → group_id (denormalized for fast lookup)
alias-group-members/{bug_id}:
  group_id: string

upstream-groups/{vuln_id}:
  upstream: []string    # IDs this vuln is downstream of
  downstream: []string  # IDs this vuln is upstream of
```

## SLO Targets
- Alias resolution P50: <10ms (cached), P99: <100ms
- Alias grouping accuracy: >99.5%
- Event processing lag: <30s after VulnImported
- AI alias detection recall: >85%

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `AliasGroup` aggregate (AddID, Merge, CanonicalID, idempotent operations)
- [x] Implement `UpstreamGroup` aggregate
- [x] Implement `AliasMerger.ProcessVulnerability` (source-declared aliases)
- [x] Implement `SimilarityDetector.FindPotentialAliases` (embedding similarity)
- [x] Implement Firestore `AliasGroupRepo`: GetByMemberID, Save, Delete old groups
- [x] Implement `ResolveAliasHandler` (lookup by member ID → canonical group)
- [x] Implement `GetAliasGroupHandler`
- [x] Implement `AddAliasHandler` (manual merge via MergeAliasGroup handler)
- [x] Implement `DetectNewAliasesHandler` (AI feature, triggered by AIEnrichmentCompleted)
- [x] Implement NATS consumer (VulnImported → extract aliases → merge)
- [x] Implement NATS consumer (AIEnrichmentCompleted → SimilarityDetector)
- [x] Implement NATS publisher (AliasGroupUpdated)
- [x] Implement `AIEnrichmentClient` port interface (gRPC: FindSimilar)
- [x] gRPC proto (`alias_relations_service.proto`)
- [x] Health HTTP handler (`/health/live`, `/health/ready`)
- [x] `config/config.go` (env-var based loader)
- [x] `cmd/server/main.go` (wire all deps, graceful shutdown)
- [x] `go.mod` + `Dockerfile` + `Makefile`
- [x] Unit tests: CanonicalID priority, Merge logic, AliasMerger (`alias_group_test.go`)
- [ ] Integration tests: NATS + Firestore emulator
- [ ] Implement real gRPC AIEnrichmentClient (FindSimilar call to ai-enrichment service)
- [ ] gRPC handler code-gen from proto (`alias_relations_handler.go`)

