// Package fetcher — CVE.org GitHub Release fetcher.
// Port from TypeScript (globalcve/src/app/api/cves/route.ts).
// Source: https://github.com/CVEProject/cvelistV5/releases
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/globalcve/mono/internal/cvesearch/domain/entity"
	"github.com/globalcve/mono/internal/cvesync/domain/repository"
)

// CVEOrgFetcher fetches CVE data from CVE.org GitHub releases.
type CVEOrgFetcher struct {
	releaseURL string
	client     *http.Client
	cveRepo    repository.CVEWriteRepository
}

// NewCVEOrgFetcher creates a new CVE.org fetcher.
func NewCVEOrgFetcher(releaseURL string, timeout time.Duration, cveRepo repository.CVEWriteRepository) *CVEOrgFetcher {
	if releaseURL == "" {
		releaseURL = "https://github.com/CVEProject/cvelistV5/releases/latest/download/deltaLog.json"
	}
	return &CVEOrgFetcher{
		releaseURL: releaseURL,
		cveRepo:    cveRepo,
		client:     &http.Client{Timeout: timeout},
	}
}

func (f *CVEOrgFetcher) Source() entity.SourceName { return entity.SourceNameCVEOrg }

// cveOrgRecord is the CVE 5.0 schema record from CVE.org.
type cveOrgRecord struct {
	DataType    string `json:"dataType"`
	DataVersion string `json:"dataVersion"`
	CVEID       string `json:"cveMetadata"`
	Metadata    struct {
		CVEID       string `json:"cveId"`
		State       string `json:"state"`
		DatePublished string `json:"datePublished"`
		DateUpdated   string `json:"dateUpdated"`
	} `json:"cveMetadata"`
	Containers struct {
		CNA struct {
			Title       string `json:"title"`
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			Metrics []struct {
				CVSSv31 *struct {
					BaseScore    float64 `json:"baseScore"`
					VectorString string  `json:"vectorString"`
				} `json:"cvssV3_1"`
				CVSSv30 *struct {
					BaseScore    float64 `json:"baseScore"`
					VectorString string  `json:"vectorString"`
				} `json:"cvssV3_0"`
			} `json:"metrics"`
			References []struct {
				URL string `json:"url"`
			} `json:"references"`
			ProblemTypes []struct {
				Descriptions []struct {
					Lang        string `json:"lang"`
					Description string `json:"description"`
					CWEId       string `json:"cweId"`
				} `json:"descriptions"`
			} `json:"problemTypes"`
		} `json:"cna"`
	} `json:"containers"`
}

// deltaLogEntry represents one entry in the CVE.org delta log.
type deltaLogEntry struct {
	File      string `json:"file"`
	CVEID     string `json:"cveId"`
	EventType string `json:"eventType"` // "published", "modified", "deleted"
	Date      string `json:"date"`
}

// deltaLog is the CVE.org delta log JSON format.
type deltaLog struct {
	FetchTime string `json:"fetchTime"`
	New       []deltaLogEntry `json:"new"`
	Updated   []deltaLogEntry `json:"updated"`
}

// Fetch retrieves the delta log from CVE.org and fetches individual CVE records.
func (f *CVEOrgFetcher) Fetch(ctx context.Context, opts FetchOptions) (int, error) {
	delta, err := f.fetchDeltaLog(ctx)
	if err != nil {
		return 0, fmt.Errorf("cveorg delta log: %w", err)
	}

	// Process new and updated entries
	entries := append(delta.New, delta.Updated...)
	log.Ctx(ctx).Info().
		Int("new", len(delta.New)).
		Int("updated", len(delta.Updated)).
		Msg("cveorg: delta log fetched")

	cves := make([]*entity.CVE, 0, len(entries))
	for _, entry := range entries {
		if entry.EventType == "deleted" {
			continue
		}
		cve, err := f.fetchCVERecord(ctx, entry.CVEID)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Str("cve_id", entry.CVEID).Msg("cveorg: skip record")
			continue
		}
		if cve != nil {
			cves = append(cves, cve)
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
	}

	if len(cves) == 0 {
		return 0, nil
	}

	inserted, updated, err := f.cveRepo.UpsertBatch(ctx, cves)
	if err != nil {
		return 0, fmt.Errorf("cveorg upsert: %w", err)
	}
	return inserted + updated, nil
}

