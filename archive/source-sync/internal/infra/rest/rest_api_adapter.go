// Package rest implements RESTAPIAdapter for the Source Sync Service.
// Handles REST_ENDPOINT sources (e.g. Chainguard, PyPA) by fetching all.json and diffing.
package rest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/osv/source-sync/internal/application/port"
	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
	"github.com/rs/zerolog"
)

// RESTAPIAdapter implements SourceFetcher for REST_ENDPOINT sources.
type RESTAPIAdapter struct {
	http *http.Client
	log  zerolog.Logger
}

// NewRESTAPIAdapter creates a REST API change detector.
func NewRESTAPIAdapter(log zerolog.Logger) *RESTAPIAdapter {
	return &RESTAPIAdapter{
		http: &http.Client{Timeout: 120 * time.Second},
		log:  log,
	}
}

// CanHandle returns true for REST_ENDPOINT source type.
func (a *RESTAPIAdapter) CanHandle(source *source_repository.SourceRepository) bool {
	return source.SourceType() == "REST_ENDPOINT"
}

// DetectChanges fetches the all.json endpoint and returns all records as changes.
// REST sources are treated as full-refresh — all records are returned on every sync.
func (a *RESTAPIAdapter) DetectChanges(
	ctx context.Context,
	source *source_repository.SourceRepository,
	forceResync bool,
) (*port.ChangeSet, error) {
	url := source.RESTAPIURL()

	a.log.Info().Str("url", url).Msg("fetching REST source")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "OSV-SourceSync/2.0")
	req.Header.Set("Accept", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Compute overall hash for change detection
	overallHash := sha256hex(body)

	// If hash matches last sync and not force-resync → no changes
	if !forceResync && source.LastSyncedHash() == overallHash {
		a.log.Info().Str("url", url).Msg("no changes (hash match)")
		return &port.ChangeSet{
			Modified:    nil,
			TotalCount:  0,
			NewSyncHash: overallHash,
		}, nil
	}

	// Parse records from response
	// REST sources return either: [{...}, {...}] or {"vulns": [...]}
	records, err := parseRESTResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parse response from %s: %w", url, err)
	}

	ext := source.Extension()
	modified := make([]port.FileChange, 0, len(records))

	for i, record := range records {
		data, _ := json.Marshal(record)
		recordHash := sha256hex(data)

		// Derive a stable path from the vulnerability ID if present
		id := extractID(record)
		path := fmt.Sprintf("%s/%s%s", source.DirectoryPath(), id, ext)
		if id == "" {
			path = fmt.Sprintf("%s/record_%d%s", source.DirectoryPath(), i, ext)
		}

		modified = append(modified, port.FileChange{
			Path:        path,
			Hash:        recordHash,
			Content:  data, // Small enough to pass inline
		})
	}

	a.log.Info().Str("url", url).Int("records", len(modified)).Msg("REST sync complete")

	return &port.ChangeSet{
		Modified:    modified,
		TotalCount:  len(modified),
		NewSyncHash: overallHash,
	}, nil
}

func parseRESTResponse(data []byte) ([]map[string]interface{}, error) {
	// Try array format first
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}

	// Try object with "vulns" key
	var obj struct {
		Vulns []map[string]interface{} `json:"vulns"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && obj.Vulns != nil {
		return obj.Vulns, nil
	}

	return nil, fmt.Errorf("unrecognized response format")
}

func extractID(record map[string]interface{}) string {
	for _, key := range []string{"id", "ID", "vuln_id", "cve_id"} {
		if v, ok := record[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
