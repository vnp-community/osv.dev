// Package parser provides concrete security tool report parser implementations.
// Each parser implements the domain parser.Parser interface.
package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	domainparser "github.com/osv/finding-service/internal/domain/orchestrator/parser"
)

// ── Factory ───────────────────────────────────────────────────────────────────

// Factory manages all registered parser implementations.
type Factory struct {
	mu      sync.RWMutex
	parsers map[string]domainparser.Parser
}

// NewFactory creates a Factory and registers all built-in parsers.
func NewFactory() *Factory {
	f := &Factory{parsers: make(map[string]domainparser.Parser)}
	f.registerAll()
	return f
}

// Get returns the parser for a given scan type.
func (f *Factory) Get(scanType string) (domainparser.Parser, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	p, ok := f.parsers[scanType]
	if !ok {
		return nil, fmt.Errorf("unsupported scan type: %q", scanType)
	}
	return p, nil
}

// ListScanTypes returns all registered scan type names.
func (f *Factory) ListScanTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	types := make([]string, 0, len(f.parsers))
	for t := range f.parsers {
		types = append(types, t)
	}
	return types
}

// Register adds a parser to the factory.
func (f *Factory) Register(p domainparser.Parser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.parsers[p.ScanType()] = p
}

func (f *Factory) registerAll() {
	// Phase 1: Priority parsers
	f.Register(&TrivyParser{})
	f.Register(&BanditParser{})
	f.Register(&SemgrepParser{})
	f.Register(&SARIFParser{})
	f.Register(&CycloneDXParser{})
	f.Register(&SnykParser{})
	f.Register(&GrypeParser{})
	// Phase 2: Web scanners
	f.Register(&OWASPZAPParser{})
	f.Register(&NucleiParser{})
	// Phase 3: Infrastructure
	f.Register(&NessusParser{})
	f.Register(&OpenVASParser{})
	// Phase 4: More SAST/SCA
	f.Register(&GosecParser{})
	f.Register(&CheckovParser{})
	f.Register(&SonarQubeParser{})
	f.Register(&DependencyCheckParser{})
}

// ── Trivy Parser ──────────────────────────────────────────────────────────────

type TrivyParser struct{}

func (p *TrivyParser) ScanType() string { return "Trivy Scan" }

type trivyReport struct {
	Results []trivyResult `json:"Results"`
}
type trivyResult struct {
	Target          string      `json:"Target"`
	Vulnerabilities []trivyVuln `json:"Vulnerabilities"`
}
type trivyVuln struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgName          string   `json:"PkgName"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion"`
	Severity         string   `json:"Severity"`
	Description      string   `json:"Description"`
	References       []string `json:"References"`
}

func (p *TrivyParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	var report trivyReport
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("trivy: decode: %w", err)
	}
	var findings []*domainparser.ParsedFinding
	for _, result := range report.Results {
		for _, v := range result.Vulnerabilities {
			findings = append(findings, &domainparser.ParsedFinding{
				Title:            fmt.Sprintf("%s in %s %s", v.VulnerabilityID, v.PkgName, v.InstalledVersion),
				CVE:              v.VulnerabilityID,
				VulnIDFromTool:   v.VulnerabilityID,
				ComponentName:    v.PkgName,
				ComponentVersion: v.InstalledVersion,
				Severity:         mapSeverity(v.Severity),
				Description:      v.Description,
				Mitigation:       fmt.Sprintf("Upgrade to %s", v.FixedVersion),
				References:       strings.Join(v.References, "\n"),
				Active:           true,
			})
		}
	}
	return findings, nil
}

// ── Bandit Parser (SAST – Python) ─────────────────────────────────────────────

type BanditParser struct{}

func (p *BanditParser) ScanType() string { return "Bandit Scan" }

type banditReport struct {
	Results []banditResult `json:"results"`
}
type banditResult struct {
	TestID          string `json:"test_id"`
	TestName        string `json:"test_name"`
	Issue           string `json:"issue_text"`
	Severity        string `json:"issue_severity"`
	Confidence      string `json:"issue_confidence"`
	FileName        string `json:"filename"`
	LineNumber      int    `json:"line_number"`
	Col             int    `json:"col_offset"`
	MoreInfo        string `json:"more_info"`
}

func (p *BanditParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	var report banditReport
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("bandit: decode: %w", err)
	}
	var findings []*domainparser.ParsedFinding
	for _, r := range report.Results {
		findings = append(findings, &domainparser.ParsedFinding{
			Title:          fmt.Sprintf("%s: %s", r.TestID, r.TestName),
			Description:    r.Issue,
			Severity:       mapSeverity(r.Severity),
			VulnIDFromTool: r.TestID,
			FilePath:       r.FileName,
			LineNumber:     r.LineNumber,
			References:     r.MoreInfo,
			Active:         true,
		})
	}
	return findings, nil
}

