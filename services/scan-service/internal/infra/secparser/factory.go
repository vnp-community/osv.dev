// Package secparser provides the security tool report parser factory and implementations.
// Each parser implements the Parser interface for use with the ImportScanUseCase.
// This package handles normalized output from 15+ security tools.
package secparser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	importuc "github.com/osv/scan-service/internal/usecase/import"
)

// ─── Registry ─────────────────────────────────────────────────────────────────

// Factory maintains all registered security tool parsers.
type Factory struct {
	mu      sync.RWMutex
	parsers map[string]importuc.Parser
}

// NewFactory creates a Factory with all built-in parsers pre-registered.
func NewFactory() *Factory {
	f := &Factory{parsers: make(map[string]importuc.Parser)}
	f.registerAll()
	return f
}

// GetParser returns the parser for a given scan type.
func (f *Factory) GetParser(scanType string) (importuc.Parser, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	p, ok := f.parsers[scanType]
	if !ok {
		return nil, fmt.Errorf("unsupported scan type: %q", scanType)
	}
	return p, nil
}

// ListScanTypes returns all registered scan type keys.
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
func (f *Factory) Register(p importuc.Parser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.parsers[p.ScanType()] = p
}

func (f *Factory) registerAll() {
	// SAST
	f.Register(&BanditParser{})
	f.Register(&SemgrepParser{})
	f.Register(&GosecParser{})
	f.Register(&CheckovParser{})
	f.Register(&SonarQubeParser{})
	// SCA / Container
	f.Register(&TrivyParser{})
	f.Register(&GrypeParser{})
	f.Register(&SnykParser{})
	f.Register(&CycloneDXParser{})
	f.Register(&DependencyCheckParser{})
	// Web / DAST
	f.Register(&OWASPZAPParser{})
	f.Register(&NucleiParser{})
	// Infrastructure
	f.Register(&NessusParser{})
	f.Register(&OpenVASParser{})
	// Universal
	f.Register(&SARIFParser{})
	// Additional parsers (>=20 total)
	f.Register(&RetireJSParser{})
	f.Register(&NPMAuditParser{})
	f.Register(&AWSSecurityHubParser{})
	f.Register(&QualysParser{})
	f.Register(&BurpParser{})
}

// ─── Trivy ────────────────────────────────────────────────────────────────────

// TrivyParser parses Trivy JSON container/filesystem scan reports.
type TrivyParser struct{}

func (p *TrivyParser) ScanType() string { return "Trivy Scan" }

