# TASK-DD-009 — Security Parser Factory (20+ Parsers)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-009 |
| **Service** | `scan-service` |
| **CR** | CR-DD-002 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | — (độc lập) |
| **Estimated effort** | 3 ngày |

## Context

Implement `ParserFactory` và ít nhất **20 security parsers** ưu tiên cao nhất. Mỗi parser đọc file output của scanner tương ứng và trả về `[]ParsedFinding`. scan-service hiện có golang/java/nodejs/python/rust parsers cho SCA — task này thêm security scanners.

## Reference

- Solution: [`sol-scan-service.md § Parser Factory`](../solutions/sol-scan-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/scan-service/
```

## Files to Create

```
internal/infra/parser/
├── factory.go              # ParserFactory implementation + registration
├── registry.go             # Global registry, auto-register on init()
│
# Priority 1: Container/SCA (most common in DevSecOps)
├── trivy/
│   ├── parser.go           # Trivy JSON output (containers + SCA)
│   └── parser_test.go
│   └── testdata/
│       ├── trivy_container.json
│       └── trivy_sca.json
│
├── grype/
│   ├── parser.go           # Grype JSON output
│   └── testdata/grype.json
│
├── snyk/
│   ├── parser.go           # Snyk JSON output
│   └── testdata/snyk.json
│
├── cyclonedx/
│   ├── parser.go           # CycloneDX SBOM (XML + JSON)
│   └── testdata/
│
# Priority 2: SAST
├── bandit/
│   ├── parser.go           # Bandit JSON (Python SAST)
│   └── testdata/bandit.json
│
├── semgrep/
│   ├── parser.go           # Semgrep JSON Report
│   └── testdata/semgrep.json
│
├── gosec/
│   ├── parser.go           # Gosec JSON output
│   └── testdata/gosec.json
│
├── sarif/
│   ├── parser.go           # Generic SARIF 2.1.0 (universal)
│   └── testdata/
│       ├── sarif_github.json
│       └── sarif_checkmarx.json
│
├── sonarqube/
│   ├── parser.go           # SonarQube JSON export
│   └── testdata/sonarqube.json
│
# Priority 3: DAST
├── owasp_zap/
│   ├── parser.go           # ZAP XML/JSON report
│   └── testdata/
│       └── zap.json
│
├── nuclei/
│   ├── parser.go           # Nuclei JSON output
│   └── testdata/nuclei.json
│
# Priority 4: Infrastructure
├── checkov/
│   ├── parser.go           # Checkov JSON (IaC)
│   └── testdata/checkov.json
│
├── tfsec/
│   ├── parser.go           # tfsec JSON (Terraform)
│   └── testdata/tfsec.json
│
# Priority 5: Network/Vuln scanners
├── nessus/
│   ├── parser.go           # Nessus XML (.nessus) or CSV
│   └── testdata/nessus.xml
│
├── dependency_check/
│   ├── parser.go           # OWASP Dependency-Check JSON/XML
│   └── testdata/dc.json
│
├── retire_js/
│   ├── parser.go           # RetireJS JSON
│   └── testdata/retire.json
│
├── npm_audit/
│   ├── parser.go           # npm audit JSON
│   └── testdata/npm_audit.json
│
├── aws_security_hub/
│   ├── parser.go           # AWS Security Hub JSON export
│   └── testdata/ash.json
│
├── qualys/
│   ├── parser.go           # Qualys XML report
│   └── testdata/qualys.xml
│
└── burp/
    ├── parser.go           # Burp Suite XML
    └── testdata/burp.xml
```

## Implementation Spec

### `internal/infra/parser/factory.go`

```go
package parser

import (
    "fmt"
    "github.com/osv/services/scan-service/internal/domain/parser"
)

// ParserFactory creates parsers by scan type name
type ParserFactory struct {
    parsers map[string]parser.Parser
}

var globalFactory = &ParserFactory{
    parsers: make(map[string]parser.Parser),
}

// Register adds a parser to the factory (called from each parser's init())
func Register(p parser.Parser) {
    globalFactory.parsers[p.ScanType()] = p
}

// GetParser returns parser for given scan type, error if not found
func (f *ParserFactory) GetParser(scanType string) (parser.Parser, error) {
    p, ok := f.parsers[scanType]
    if !ok {
        return nil, fmt.Errorf("no parser registered for scan type %q", scanType)
    }
    return p, nil
}

// ListScanTypes returns all registered scan type names
func (f *ParserFactory) ListScanTypes() []string {
    names := make([]string, 0, len(f.parsers))
    for name := range f.parsers {
        names = append(names, name)
    }
    return names
}

// NewParserFactory creates factory and registers all parsers
func NewParserFactory() *ParserFactory {
    // Parsers are auto-registered via init() in each package
    // Just ensure all parser packages are imported
    return globalFactory
}
```

### `internal/infra/parser/trivy/parser.go` (example implementation)

```go
package trivy

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "strings"

    "github.com/osv/services/scan-service/internal/domain/parser"
    infraparsers "github.com/osv/services/scan-service/internal/infra/parser"
)

