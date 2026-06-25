# OSV Platform Integration Tests

Run the full platform stack before executing tests:

```bash
cd deploy/dev
docker compose up -d
# Wait for services to be healthy
sleep 30
# Run all integration tests
cd ../../tests/integration
go test ./... -v -timeout 10m
```

## Test files

| File | Coverage |
|------|---------|
| `cli_importer_test.go` | Import → NATS publish → data-service ingest |
| `osv_query_test.go` | `GET /v1/vulns/{id}`, `POST /v1/query`, `POST /v1/querybatch` |
| `ai_enrichment_test.go` | Worker enrichment → AI service → result in data-service |
| `full_pipeline_test.go` | End-to-end: import → enrich → search → query |
| `sitemap_test.go` | `GET /sitemap.xml` XML format validation |
| `schema_validation_test.go` | `POST /admin/validate` accept/reject OSV records |

## Environment variables

```bash
export GATEWAY_URL=http://localhost:8080
export DATA_SERVICE_URL=http://localhost:8082
export SEARCH_SERVICE_URL=http://localhost:8083
export NATS_URL=nats://localhost:4222
```

## Running individual test suites

```bash
# Only gateway API tests
go test ./... -run TestOSVQuery -v

# Only CLI pipeline tests  
go test ./... -run TestCLIImporter -v

# Full pipeline (slowest)
go test ./... -run TestFullPipeline -v -timeout 15m
```