type trivyReport struct {
	Results []struct {
		Target          string `json:"Target"`
		Vulnerabilities []struct {
			VulnerabilityID  string   `json:"VulnerabilityID"`
			PkgName          string   `json:"PkgName"`
			InstalledVersion string   `json:"InstalledVersion"`
			FixedVersion     string   `json:"FixedVersion"`
			Severity         string   `json:"Severity"`
			Description      string   `json:"Description"`
			References       []string `json:"References"`
			CVSS             struct {
				Nvd struct {
					V3Score  float64 `json:"V3Score"`
					V3Vector string  `json:"V3Vector"`
				} `json:"nvd"`
			} `json:"CVSS"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

func (p *TrivyParser) GetFindings(_ context.Context, file io.Reader, tc *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report trivyReport
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("trivy: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, result := range report.Results {
		for _, v := range result.Vulnerabilities {
			score := v.CVSS.Nvd.V3Score
			f := &importuc.ParsedFinding{
				Title:            fmt.Sprintf("%s in %s %s", v.VulnerabilityID, v.PkgName, v.InstalledVersion),
				CVE:              v.VulnerabilityID,
				VulnIDFromTool:   v.VulnerabilityID,
				ComponentName:    v.PkgName,
				ComponentVersion: v.InstalledVersion,
				Severity:         mapSeverity(v.Severity),
				Description:      v.Description,
				Mitigation:       fmt.Sprintf("Upgrade to %s", v.FixedVersion),
				References:       strings.Join(v.References, "\n"),
				CVSSv3:           v.CVSS.Nvd.V3Vector,
				Active:           true,
			}
			if score > 0 {
				s := score
				f.CVSSv3Score = &s
			}
			out = append(out, f)
		}
	}
	return out, nil
}

// ─── Bandit ───────────────────────────────────────────────────────────────────

// BanditParser parses Bandit SAST JSON reports (Python).
type BanditParser struct{}

func (p *BanditParser) ScanType() string { return "Bandit Scan" }

func (p *BanditParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Results []struct {
			TestID     string `json:"test_id"`
			TestName   string `json:"test_name"`
			IssueText  string `json:"issue_text"`
			Severity   string `json:"issue_severity"`
			Confidence string `json:"issue_confidence"`
			Filename   string `json:"filename"`
			LineNumber int    `json:"line_number"`
			MoreInfo   string `json:"more_info"`
		} `json:"results"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("bandit: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, r := range report.Results {
		out = append(out, &importuc.ParsedFinding{
			Title:          fmt.Sprintf("%s: %s", r.TestID, r.TestName),
			Description:    r.IssueText,
			Severity:       mapSeverity(r.Severity),
			VulnIDFromTool: r.TestID,
			FilePath:       r.Filename,
			LineNumber:     r.LineNumber,
			References:     r.MoreInfo,
			Active:         true,
		})
	}
	return out, nil
}

// ─── Semgrep ──────────────────────────────────────────────────────────────────

// SemgrepParser parses Semgrep JSON SAST reports.
type SemgrepParser struct{}

func (p *SemgrepParser) ScanType() string { return "Semgrep JSON Report" }

func (p *SemgrepParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
				Metadata struct {
					CVE        string `json:"cve"`
					References []string `json:"references"`
				} `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("semgrep: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, r := range report.Results {
		out = append(out, &importuc.ParsedFinding{
			Title:          r.CheckID,
			Description:    r.Extra.Message,
			Severity:       mapSemgrepSeverity(r.Extra.Severity),
			VulnIDFromTool: r.CheckID,
			CVE:            r.Extra.Metadata.CVE,
			FilePath:       r.Path,
			LineNumber:     r.Start.Line,
			References:     strings.Join(r.Extra.Metadata.References, "\n"),
			Active:         true,
		})
	}
	return out, nil
}

// ─── Gosec ────────────────────────────────────────────────────────────────────

// GosecParser parses gosec JSON SAST reports (Go).
type GosecParser struct{}

func (p *GosecParser) ScanType() string { return "Gosec Scanner" }

func (p *GosecParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Issues []struct {
			RuleID     string `json:"rule_id"`
			Severity   string `json:"severity"`
			Confidence string `json:"confidence"`
			Details    string `json:"details"`
			File       string `json:"file"`
			Line       string `json:"line"`
			Code       string `json:"code"`
		} `json:"Issues"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("gosec: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, issue := range report.Issues {
		out = append(out, &importuc.ParsedFinding{
			Title:          fmt.Sprintf("G%s: %s", issue.RuleID, issue.Details),
			Description:    fmt.Sprintf("%s\n\nCode:\n%s", issue.Details, issue.Code),
			Severity:       mapSeverity(issue.Severity),
			VulnIDFromTool: issue.RuleID,
			FilePath:       issue.File,
			Active:         true,
		})
	}
	return out, nil
}

// ─── Grype ────────────────────────────────────────────────────────────────────

// GrypeParser parses Anchore Grype JSON vulnerability reports.
type GrypeParser struct{}

func (p *GrypeParser) ScanType() string { return "Anchore Grype" }

func (p *GrypeParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Matches []struct {
			Vulnerability struct {
				ID          string   `json:"id"`
				Severity    string   `json:"severity"`
				Description string   `json:"description"`
				Cvss        []struct {
					Version string  `json:"version"`
					Vector  string  `json:"vector"`
					Metrics struct {
						BaseScore float64 `json:"baseScore"`
					} `json:"metrics"`
				} `json:"cvss"`
				Fix struct {
					Versions []string `json:"versions"`
				} `json:"fix"`
				URLs []string `json:"urls"`
			} `json:"vulnerability"`
			Artifact struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Type    string `json:"type"`
			} `json:"artifact"`
		} `json:"matches"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("grype: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, m := range report.Matches {
		v := m.Vulnerability
		a := m.Artifact
		f := &importuc.ParsedFinding{
			Title:            fmt.Sprintf("%s in %s %s", v.ID, a.Name, a.Version),
			CVE:              v.ID,
			VulnIDFromTool:   v.ID,
			ComponentName:    a.Name,
			ComponentVersion: a.Version,
			Severity:         mapSeverity(v.Severity),
			Description:      v.Description,
			References:       strings.Join(v.URLs, "\n"),
			Active:           true,
		}
		if len(v.Fix.Versions) > 0 {
			f.Mitigation = "Upgrade to: " + strings.Join(v.Fix.Versions, ", ")
		}
		for _, cvss := range v.Cvss {
			if strings.HasPrefix(cvss.Version, "3") {
				f.CVSSv3 = cvss.Vector
				score := cvss.Metrics.BaseScore
				f.CVSSv3Score = &score
				break
			}
		}
		out = append(out, f)
	}
	return out, nil
}

// ─── Snyk ─────────────────────────────────────────────────────────────────────

// SnykParser parses Snyk JSON vulnerability reports.
type SnykParser struct{}

func (p *SnykParser) ScanType() string { return "Snyk Scan" }

func (p *SnykParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Vulnerabilities []struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Severity    string   `json:"severity"`
			CVSSScore   float64  `json:"cvssScore"`
			CVSSv3      string   `json:"CVSSv3"`
			From        []string `json:"from"`
			PackageName string   `json:"packageName"`
			Version     string   `json:"version"`
			References  []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			} `json:"references"`
			Identifiers struct {
				CVE []string `json:"CVE"`
				CWE []string `json:"CWE"`
			} `json:"identifiers"`
		} `json:"vulnerabilities"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("snyk: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, v := range report.Vulnerabilities {
		var refs []string
		for _, r := range v.References {
			refs = append(refs, r.URL)
		}
		var cve string
		if len(v.Identifiers.CVE) > 0 {
			cve = v.Identifiers.CVE[0]
		}
		f := &importuc.ParsedFinding{
			Title:            v.Title,
			Description:      v.Description,
			Severity:         mapSeverity(v.Severity),
			CVE:              cve,
			VulnIDFromTool:   v.ID,
			ComponentName:    v.PackageName,
			ComponentVersion: v.Version,
			CVSSv3:           v.CVSSv3,
			References:       strings.Join(refs, "\n"),
			Active:           true,
		}
		if v.CVSSScore > 0 {
			s := v.CVSSScore
			f.CVSSv3Score = &s
		}
		out = append(out, f)
	}
	return out, nil
}

// ─── CycloneDX ────────────────────────────────────────────────────────────────

// CycloneDXParser parses CycloneDX SBOM JSON vulnerability reports.
type CycloneDXParser struct{}

func (p *CycloneDXParser) ScanType() string { return "CycloneDX Scan" }

func (p *CycloneDXParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Vulnerabilities []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Ratings     []struct {
				Score    float64 `json:"score"`
				Severity string  `json:"severity"`
				Method   string  `json:"method"`
				Vector   string  `json:"vector"`
			} `json:"ratings"`
			Affects []struct {
				Ref string `json:"ref"`
			} `json:"affects"`
			Advisories []struct {
				URL string `json:"url"`
			} `json:"advisories"`
		} `json:"vulnerabilities"`
		Components []struct {
			BOMRef  string `json:"bom-ref"`
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"components"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("cyclonedx: %w", err)
	}
	// Build component lookup
	compMap := make(map[string]struct{ Name, Version string })
	for _, c := range report.Components {
		compMap[c.BOMRef] = struct{ Name, Version string }{c.Name, c.Version}
	}
	var out []*importuc.ParsedFinding
	for _, v := range report.Vulnerabilities {
		sev := "Info"
		var cvssScore *float64
		var cvssVec string
		for _, r := range v.Ratings {
			sev = mapSeverity(r.Severity)
			if r.Score > 0 {
				s := r.Score
				cvssScore = &s
			}
			if r.Vector != "" {
				cvssVec = r.Vector
			}
			break
		}
		var compName, compVer string
		if len(v.Affects) > 0 {
			if comp, ok := compMap[v.Affects[0].Ref]; ok {
				compName = comp.Name
				compVer = comp.Version
			}
		}
		var refs []string
		for _, a := range v.Advisories {
			refs = append(refs, a.URL)
		}
		f := &importuc.ParsedFinding{
			Title:            v.ID,
			Description:      v.Description,
			Severity:         sev,
			CVE:              v.ID,
			VulnIDFromTool:   v.ID,
			ComponentName:    compName,
			ComponentVersion: compVer,
			CVSSv3:           cvssVec,
			CVSSv3Score:      cvssScore,
			References:       strings.Join(refs, "\n"),
			Active:           true,
		}
		out = append(out, f)
	}
	return out, nil
}