func (f *CVEOrgFetcher) fetchDeltaLog(ctx context.Context) (*deltaLog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.releaseURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get delta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("cveorg status %d: %s", resp.StatusCode, body)
	}

	var delta deltaLog
	if err := json.NewDecoder(resp.Body).Decode(&delta); err != nil {
		return nil, fmt.Errorf("decode delta log: %w", err)
	}
	return &delta, nil
}

// fetchCVERecord fetches a single CVE record from CVE.org GitHub raw.
func (f *CVEOrgFetcher) fetchCVERecord(ctx context.Context, cveID string) (*entity.CVE, error) {
	if !entity.IsValidID(cveID) {
		return nil, nil
	}

	// Build GitHub raw URL: CVE-YYYY-NNNN → year/NNNN/CVE-YYYY-NNNN.json
	parts := strings.Split(cveID, "-")
	if len(parts) < 3 {
		return nil, nil
	}
	year := parts[1]
	num := parts[2]
	// Pad to thousands: 12345 → 12xxx, 1234 → 1xxx
	prefix := num
	if len(num) >= 4 {
		prefix = num[:len(num)-3] + "xxx"
	}

	rawURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/%s/%s/%s.json",
		year, prefix, cveID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", cveID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // CVE not yet published
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cveorg %s status %d", cveID, resp.StatusCode)
	}

	var record cveOrgRecord
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, fmt.Errorf("decode %s: %w", cveID, err)
	}

	return f.convert(&record), nil
}

func (f *CVEOrgFetcher) convert(r *cveOrgRecord) *entity.CVE {
	cve := &entity.CVE{
		ID:       r.Metadata.CVEID,
		Source:   entity.SourceCVEOrg,
		Severity: entity.SeverityUnknown,
		Link:     "https://www.cve.org/CVERecord?id=" + r.Metadata.CVEID,
	}

	// Parse dates
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, r.Metadata.DatePublished); err == nil {
			cve.Published = t
			break
		}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, r.Metadata.DateUpdated); err == nil {
			cve.Modified = t
			break
		}
	}

	cve.Summary = r.Containers.CNA.Title

	// English description
	for _, d := range r.Containers.CNA.Descriptions {
		if d.Lang == "en" {
			cve.Description = d.Value
			break
		}
	}
	if cve.Description == "" {
		cve.Description = cve.Summary
	}

	// CVSS metrics
	for _, m := range r.Containers.CNA.Metrics {
		if m.CVSSv31 != nil && m.CVSSv31.BaseScore > 0 {
			score := m.CVSSv31.BaseScore
			cve.CVSS3Score = &score
			cve.CVSS3Vector = m.CVSSv31.VectorString
			cve.Severity = entity.SeverityFromCVSS3(score)
			break
		}
		if m.CVSSv30 != nil && m.CVSSv30.BaseScore > 0 {
			score := m.CVSSv30.BaseScore
			cve.CVSS3Score = &score
			cve.CVSS3Vector = m.CVSSv30.VectorString
			cve.Severity = entity.SeverityFromCVSS3(score)
			break
		}
	}

	// CWE IDs
	for _, pt := range r.Containers.CNA.ProblemTypes {
		for _, d := range pt.Descriptions {
			if d.CWEId != "" {
				cve.CWE = appendUniq(cve.CWE, d.CWEId)
			}
		}
	}

	// References
	for _, ref := range r.Containers.CNA.References {
		if ref.URL != "" {
			cve.References = append(cve.References, ref.URL)
		}
	}

	return cve
}
