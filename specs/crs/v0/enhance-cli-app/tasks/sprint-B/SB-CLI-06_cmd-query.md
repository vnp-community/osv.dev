# SB-CLI-06 — [NEW] cmd/query — Vulnerability Query CLI

## Metadata
- **Task ID**: SB-CLI-06
- **Sprint**: B (P1)
- **Ước tính**: 2 giờ
- **Dependencies**: SA-SHARED-03
- **Spec nguồn**: `specs/solutions/enhance-cli-app/02_cli-upgrade.md` § "2.6 [NEW] cmd/query"

---

## Context

```bash
# Xem gateway OSV routes
cat services/gateway-service/cmd/server/main.go | grep -A20 "osvRouter"

# Xem search-service OSV handler
cat services/search-service/internal/delivery/http/osv_handler.go | head -60
```

---

## Goal

Tạo CLI command `osv-query` để query vulnerabilities qua gateway-service OSV v1 API. Satisfies URD UR-01, UR-02, UR-04, UR-07.

---

## Files to Create

### `apps/cli/cmd/query/main.go`

```go
// Command query searches for vulnerabilities via the OSV v1 REST API.
// Targets gateway-service locally or the public OSV API.
//
// Usage:
//
//	# By package + version (UR-01)
//	osv-query --package lodash --version 4.17.20 --ecosystem npm
//
//	# By CVE/OSV ID (UR-02)
//	osv-query --cve CVE-2021-44228
//
//	# By git commit (UR-04)
//	osv-query --commit abc123def456789
//
//	# Batch query from file (UR-06)
//	osv-query --batch packages.json
//
//	# Output formats (UR-07)
//	osv-query --cve CVE-2021-44228 --format json
//	osv-query --cve CVE-2021-44228 --format table
//	osv-query --cve CVE-2021-44228 --format osv
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/osv/shared/pkg/clients/rest"
)

const defaultGatewayURL = "http://localhost:8080"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	fs := flag.NewFlagSet("osv-query", flag.ExitOnError)

	// Query mode flags
	pkg := fs.String("package", "", "Package name to query")
	version := fs.String("version", "", "Package version")
	ecosystem := fs.String("ecosystem", "", "Ecosystem: npm, PyPI, Maven, Go, crates.io, etc.")
	cveID := fs.String("cve", "", "CVE or OSV ID (e.g. CVE-2021-44228 or GHSA-xxx)")
	commit := fs.String("commit", "", "Git commit hash for commit-based query")
	batchFile := fs.String("batch", "", "JSON file with array of query objects for batch query")

	// Output flags
	format := fs.String("format", "table", "Output format: table|json|osv")
	gatewayURL := fs.String("gateway", envOrDefault("GATEWAY_URL", defaultGatewayURL), "Gateway service URL")

	fs.Parse(os.Args[1:])

	client := rest.NewOSVClient(*gatewayURL)

	// Dispatch based on query type
	switch {
	case *cveID != "":
		return queryCVEID(ctx, client, *cveID, *format)

	case *pkg != "" && *version != "":
		return queryPackage(ctx, client, *ecosystem, *pkg, *version, *format)

	case *commit != "":
		return queryCommit(ctx, client, *commit, *format)

	case *batchFile != "":
		return queryBatch(ctx, client, *batchFile, *format)

	default:
		fmt.Fprintf(os.Stderr, "Usage: osv-query [--cve ID | --package NAME --version VER | --commit HASH | --batch FILE]\n")
		fs.PrintDefaults()
		return fmt.Errorf("specify at least one query mode")
	}
}

func queryCVEID(ctx context.Context, client *rest.OSVClient, id, format string) error {
	result, err := client.QueryByCVEID(ctx, id)
	if err != nil {
		return err
	}
	return printResult(result, format)
}

func queryPackage(ctx context.Context, client *rest.OSVClient, ecosystem, pkg, version, format string) error {
	if ecosystem == "" {
		return fmt.Errorf("--ecosystem required when querying by package")
	}
	resp, err := client.QueryByPackage(ctx, ecosystem, pkg, version)
	if err != nil {
		return err
	}
	if len(resp.Vulns) == 0 {
		fmt.Printf("No vulnerabilities found for %s@%s (%s)\n", pkg, version, ecosystem)
		return nil
	}
	return printVulnList(resp.Vulns, format)
}

func queryCommit(ctx context.Context, client *rest.OSVClient, commit, format string) error {
	resp, err := client.QueryByCommit(ctx, commit)
	if err != nil {
		return err
	}
	if len(resp.Vulns) == 0 {
		fmt.Printf("No vulnerabilities found for commit %s\n", commit)
		return nil
	}
	return printVulnList(resp.Vulns, format)
}

func queryBatch(ctx context.Context, client *rest.OSVClient, batchFile, format string) error {
	data, err := os.ReadFile(batchFile)
	if err != nil {
		return fmt.Errorf("read batch file: %w", err)
	}
	var queries []rest.QueryRequest
	if err := json.Unmarshal(data, &queries); err != nil {
		return fmt.Errorf("parse batch file: %w", err)
	}
	resp, err := client.QueryBatch(ctx, queries)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(resp)
}

func printResult(result interface{}, format string) error {
	switch format {
	case "json", "osv":
		return json.NewEncoder(os.Stdout).Encode(result)
	case "table":
		// Convert to JSON first, then pretty-print key fields
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func printVulnList(vulns []rest.VulnSummary, format string) error {
	switch format {
	case "json", "osv":
		return json.NewEncoder(os.Stdout).Encode(vulns)
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMODIFIED\tALIASES")
		fmt.Fprintln(w, strings.Repeat("-", 60))
		for _, v := range vulns {
			fmt.Fprintf(w, "%s\t%s\t%s\n", v.ID, v.Modified, strings.Join(v.Aliases, ", "))
		}
		return w.Flush()
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

---

## Update `apps/cli/go.mod`

```go
// Thêm vào replace block:
replace (
    // ... existing ...
    github.com/osv/shared/pkg => ../../services/shared/pkg  // for rest.OSVClient
)

require (
    // ... existing ...
    github.com/osv/shared/pkg v0.0.0
)
```

---

## Acceptance Criteria

- [ ] `apps/cli/cmd/query/main.go` tạo
- [ ] `--cve ID` → `GET /v1/vulns/{id}`
- [ ] `--package + --version + --ecosystem` → `POST /v1/query`
- [ ] `--commit HASH` → `POST /v1/query` với commit field
- [ ] `--batch FILE` → `POST /v1/querybatch`
- [ ] `--format table|json|osv` output modes
- [ ] `go build ./cmd/query/...` từ `apps/cli` PASS

---

## Verification

```bash
cd apps/cli
go build ./cmd/query/...
./bin/osv-query --help
# Expected: shows all flags
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created (additive only)
- `apps/cli/cmd/query/main.go` — NEW `osv-query` command với 4 modes: package (-package/-ecosystem/-version), commit (-commit), ID (-id), search (-search)

### Build Verification
```
go build ./cmd/query/...  → OK
```