// ─── Checkov ──────────────────────────────────────────────────────────────────

// CheckovParser parses Checkov IaC JSON reports.
type CheckovParser struct{}

func (p *CheckovParser) ScanType() string { return "Checkov Scan" }

func (p *CheckovParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Results struct {
			FailedChecks []struct {
				CheckID       string   `json:"check_id"`
				CheckName     string   `json:"check"`
				Resource      string   `json:"resource"`
				File          string   `json:"file_path"`
				FileLineRange []int    `json:"file_line_range"`
				Guideline     string   `json:"guideline"`
				Severity      string   `json:"severity"`
			} `json:"failed_checks"`
		} `json:"results"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("checkov: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, c := range report.Results.FailedChecks {
		sev := "Medium"
		if c.Severity != "" {
			sev = mapSeverity(c.Severity)
		}
		line := 0
		if len(c.FileLineRange) > 0 {
			line = c.FileLineRange[0]
		}
		out = append(out, &importuc.ParsedFinding{
			Title:          fmt.Sprintf("%s: %s", c.CheckID, c.CheckName),
			Description:    fmt.Sprintf("Resource: %s\n%s", c.Resource, c.CheckName),
			Severity:       sev,
			VulnIDFromTool: c.CheckID,
			FilePath:       c.File,
			LineNumber:     line,
			References:     c.Guideline,
			Active:         true,
		})
	}
	return out, nil
}

// ─── SonarQube ────────────────────────────────────────────────────────────────

// SonarQubeParser parses SonarQube JSON export reports.
type SonarQubeParser struct{}

func (p *SonarQubeParser) ScanType() string { return "SonarQube Scan detailed" }

func (p *SonarQubeParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Issues []struct {
			Key       string `json:"key"`
			Rule      string `json:"rule"`
			Severity  string `json:"severity"`
			Component string `json:"component"`
			Message   string `json:"message"`
			Line      int    `json:"line"`
			Type      string `json:"type"` // BUG, VULNERABILITY, CODE_SMELL
		} `json:"issues"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("sonarqube: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, issue := range report.Issues {
		if issue.Type != "VULNERABILITY" && issue.Type != "BUG" {
			continue // only security-relevant issues
		}
		out = append(out, &importuc.ParsedFinding{
			Title:          fmt.Sprintf("[%s] %s", issue.Rule, issue.Message),
			Description:    issue.Message,
			Severity:       mapSonarSeverity(issue.Severity),
			VulnIDFromTool: issue.Key,
			FilePath:       issue.Component,
			LineNumber:     issue.Line,
			Active:         true,
		})
	}
	return out, nil
}

func mapSonarSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "BLOCKER", "CRITICAL":
		return "Critical"
	case "MAJOR":
		return "High"
	case "MINOR":
		return "Medium"
	case "INFO":
		return "Info"
	default:
		return "Low"
	}
}

// ─── OWASP ZAP ────────────────────────────────────────────────────────────────

// OWASPZAPParser parses OWASP ZAP JSON scan reports.
type OWASPZAPParser struct{}

func (p *OWASPZAPParser) ScanType() string { return "ZAP Scan" }

func (p *OWASPZAPParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Site []struct {
			Alerts []struct {
				Alert       string `json:"alert"`
				Name        string `json:"name"`
				Description string `json:"desc"`
				Solution    string `json:"solution"`
				Reference   string `json:"reference"`
				Risk        string `json:"riskdesc"` // "High (Medium)", "Medium (Low)"
				CWEID       int    `json:"cweid"`
				Instances   []struct {
					URI     string `json:"uri"`
					Method  string `json:"method"`
				} `json:"instances"`
			} `json:"alerts"`
		} `json:"site"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("zap: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, site := range report.Site {
		for _, alert := range site.Alerts {
			// Extract severity from riskdesc: "High (Medium)" → "High"
			sev := "Info"
			parts := strings.SplitN(alert.Risk, " ", 2)
			if len(parts) > 0 {
				sev = mapSeverity(parts[0])
			}
			var endpoints []string
			for _, inst := range alert.Instances {
				endpoints = append(endpoints, inst.URI)
			}
			out = append(out, &importuc.ParsedFinding{
				Title:       alert.Name,
				Description: alert.Description,
				Mitigation:  alert.Solution,
				References:  alert.Reference,
				Severity:    sev,
				CWE:         alert.CWEID,
				Active:      true,
			})
			_ = endpoints
		}
	}
	return out, nil
}

// ─── Nuclei ───────────────────────────────────────────────────────────────────

// NucleiParser parses Nuclei JSON scan output (JSONL format).
type NucleiParser struct{}

func (p *NucleiParser) ScanType() string { return "Nuclei Scan" }

func (p *NucleiParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("nuclei: read: %w", err)
	}
	// Nuclei outputs JSONL (one JSON object per line)
	var out []*importuc.ParsedFinding
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var item struct {
			TemplateID string `json:"template-id"`
			Info       struct {
				Name        string   `json:"name"`
				Severity    string   `json:"severity"`
				Description string   `json:"description"`
				Tags        []string `json:"tags"`
				Reference   []string `json:"reference"`
			} `json:"info"`
			Host      string `json:"host"`
			MatchedAt string `json:"matched-at"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		out = append(out, &importuc.ParsedFinding{
			Title:          item.Info.Name,
			Description:    item.Info.Description,
			Severity:       mapSeverity(item.Info.Severity),
			VulnIDFromTool: item.TemplateID,
			References:     strings.Join(item.Info.Reference, "\n"),
			Tags:           item.Info.Tags,
			Active:         true,
		})
	}
	return out, nil
}

