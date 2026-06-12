// Package redhat implements a DataSource for Red Hat security advisories (OVAL).
// Downloads OVAL XML data for RHEL 7/8/9 and extracts CVE-affected package ranges.
package redhat

import (
	"compress/bzip2"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/osv/ingestion-service/internal/adapter/external/sources"
)

const defaultBaseURL = "https://access.redhat.com/security/data/oval/v2"

// DefaultVersions lists the RHEL major versions to fetch.
var DefaultVersions = []int{7, 8, 9}

// RedHatSource implements sources.DataSource for Red Hat OVAL data.
type RedHatSource struct {
	baseURL    string
	httpClient *http.Client
	versions   []int
}

// New creates a new RedHatSource.
func New(baseURL string, versions []int, httpClient *http.Client) *RedHatSource {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if len(versions) == 0 {
		versions = DefaultVersions
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &RedHatSource{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		versions:   versions,
	}
}

// Name returns the source identifier.
func (s *RedHatSource) Name() string { return "REDHAT" }

// FetchCVEData downloads OVAL XML for each RHEL version and maps CVE data.
func (s *RedHatSource) FetchCVEData(ctx context.Context) (sources.CVEData, error) {
	data := sources.CVEData{Source: s.Name()}

	for _, ver := range s.versions {
		if err := ctx.Err(); err != nil {
			return data, err
		}
		sev, ranges, err := s.fetchVersion(ctx, ver)
		if err != nil {
			// Non-fatal: continue with other versions
			continue
		}
		data.Severities = append(data.Severities, sev...)
		data.Ranges = append(data.Ranges, ranges...)
	}

	return data, nil
}

func (s *RedHatSource) fetchVersion(ctx context.Context, ver int) ([]sources.CVESeverityRow, []sources.CVERangeRow, error) {
	// Red Hat OVAL URL format
	url := fmt.Sprintf("%s/RHEL%d/rhel-%d.oval.xml.bz2", s.baseURL, ver, ver)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("redhat: fetch RHEL%d: %w", ver, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("redhat: RHEL%d: status %d", ver, resp.StatusCode)
	}

	body := bzip2.NewReader(resp.Body)
	return parseOVAL(body, ver)
}

// ─── OVAL XML Structures ────────────────────────────────────────────────────

type ovalRoot struct {
	XMLName     xml.Name     `xml:"oval_definitions"`
	Definitions []ovalDef    `xml:"definitions>definition"`
}

type ovalDef struct {
	Class    string         `xml:"class,attr"`
	Metadata ovalMetadata   `xml:"metadata"`
	Criteria ovalCriteria   `xml:"criteria"`
}

type ovalMetadata struct {
	Title       string      `xml:"title"`
	Severity    string      `xml:"advisory>severity"`
	CVEs        []ovalCVE   `xml:"advisory>cve"`
}

type ovalCVE struct {
	ID string `xml:",chardata"`
}

type ovalCriteria struct {
	Criterions []ovalCriterion `xml:"criterion"`
	Criterias  []ovalCriteria  `xml:"criteria"`
}

type ovalCriterion struct {
	Comment string `xml:"comment,attr"`
}

// parseOVAL parses OVAL XML and extracts CVE severity and package version ranges.
func parseOVAL(r io.Reader, rhelVersion int) ([]sources.CVESeverityRow, []sources.CVERangeRow, error) {
	var root ovalRoot
	decoder := xml.NewDecoder(r)
	// XXE-safe: disable external entities
	decoder.Entity = map[string]string{}

	if err := decoder.Decode(&root); err != nil {
		return nil, nil, fmt.Errorf("redhat: xml decode: %w", err)
	}

	var severities []sources.CVESeverityRow
	var ranges []sources.CVERangeRow
	vendor := fmt.Sprintf("rhel%d", rhelVersion)

	for _, def := range root.Definitions {
		if def.Class != "patch" && def.Class != "vulnerability" {
			continue
		}

		for _, cveRef := range def.Metadata.CVEs {
			cveNum := strings.TrimSpace(cveRef.ID)
			if !strings.HasPrefix(cveNum, "CVE-") {
				continue
			}

			severity := normalizeSeverity(def.Metadata.Severity)

			severities = append(severities, sources.CVESeverityRow{
				CVENumber:   cveNum,
				Severity:    severity,
				Description: def.Metadata.Title,
				DataSource:  "REDHAT",
			})

			// Extract package names from criteria comments
			pkgs := extractPackages(def.Criteria)
			for _, pkg := range pkgs {
				ranges = append(ranges, sources.CVERangeRow{
					CVENumber:  cveNum,
					Vendor:     vendor,
					Product:    pkg,
					DataSource: "REDHAT",
				})
			}
		}
	}

	return severities, ranges, nil
}

func normalizeSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return "CRITICAL"
	case "IMPORTANT":
		return "HIGH"
	case "MODERATE":
		return "MEDIUM"
	case "LOW":
		return "LOW"
	default:
		return "NONE"
	}
}

// extractPackages collects package names from OVAL criterion comments.
// Comments look like: "httpd is earlier than 0:2.4.37-..." or "httpd is installed"
func extractPackages(criteria ovalCriteria) []string {
	seen := make(map[string]struct{})
	var pkgs []string

	var walk func(c ovalCriteria)
	walk = func(c ovalCriteria) {
		for _, cr := range c.Criterions {
			comment := cr.Comment
			// Extract package name: first word before " is "
			if idx := strings.Index(comment, " is "); idx > 0 {
				pkg := strings.ToLower(strings.TrimSpace(comment[:idx]))
				// Strip arch suffix (e.g. ".x86_64")
				if i := strings.LastIndex(pkg, "."); i > 0 {
					pkg = pkg[:i]
				}
				if pkg != "" {
					if _, ok := seen[pkg]; !ok {
						seen[pkg] = struct{}{}
						pkgs = append(pkgs, pkg)
					}
				}
			}
		}
		for _, sub := range c.Criterias {
			walk(sub)
		}
	}
	walk(criteria)
	return pkgs
}