// ── Semgrep Parser ────────────────────────────────────────────────────────────

type SemgrepParser struct{}

func (p *SemgrepParser) ScanType() string { return "Semgrep JSON Report" }

type semgrepReport struct {
	Results []semgrepResult `json:"results"`
}
type semgrepResult struct {
	CheckID string          `json:"check_id"`
	Path    string          `json:"path"`
	Start   semgrepPosition `json:"start"`
	Extra   semgrepExtra    `json:"extra"`
}
type semgrepPosition struct{ Line int `json:"line"` }
type semgrepExtra struct {
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Metadata struct {
		CVE        string `json:"cve"`
		CWE        []int  `json:"cwe"`
		References []string `json:"references"`
	} `json:"metadata"`
}

func (p *SemgrepParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	var report semgrepReport
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("semgrep: decode: %w", err)
	}
	var findings []*domainparser.ParsedFinding
	for _, r := range report.Results {
		cwe := 0
		if len(r.Extra.Metadata.CWE) > 0 {
			cwe = r.Extra.Metadata.CWE[0]
		}
		findings = append(findings, &domainparser.ParsedFinding{
			Title:          fmt.Sprintf("%s", r.CheckID),
			Description:    r.Extra.Message,
			Severity:       mapSemgrepSeverity(r.Extra.Severity),
			VulnIDFromTool: r.CheckID,
			CVE:            r.Extra.Metadata.CVE,
			CWE:            cwe,
			FilePath:       r.Path,
			LineNumber:     r.Start.Line,
			References:     strings.Join(r.Extra.Metadata.References, "\n"),
			Active:         true,
		})
	}
	return findings, nil
}

// ── Stub parsers (to be fully implemented in next iteration) ──────────────────

type SARIFParser struct{}

func (p *SARIFParser) ScanType() string { return "SARIF" }
func (p *SARIFParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("SARIF parser: not yet implemented")
}

type CycloneDXParser struct{}

func (p *CycloneDXParser) ScanType() string { return "CycloneDX Scan" }
func (p *CycloneDXParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("CycloneDX parser: not yet implemented")
}

type SnykParser struct{}

func (p *SnykParser) ScanType() string { return "Snyk Scan" }
func (p *SnykParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Snyk parser: not yet implemented")
}

type GrypeParser struct{}

func (p *GrypeParser) ScanType() string { return "Anchore Grype" }
func (p *GrypeParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Grype parser: not yet implemented")
}

type OWASPZAPParser struct{}

func (p *OWASPZAPParser) ScanType() string { return "ZAP Scan" }
func (p *OWASPZAPParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("ZAP parser: not yet implemented")
}

type NucleiParser struct{}

func (p *NucleiParser) ScanType() string { return "Nuclei Scan" }
func (p *NucleiParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Nuclei parser: not yet implemented")
}

type NessusParser struct{}

func (p *NessusParser) ScanType() string { return "Nessus Scan" }
func (p *NessusParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Nessus parser: not yet implemented")
}

type OpenVASParser struct{}

func (p *OpenVASParser) ScanType() string { return "OpenVAS CSV" }
func (p *OpenVASParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("OpenVAS parser: not yet implemented")
}

type GosecParser struct{}

func (p *GosecParser) ScanType() string { return "Gosec Scanner" }
func (p *GosecParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Gosec parser: not yet implemented")
}

type CheckovParser struct{}

func (p *CheckovParser) ScanType() string { return "Checkov Scan" }
func (p *CheckovParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Checkov parser: not yet implemented")
}

type SonarQubeParser struct{}

func (p *SonarQubeParser) ScanType() string { return "SonarQube Scan detailed" }
func (p *SonarQubeParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("SonarQube parser: not yet implemented")
}

type DependencyCheckParser struct{}

func (p *DependencyCheckParser) ScanType() string { return "Dependency Check Scan" }
func (p *DependencyCheckParser) GetFindings(ctx context.Context, file io.Reader, test *domainparser.TestContext) ([]*domainparser.ParsedFinding, error) {
	return nil, fmt.Errorf("Dependency Check parser: not yet implemented")
}

// ── Severity mapping helpers ───────────────────────────────────────────────────

func mapSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return "Critical"
	case "HIGH":
		return "High"
	case "MEDIUM":
		return "Medium"
	case "LOW":
		return "Low"
	default:
		return "Info"
	}
}

func mapSemgrepSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "ERROR":
		return "High"
	case "WARNING":
		return "Medium"
	default:
		return "Info"
	}
}