// ─── Nessus ───────────────────────────────────────────────────────────────────

// NessusParser parses Nessus CSV export format (simplified JSON wrapper).
// For full XML .nessus parsing, use an XML library.
type NessusParser struct{}

func (p *NessusParser) ScanType() string { return "Nessus Scan" }

func (p *NessusParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	// Nessus JSON export format (from REST API /scans/{id}/export?format=nessus_db)
	var report struct {
		Hosts []struct {
			Hostname string `json:"hostname"`
			Vulnerabilities []struct {
				PluginID   int    `json:"plugin_id"`
				PluginName string `json:"plugin_name"`
				Severity   int    `json:"severity"` // 0=info,1=low,2=medium,3=high,4=critical
				Description string `json:"description"`
				Solution    string `json:"solution"`
				CVE         string `json:"cve"`
				CVSSScore   float64 `json:"cvss_base_score"`
				CVSSVector  string  `json:"cvss_vector"`
				SeeAlso     string  `json:"see_also"`
			} `json:"vulnerabilities"`
		} `json:"hosts"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("nessus: %w", err)
	}
	nessusServerityMap := []string{"Info", "Low", "Medium", "High", "Critical"}
	var out []*importuc.ParsedFinding
	for _, host := range report.Hosts {
		for _, v := range host.Vulnerabilities {
			sev := "Info"
			if v.Severity >= 0 && v.Severity < len(nessusServerityMap) {
				sev = nessusServerityMap[v.Severity]
			}
			f := &importuc.ParsedFinding{
				Title:          fmt.Sprintf("[%d] %s", v.PluginID, v.PluginName),
				Description:    v.Description,
				Mitigation:     v.Solution,
				References:     v.SeeAlso,
				Severity:       sev,
				CVE:            v.CVE,
				CVSSv3:         v.CVSSVector,
				VulnIDFromTool: fmt.Sprintf("%d", v.PluginID),
				Active:         true,
			}
			if v.CVSSScore > 0 {
				s := v.CVSSScore
				f.CVSSv3Score = &s
			}
			out = append(out, f)
		}
	}
	return out, nil
}

// ─── OpenVAS ──────────────────────────────────────────────────────────────────

// OpenVASParser parses OpenVAS CSV export format.
type OpenVASParser struct{}

func (p *OpenVASParser) ScanType() string { return "OpenVAS CSV" }

func (p *OpenVASParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("openvas: read: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	var out []*importuc.ParsedFinding
	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cols := strings.Split(line, ",")
		if len(cols) < 7 {
			continue
		}
		// CSV columns: IP, Hostname, Port, Port Protocol, CVSS, Severity, Solution Type, NVT Name, Summary, Specific Result, NVT OID, CVEs, ...
		out = append(out, &importuc.ParsedFinding{
			Title:       strings.Trim(safeCol(cols, 7), "\""),
			Description: strings.Trim(safeCol(cols, 8), "\""),
			Severity:    mapSeverity(strings.Trim(safeCol(cols, 5), "\"")),
			CVE:         strings.Trim(safeCol(cols, 11), "\""),
			Active:      true,
		})
	}
	return out, nil
}

func safeCol(cols []string, i int) string {
	if i < len(cols) {
		return cols[i]
	}
	return ""
}

// ─── Dependency-Check ─────────────────────────────────────────────────────────

// DependencyCheckParser parses OWASP Dependency-Check JSON reports.
type DependencyCheckParser struct{}

func (p *DependencyCheckParser) ScanType() string { return "Dependency Check Scan" }

func (p *DependencyCheckParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Dependencies []struct {
			FileName        string `json:"fileName"`
			PackageName     string `json:"packages"`
			Vulnerabilities []struct {
				Name        string  `json:"name"`
				Description string  `json:"description"`
				Severity    string  `json:"severity"`
				CVSSV3      struct {
					BaseScore  float64 `json:"baseScore"`
					BaseVector string  `json:"attackVector"`
				} `json:"cvssv3"`
				References []struct {
					Source string `json:"source"`
					URL    string `json:"url"`
				} `json:"references"`
				CWE string `json:"cwe"`
			} `json:"vulnerabilities"`
		} `json:"dependencies"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("dependency-check: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, dep := range report.Dependencies {
		for _, v := range dep.Vulnerabilities {
			var refs []string
			for _, r := range v.References {
				refs = append(refs, r.URL)
			}
			f := &importuc.ParsedFinding{
				Title:          fmt.Sprintf("%s in %s", v.Name, dep.FileName),
				Description:    v.Description,
				Severity:       mapSeverity(v.Severity),
				CVE:            v.Name,
				VulnIDFromTool: v.Name,
				ComponentName:  dep.FileName,
				References:     strings.Join(refs, "\n"),
				Active:         true,
			}
			if v.CVSSV3.BaseScore > 0 {
				s := v.CVSSV3.BaseScore
				f.CVSSv3Score = &s
			}
			out = append(out, f)
		}
	}
	return out, nil
}

// ─── SARIF ────────────────────────────────────────────────────────────────────

// SARIFParser parses SARIF 2.1.0 reports (GitHub CodeQL, ESLint SARIF, Checkmarx, etc.).
type SARIFParser struct{}

func (p *SARIFParser) ScanType() string { return "SARIF" }

func (p *SARIFParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID               string `json:"id"`
						ShortDescription struct {
							Text string `json:"text"`
						} `json:"shortDescription"`
						FullDescription struct {
							Text string `json:"text"`
						} `json:"fullDescription"`
						DefaultConfiguration struct {
							Level string `json:"level"` // error|warning|note
						} `json:"defaultConfiguration"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"` // error|warning|note
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("sarif: %w", err)
	}
	// Build rule lookup
	ruleMap := make(map[string]string)
	for _, run := range report.Runs {
		for _, rule := range run.Tool.Driver.Rules {
			ruleMap[rule.ID] = rule.ShortDescription.Text
		}
	}
	var out []*importuc.ParsedFinding
	for _, run := range report.Runs {
		for _, result := range run.Results {
			title := result.RuleID
			if desc, ok := ruleMap[result.RuleID]; ok && desc != "" {
				title = desc
			}
			filePath := ""
			lineNum := 0
			if len(result.Locations) > 0 {
				filePath = result.Locations[0].PhysicalLocation.ArtifactLocation.URI
				lineNum = result.Locations[0].PhysicalLocation.Region.StartLine
			}
			out = append(out, &importuc.ParsedFinding{
				Title:          title,
				Description:    result.Message.Text,
				Severity:       mapSARIFLevel(result.Level),
				VulnIDFromTool: result.RuleID,
				FilePath:       filePath,
				LineNumber:     lineNum,
				Active:         true,
			})
		}
	}
	return out, nil
}