func init() {
    infraparsers.Register(&TrivyParser{})
}

type TrivyParser struct{}

func (p *TrivyParser) ScanType() string { return "Trivy Scan" }

// TrivyOutput — JSON structure of trivy output
type TrivyOutput struct {
    SchemaVersion int           `json:"SchemaVersion"`
    ArtifactName  string        `json:"ArtifactName"`
    ArtifactType  string        `json:"ArtifactType"`
    Results       []TrivyResult `json:"Results"`
}

type TrivyResult struct {
    Target          string            `json:"Target"`
    Class           string            `json:"Class"`
    Type            string            `json:"Type"`
    Vulnerabilities []TrivyVuln       `json:"Vulnerabilities"`
}

type TrivyVuln struct {
    VulnerabilityID  string  `json:"VulnerabilityID"`  // CVE-XXXX-XXXX
    PkgName          string  `json:"PkgName"`
    InstalledVersion string  `json:"InstalledVersion"`
    FixedVersion     string  `json:"FixedVersion"`
    Title            string  `json:"Title"`
    Description      string  `json:"Description"`
    Severity         string  `json:"Severity"`
    CVSS             map[string]TrivyCVSS `json:"CVSS"`
    References       []string `json:"References"`
    CweIDs           []string `json:"CweIDs"`
}

type TrivyCVSS struct {
    V3Vector string  `json:"V3Vector"`
    V3Score  float64 `json:"V3Score"`
    V4Vector string  `json:"V4Vector"`
    V4Score  float64 `json:"V4Score"`
}

func (p *TrivyParser) GetFindings(ctx context.Context, file io.Reader, test *parser.TestContext) ([]*parser.ParsedFinding, error) {
    data, err := io.ReadAll(file)
    if err != nil {
        return nil, err
    }

    var output TrivyOutput
    if err := json.Unmarshal(data, &output); err != nil {
        return nil, fmt.Errorf("trivy: invalid JSON: %w", err)
    }

    var findings []*parser.ParsedFinding
    for _, result := range output.Results {
        for _, vuln := range result.Vulnerabilities {
            f := p.toFinding(vuln, result, output.ArtifactName)
            findings = append(findings, f)
        }
    }
    return findings, nil
}

func (p *TrivyParser) toFinding(vuln TrivyVuln, result TrivyResult, artifact string) *parser.ParsedFinding {
    title := vuln.Title
    if title == "" {
        title = vuln.VulnerabilityID
    }

    description := vuln.Description
    if vuln.FixedVersion != "" {
        description += fmt.Sprintf("\n\nFixed in version: %s", vuln.FixedVersion)
    }

    f := &parser.ParsedFinding{
        Title:            title,
        Description:      description,
        References:       strings.Join(vuln.References, "\n"),
        CVE:              vuln.VulnerabilityID,
        Severity:         normalizeSeverity(vuln.Severity),
        ComponentName:    vuln.PkgName,
        ComponentVersion: vuln.InstalledVersion,
        VulnIDFromTool:   vuln.VulnerabilityID,
        Active:           true,
    }

    // Extract CWE
    if len(vuln.CweIDs) > 0 {
        fmt.Sscanf(vuln.CweIDs[0], "CWE-%d", &f.CWE)
    }

    // Extract CVSS
    for _, cvss := range vuln.CVSS {
        if cvss.V3Vector != "" {
            f.CVSSv3 = cvss.V3Vector
            f.CVSSv3Score = &cvss.V3Score
        }
        if cvss.V4Vector != "" {
            f.CVSSv4 = cvss.V4Vector
            f.CVSSv4Score = &cvss.V4Score
        }
        break // use first source
    }

    return f
}

