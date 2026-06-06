# Service 11 — Alias & Relations Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P1  
> **Language:** Go  
> **Pattern:** CQRS + Clean Architecture  
> **Communication:** gRPC (sync) + NATS (async events)

---

## 1. Trách Nhiệm

Service quản lý **alias groups và upstream/downstream relationships** giữa các vulnerabilities. Thay thế Alias Worker Python và relations service Go cũ.

**Responsibilities:**
- Group vulnerability IDs từ different sources referring to same vuln
  - Example: CVE-2021-12345 ↔ GHSA-xxxx-xxxx-xxxx ↔ PYSEC-2021-xxx
- Manage `AliasGroup` entities (M:M relationships)
- Track `UpstreamGroup` (upstream/downstream vuln inheritance)
- Track `RelatedGroup` (related but distinct vulnerabilities)
- Detect new aliases via AI similarity (using AI Enrichment embeddings)
- Notify Ingestion Service when alias groups change (cache invalidation)
- Expose alias resolution API (resolve any ID → canonical group)

**NOT Responsible for:**
- Storing core vulnerability data (Ingestion Service)
- Querying vulnerabilities (Query Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── AliasGroup aggregate (set of IDs → same vulnerability)
  ├── UpstreamGroup aggregate (upstream → downstream relationships)
  ├── RelatedGroup aggregate (related but distinct)
  ├── VulnID value object
  └── Repository: AliasGroupRepository, UpstreamGroupRepository

Application:
  ├── MergeAliasGroupCommand + Handler
  ├── AddAliasCommand + Handler
  ├── UpdateUpstreamCommand + Handler
  ├── ResolveAliasQuery + Handler           (ID → group)
  ├── GetAliasGroupQuery + Handler
  └── DetectNewAliasesCommand + Handler     (AI-powered)

Infrastructure:
  ├── FirestoreAliasGroupRepo
  ├── NATSConsumer + Publisher
  ├── AIEnrichmentGrpcClient (for embedding similarity)
  └── QueryServiceGrpcClient (resolve IDs)

Interface:
  ├── gRPC handler (AliasRelationsService)
  └── NATS consumer (VulnImported events)
```

---

## 3. Directory Structure

```
services/alias-relations/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── aggregate/
│   │   │   ├── alias_group/
│   │   │   │   ├── alias_group.go           # Aggregate root
│   │   │   │   └── alias_group_test.go
│   │   │   ├── upstream_group/
│   │   │   │   └── upstream_group.go
│   │   │   └── related_group/
│   │   │       └── related_group.go
│   │   ├── valueobject/
│   │   │   ├── vuln_id.go
│   │   │   └── relationship_type.go         # ALIAS | UPSTREAM | RELATED
│   │   ├── service/
│   │   │   ├── alias_merger.go              # Merge overlapping groups
│   │   │   └── similarity_detector.go       # Detect new aliases via embeddings
│   │   └── repository/
│   │       ├── alias_group_repo.go
│   │       └── upstream_group_repo.go
│   ├── application/
│   │   ├── command/
│   │   │   ├── merge_alias_group/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   ├── add_alias/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── detect_new_aliases/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   └── query/
│   │       ├── resolve_alias/
│   │       │   ├── query.go
│   │       │   └── handler.go
│   │       └── get_alias_group/
│   │           ├── query.go
│   │           └── handler.go
│   └── infra/
│       ├── persistence/
│       │   └── firestore/
│       │       ├── alias_group_repo.go
│       │       └── upstream_group_repo.go
│       ├── messaging/
│       │   └── nats/
│       │       ├── consumer.go
│       │       └── publisher.go
│       └── client/
│           ├── ai_enrichment_client.go      # Get embeddings for similarity
│           └── query_service_client.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── alias_relations_handler.go
│   │   └── proto/
│   │       └── alias_relations_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/config.go
├── Dockerfile
└── go.mod
```

---

## 4. Proto Definition

```protobuf
// proto/alias_relations_service.proto
syntax = "proto3";
package osv.alias.v1;

service AliasRelationsService {
  // Resolve any vuln ID to its alias group
  rpc ResolveAlias(ResolveAliasRequest) returns (ResolveAliasResponse);
  
  // Get all members of an alias group
  rpc GetAliasGroup(GetAliasGroupRequest) returns (AliasGroup);
  
  // Admin: manually declare alias relationship
  rpc AddAlias(AddAliasRequest) returns (AddAliasResponse);
  
  // Get upstream/downstream relationships
  rpc GetUpstreamRelations(GetUpstreamRequest) returns (UpstreamRelations);
}

message ResolveAliasRequest {
  string vuln_id = 1;  // Any ID (CVE, GHSA, PYSEC...)
}

message ResolveAliasResponse {
  string canonical_id         = 1;   // Primary ID for this group
  repeated string all_ids     = 2;   // All aliases including canonical
  string group_id             = 3;
  string last_modified        = 4;
}

message AliasGroup {
  string group_id             = 1;
  repeated string bug_ids     = 2;   // All IDs in group
  string last_modified        = 3;
  string detection_method     = 4;   // MANUAL | SOURCE_DECLARED | AI_DETECTED
}

message UpstreamRelations {
  string vuln_id              = 1;
  repeated string upstream    = 2;   // This vuln is downstream of...
  repeated string downstream  = 3;   // This vuln is upstream of...
}
```

---

## 5. Domain — AliasGroup Aggregate

```go
// domain/aggregate/alias_group/alias_group.go
package alias_group

// AliasGroup represents a set of vulnerability IDs that refer to the same vulnerability.
// Example: {CVE-2021-12345, GHSA-xxxx-xxxx-xxxx, PYSEC-2021-100}
type AliasGroup struct {
    id           string
    bugIDs       map[string]struct{}   // Set of vulnerability IDs
    lastModified time.Time
    detectionMethod DetectionMethod    // MANUAL | SOURCE_DECLARED | AI_DETECTED
    
    events []domain.Event
}

// Merge combines two alias groups into one.
// Business rule: if groups overlap → merge into single group.
func (g *AliasGroup) Merge(other *AliasGroup) {
    for id := range other.bugIDs {
        g.bugIDs[id] = struct{}{}
    }
    g.lastModified = time.Now().UTC()
    g.events = append(g.events, event.NewAliasGroupUpdated(g.id, g.BugIDs()))
}

// AddID adds a new vulnerability ID to the group.
func (g *AliasGroup) AddID(id string) {
    if _, exists := g.bugIDs[id]; exists {
        return // Idempotent
    }
    g.bugIDs[id] = struct{}{}
    g.lastModified = time.Now().UTC()
    g.events = append(g.events, event.NewAliasGroupUpdated(g.id, g.BugIDs()))
}

// CanonicalID returns the "primary" ID for this group.
// Priority: CVE > GHSA > OSV > others (alphabetical)
func (g *AliasGroup) CanonicalID() string {
    for _, prefix := range []string{"CVE-", "GHSA-", "OSV-"} {
        for id := range g.bugIDs {
            if strings.HasPrefix(id, prefix) {
                return id
            }
        }
    }
    ids := g.BugIDs()
    sort.Strings(ids)
    return ids[0]
}
```

---

## 6. Alias Detection Algorithm

```go
// domain/service/alias_merger.go

// AliasesDeclaredInOSV detects alias relationships from the vulnerability's
// own `aliases[]` field in OSV schema.
// Example: GHSA-xxx declares {"aliases": ["CVE-2021-12345"]}

type AliasMerger struct {
    groupRepo    repository.AliasGroupRepository
    tracer       trace.Tracer
    logger       *zerolog.Logger
}

func (m *AliasMerger) ProcessVulnerability(
    ctx context.Context,
    vulnID string,
    declaredAliases []string,
) error {
    // 1. Find existing groups containing any of these IDs
    var affectedGroups []*AliasGroup
    
    allIDs := append([]string{vulnID}, declaredAliases...)
    for _, id := range allIDs {
        group, err := m.groupRepo.GetByMemberID(ctx, id)
        if err != nil && !errors.Is(err, domain.ErrNotFound) {
            return err
        }
        if group != nil {
            affectedGroups = append(affectedGroups, group)
        }
    }
    
    // 2. Merge all affected groups into one
    var merged *AliasGroup
    if len(affectedGroups) == 0 {
        // New group
        merged = NewAliasGroup(allIDs, DetectionMethodSourceDeclared)
    } else {
        merged = affectedGroups[0]
        for _, g := range affectedGroups[1:] {
            merged.Merge(g)
        }
        for _, id := range allIDs {
            merged.AddID(id)
        }
    }
    
    // 3. Save (may delete old groups and create new merged one)
    return m.groupRepo.Save(ctx, merged)
}
```

---

## 7. AI-Powered Alias Detection

```go
// domain/service/similarity_detector.go

// Detect aliases by embedding similarity (AI feature).
// Two vulnerabilities with very high embedding similarity
// may refer to the same underlying issue.

type SimilarityDetector struct {
    aiClient port.AIEnrichmentClient
    threshold float32  // Default: 0.95 cosine similarity
}

func (d *SimilarityDetector) FindPotentialAliases(
    ctx context.Context,
    vulnID string,
    embedding []float32,
) ([]string, error) {
    
    // Query vector search for similar vulnerabilities
    similar, err := d.aiClient.FindSimilar(ctx, &port.SimilaritySearchRequest{
        Embedding:  embedding,
        TopK:       20,
        MinScore:   d.threshold,  // Only very high similarity
        ExcludeID:  vulnID,
    })
    if err != nil {
        return nil, err
    }
    
    // Filter out already-known aliases
    var potentialAliases []string
    for _, match := range similar {
        if match.Score >= d.threshold {
            potentialAliases = append(potentialAliases, match.VulnID)
        }
    }
    
    return potentialAliases, nil
}
```

---

## 8. Events

```go
// Outbound: AliasGroupUpdated
// Topic: osv.alias.group.updated
// Consumers: Query Service (cache invalidation), Notification Service

type AliasGroupUpdated struct {
    EventID      string    `json:"event_id"`
    OccurredAt   time.Time `json:"occurred_at"`
    GroupID      string    `json:"group_id"`
    BugIDs       []string  `json:"bug_ids"`       // All IDs in group
    CanonicalID  string    `json:"canonical_id"`   // Primary ID
    LastModified time.Time `json:"last_modified"`
}

// Inbound events consumed:
// - osv.vuln.imported → check new vulnerability's aliases[] field
// - osv.ai.enrichment.completed → check embedding similarity for new aliases
```

---

## 9. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| Alias resolution P50 | < 10ms (cached) |
| Alias resolution P99 | < 100ms |
| Alias grouping accuracy | > 99.5% |
| Event processing lag | < 30s after VulnImported |
| AI alias detection recall | > 85% (vs manual labeling) |

---

## 10. Implementation Status

> **Status:** ✅ Core Implemented | **Updated:** 2026-06-01

### Implemented
- [x] `domain/aggregate/alias_group/alias_group.go` — AliasGroup aggregate (AddID idempotent, Merge, CanonicalID priority: CVE>GHSA>OSV, domain events)
- [x] `domain/aggregate/alias_group/alias_group_test.go` — Unit tests (8 tests)
- [x] `domain/aggregate/upstream_group/upstream_group.go` — UpstreamGroup aggregate (directional relationships)
- [x] `domain/service/alias_merger.go` — AliasMerger (ProcessVulnerability: find groups → merge → save)
- [x] `domain/service/similarity_detector.go` — SimilarityDetector (embedding cosine similarity, threshold=0.95)
- [x] `domain/valueobject/vuln_id.go` + `relationship_type.go` — VulnID, RelationshipType (ALIAS/UPSTREAM/RELATED)
- [x] `domain/repository/alias_group_repo.go` — AliasGroupRepository + UpstreamGroupRepository interfaces
- [x] `application/command/merge_alias_group/handler.go` — MergeAliasGroup command
- [x] `application/command/detect_new_aliases/handler.go` — DetectNewAliases command (AI-powered)
- [x] `application/query/resolve_alias/handler.go` — ResolveAlias query (any ID → canonical group)
- [x] `infra/persistence/firestore/alias_group_repo.go` — Firestore batch save + member index
- [x] `infra/messaging/nats/consumer.go` — 2 consumers: VulnImported + AIEnrichmentCompleted
- [x] `infra/messaging/nats/publisher.go` — AliasGroupUpdated publisher
- [x] `interface/grpc/proto/alias_relations_service.proto` — 4 RPCs: ResolveAlias, GetAliasGroup, AddAlias, GetUpstreamRelations
- [x] `interface/http/handler/health_handler.go`, `config/config.go`, `cmd/server/main.go`
- [x] `Dockerfile`, `Makefile`, `go.mod`

### Pending
- [ ] Integration tests (NATS JetStream + Firestore emulators)
- [ ] `infra/client/ai_enrichment_client.go` — gRPC client to AI Enrichment Service (FindSimilar)
- [ ] `interface/grpc/handler/alias_relations_handler.go` — gRPC handler (from proto-gen)
- [ ] `infra/persistence/firestore/upstream_group_repo.go` — UpstreamGroupRepo Firestore impl

### Deviations from Spec
- UpstreamGroup implemented as domain aggregate; RelatedGroup deferred (not yet needed)
- AIEnrichmentClient is interface only; real gRPC client pending VertexAI/proto-gen integration