func mapSARIFLevel(level string) string {
	switch strings.ToLower(level) {
	case "error":
		return "High"
	case "warning":
		return "Medium"
	case "note", "info", "open":
		return "Info"
	default:
		return "Medium"
	}
}

// ─── RetireJS ─────────────────────────────────────────────────────────────────

// RetireJSParser parses RetireJS JSON vulnerability reports.
type RetireJSParser struct{}

func (p *RetireJSParser) ScanType() string { return "RetireJS Scan" }

func (p *RetireJSParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var results []struct {
		File     string `json:"file"`
		Results []struct {
			Component  string `json:"component"`
			Version    string `json:"version"`
			Detection  string `json:"detection"`
			Vulnerabilities []struct {
				Info          []string `json:"info"`
				Severity      string   `json:"severity"`
				Summary       string   `json:"summary"`
				Identifiers   struct {
					CVE  []string `json:"CVE"`
					Bug  string   `json:"bug"`
				} `json:"identifiers"`
			} `json:"vulnerabilities"`
		} `json:"results"`
	}
	if err := json.NewDecoder(file).Decode(&results); err != nil {
		return nil, fmt.Errorf("retirejs: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, file := range results {
		for _, result := range file.Results {
			for _, v := range result.Vulnerabilities {
				var cve string
				if len(v.Identifiers.CVE) > 0 {
					cve = v.Identifiers.CVE[0]
				}
				out = append(out, &importuc.ParsedFinding{
					Title:            fmt.Sprintf("%s in %s %s", v.Summary, result.Component, result.Version),
					Description:      v.Summary,
					Severity:         mapSeverity(v.Severity),
					CVE:              cve,
					ComponentName:    result.Component,
					ComponentVersion: result.Version,
					FilePath:         file.File,
					References:       strings.Join(v.Info, "\n"),
					Active:           true,
				})
			}
		}
	}
	return out, nil
}

// ─── NPM Audit ────────────────────────────────────────────────────────────────

// NPMAuditParser parses npm audit JSON output.
type NPMAuditParser struct{}

func (p *NPMAuditParser) ScanType() string { return "NPM Audit Scan" }

func (p *NPMAuditParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Vulnerabilities map[string]struct {
			Name     string   `json:"name"`
			Severity string   `json:"severity"`
			Via      []interface{} `json:"via"` // can be string or object
			Range    string   `json:"range"`
			FixAvailable bool `json:"fixAvailable"`
		} `json:"vulnerabilities"`
		Advisories map[string]struct {
			ID             int    `json:"id"`
			Title          string `json:"title"`
			Severity       string `json:"severity"`
			Overview       string `json:"overview"`
			ModuleName     string `json:"module_name"`
			VulnerableVersions string `json:"vulnerable_versions"`
			PatchedVersions    string `json:"patched_versions"`
			CVE            []string `json:"cves"`
			URL            string `json:"url"`
		} `json:"advisories"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("npm-audit: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, adv := range report.Advisories {
		var cve string
		if len(adv.CVE) > 0 {
			cve = adv.CVE[0]
		}
		fix := ""
		if adv.PatchedVersions != "" {
			fix = "Upgrade to: " + adv.PatchedVersions
		}
		out = append(out, &importuc.ParsedFinding{
			Title:            adv.Title,
			Description:      adv.Overview,
			Mitigation:       fix,
			Severity:         mapSeverity(adv.Severity),
			CVE:              cve,
			VulnIDFromTool:   fmt.Sprintf("%d", adv.ID),
			ComponentName:    adv.ModuleName,
			ComponentVersion: adv.VulnerableVersions,
			References:       adv.URL,
			Active:           true,
		})
	}
	return out, nil
}

// ─── AWS Security Hub ─────────────────────────────────────────────────────────

// AWSSecurityHubParser parses AWS Security Hub ASFF JSON findings.
type AWSSecurityHubParser struct{}

func (p *AWSSecurityHubParser) ScanType() string { return "AWS Security Hub Scan" }

func (p *AWSSecurityHubParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Findings []struct {
			ID          string `json:"Id"`
			Title       string `json:"Title"`
			Description string `json:"Description"`
			Severity    struct {
				Label string `json:"Label"` // CRITICAL|HIGH|MEDIUM|LOW|INFORMATIONAL
			} `json:"Severity"`
			Remediation struct {
				Recommendation struct {
					Text string `json:"Text"`
					URL  string `json:"Url"`
				} `json:"Recommendation"`
			} `json:"Remediation"`
			Types []string `json:"Types"`
		} `json:"Findings"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("aws-security-hub: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, f := range report.Findings {
		sev := mapSeverity(f.Severity.Label)
		if strings.ToUpper(f.Severity.Label) == "INFORMATIONAL" {
			sev = "Info"
		}
		out = append(out, &importuc.ParsedFinding{
			Title:          f.Title,
			Description:    f.Description,
			Mitigation:     f.Remediation.Recommendation.Text,
			References:     f.Remediation.Recommendation.URL,
			Severity:       sev,
			VulnIDFromTool: f.ID,
			Active:         true,
		})
	}
	return out, nil
}

// ─── Qualys ───────────────────────────────────────────────────────────────────

// QualysParser parses Qualys vulnerability JSON export (simplified).
type QualysParser struct{}

func (p *QualysParser) ScanType() string { return "Qualys Scan" }

func (p *QualysParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Host []struct {
			IP   string `json:"ip"`
			Vulns []struct {
				QID      int    `json:"qid"`
				Title    string `json:"title"`
				Severity int    `json:"severity"` // 1-5
				CVE      string `json:"cve"`
				Diagnosis string `json:"diagnosis"`
				Solution  string `json:"solution"`
			} `json:"vulns"`
		} `json:"host"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("qualys: %w", err)
	}
	qualysSevMap := []string{"Info", "Info", "Low", "Medium", "High", "Critical"}
	var out []*importuc.ParsedFinding
	for _, host := range report.Host {
		for _, v := range host.Vulns {
			sev := "Info"
			if v.Severity >= 1 && v.Severity < len(qualysSevMap) {
				sev = qualysSevMap[v.Severity]
			}
			out = append(out, &importuc.ParsedFinding{
				Title:          v.Title,
				Description:    v.Diagnosis,
				Mitigation:     v.Solution,
				Severity:       sev,
				CVE:            v.CVE,
				VulnIDFromTool: fmt.Sprintf("%d", v.QID),
				Active:         true,
			})
		}
	}
	return out, nil
}

// ─── Burp Suite ───────────────────────────────────────────────────────────────

// BurpParser parses Burp Suite JSON export reports.
type BurpParser struct{}

func (p *BurpParser) ScanType() string { return "Burp Scan" }

func (p *BurpParser) GetFindings(_ context.Context, file io.Reader, _ *importuc.ParseTestContext) ([]*importuc.ParsedFinding, error) {
	var report struct {
		Issues []struct {
			TypeIndex   int    `json:"type_index"`
			Name        string `json:"name"`
			Severity    string `json:"severity"` // high|medium|low|info
			Confidence  string `json:"confidence"`
			Description string `json:"issue_description"`
			Remediation string `json:"remediation_background"`
			References  string `json:"references"`
			SerialNumber string `json:"serial_number"`
		} `json:"issue_events"`
	}
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return nil, fmt.Errorf("burp: %w", err)
	}
	var out []*importuc.ParsedFinding
	for _, issue := range report.Issues {
		out = append(out, &importuc.ParsedFinding{
			Title:          issue.Name,
			Description:    issue.Description,
			Mitigation:     issue.Remediation,
			References:     issue.References,
			Severity:       mapSeverity(issue.Severity),
			VulnIDFromTool: issue.SerialNumber,
			Active:         true,
		})
	}
	return out, nil
}

// ─── Severity helpers ─────────────────────────────────────────────────────────

func mapSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return "Critical"
	case "HIGH":
		return "High"
	case "MEDIUM", "MODERATE":
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