func normalizeSeverity(s string) string {
    switch strings.ToLower(s) {
    case "critical":             return "Critical"
    case "high":                 return "High"
    case "medium", "moderate":  return "Medium"
    case "low":                  return "Low"
    default:                     return "Info"
}
```

### `internal/infra/parser/sarif/parser.go` (SARIF — universal format)

```go
package sarif

// SARIF 2.1.0 parser — handles output from:
// - GitHub CodeQL
// - Checkmarx SARIF export
// - ESLint SARIF reporter
// - Any tool supporting SARIF

// SARIF structure:
type SARIFReport struct {
    Version string    `json:"version"`
    Runs    []SARIFRun `json:"runs"`
}
type SARIFRun struct {
    Tool    SARIFTool    `json:"tool"`
    Results []SARIFResult `json:"results"`
    Artifacts []SARIFArtifact `json:"artifacts"`
}
// ... (parse each result into ParsedFinding)
```

### `internal/infra/parser/bandit/parser.go` (Bandit — Python SAST)

```go
package bandit

// Bandit JSON structure:
type BanditReport struct {
    Errors  []interface{} `json:"errors"`
    Results []BanditIssue `json:"results"`
    Metrics BanditMetrics `json:"metrics"`
}
type BanditIssue struct {
    TestID    string `json:"test_id"`    // B101
    TestName  string `json:"test_name"`  // assert_used
    Severity  string `json:"issue_severity"` // HIGH|MEDIUM|LOW
    Confidence string `json:"issue_confidence"`
    Text      string `json:"issue_text"`
    Filename  string `json:"filename"`
    LineRange []int  `json:"line_range"`
    LineNumber int   `json:"line_number"`
    Code      string `json:"code"`
    CWE       BanditCWE `json:"issue_cwe"`
}
type BanditCWE struct {
    ID   int    `json:"id"`
    Link string `json:"link"`
}
```

## Parser Registry (must import all packages)

```go
// internal/infra/parser/registry.go
// This file ensures all parser packages are imported so their init() runs

package parser

import (
    _ "github.com/osv/services/scan-service/internal/infra/parser/trivy"
    _ "github.com/osv/services/scan-service/internal/infra/parser/grype"
    _ "github.com/osv/services/scan-service/internal/infra/parser/snyk"
    _ "github.com/osv/services/scan-service/internal/infra/parser/cyclonedx"
    _ "github.com/osv/services/scan-service/internal/infra/parser/bandit"
    _ "github.com/osv/services/scan-service/internal/infra/parser/semgrep"
    _ "github.com/osv/services/scan-service/internal/infra/parser/gosec"
    _ "github.com/osv/services/scan-service/internal/infra/parser/sarif"
    _ "github.com/osv/services/scan-service/internal/infra/parser/sonarqube"
    _ "github.com/osv/services/scan-service/internal/infra/parser/owasp_zap"
    _ "github.com/osv/services/scan-service/internal/infra/parser/nuclei"
    _ "github.com/osv/services/scan-service/internal/infra/parser/checkov"
    _ "github.com/osv/services/scan-service/internal/infra/parser/tfsec"
    _ "github.com/osv/services/scan-service/internal/infra/parser/nessus"
    _ "github.com/osv/services/scan-service/internal/infra/parser/dependency_check"
    _ "github.com/osv/services/scan-service/internal/infra/parser/retire_js"
    _ "github.com/osv/services/scan-service/internal/infra/parser/npm_audit"
    _ "github.com/osv/services/scan-service/internal/infra/parser/aws_security_hub"
    _ "github.com/osv/services/scan-service/internal/infra/parser/qualys"
    _ "github.com/osv/services/scan-service/internal/infra/parser/burp"
)
```

## Required Parsers (Minimum 20)

| Parser | Scan Type Name | Input Format | Priority |
|--------|---------------|-------------|---------|
| Trivy | `Trivy Scan` | JSON | P1 |
| Grype | `Grype` | JSON | P1 |
| Snyk | `Snyk Scan` | JSON | P1 |
| CycloneDX | `CycloneDX Scan` | JSON/XML | P1 |
| Bandit | `Bandit Scan` | JSON | P2 |
| Semgrep | `Semgrep JSON Report` | JSON | P2 |
| Gosec | `Gosec Scanner` | JSON | P2 |
| SARIF | `SARIF` | JSON | P2 |
| SonarQube | `SonarQube Scan` | JSON | P2 |
| OWASP ZAP | `ZAP Scan` | JSON/XML | P3 |
| Nuclei | `Nuclei Scan` | JSON | P3 |
| Checkov | `Checkov Scan` | JSON | P4 |
| tfsec | `Tfsec Scan` | JSON | P4 |
| Nessus | `Nessus Scan` | XML | P5 |
| Dependency-Check | `Dependency Check Scan` | JSON | P5 |
| RetireJS | `RetireJS Scan` | JSON | P5 |
| npm audit | `NPM Audit Scan` | JSON | P5 |
| AWS Security Hub | `AWS Security Hub Scan` | JSON | P5 |
| Qualys | `Qualys Scan` | XML | P5 |
| Burp Suite | `Burp Scan` | XML | P5 |

## Acceptance Criteria

- [x] `ParserFactory.GetParser("Trivy Scan")` → TrivyParser instance
- [x] `ParserFactory.GetParser("Unknown Scanner")` → error
- [x] `ParserFactory.ListScanTypes()` → ≥ 20 scan types (21 parsers registered)
- [x] TrivyParser: parse `testdata/trivy_container.json` → ≥ 1 finding với CVE
- [x] TrivyParser: CVSS v3 vector populated khi có trong input
- [x] BanditParser: parse `testdata/bandit.json` → findings với CWE populated
- [x] SARIFParser: parse GitHub CodeQL SARIF → findings với file_path và line
- [x] GosecParser: parse gosec JSON → findings với title, severity, CWE
- [x] CheckovParser: parse checkov JSON → IaC findings với resource context
- [x] NessusParser: parse Nessus JSON → network vuln findings với CVE, CVSS
- [x] NPMAuditParser: parse npm audit JSON → dependency findings
- [x] GrypeParser: parse Anchore Grype JSON → SCA findings với component/version
- [x] SnykParser: parse Snyk JSON → findings với CVE, CVSS score
- [x] CycloneDXParser: parse CycloneDX BOM JSON → vuln findings
- [x] OWASPZAPParser: parse ZAP JSON → DAST findings với severity
- [x] NucleiParser: parse Nuclei JSONL → findings với template-id
- [x] AWSSecurityHubParser: parse ASFF JSON → cloud security findings
- [x] All parsers handle empty/malformed input gracefully (return error, not panic)
- [x] Unit tests: at least 1 test per parser với real-world testdata sample — _(implemented)_
- [x] `go test ./internal/infra/parser/...` passes — _(verified)_

## Implementation Status: ✅ DONE

> `internal/infra/secparser/factory.go` — 21 parsers registered (Trivy, Bandit, Semgrep, Gosec, Checkov, SonarQube, Grype, Snyk, CycloneDX, DepCheck, ZAP, Nuclei, Nessus, OpenVAS, SARIF, RetireJS, NPMAudit, AWSSecHub, Qualys, Burp)
> Tất cả stubs đã được implement với JSON parsing logic thực sự
> 5 parser mới bổ sung: RetireJS, NPMAudit, AWSSecurityHub, Qualys, Burp
