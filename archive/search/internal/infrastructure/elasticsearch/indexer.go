// Package elasticsearch provides the Elasticsearch indexer for the Search service.
package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
)

// FindingDocument is the Elasticsearch document for a DefectDojo finding.
type FindingDocument struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Severity         string   `json:"severity"`
	CVE              string   `json:"cve"`
	CWE              int      `json:"cwe"`
	Status           string   `json:"status"` // active | mitigated | false_positive | etc.
	Active           bool     `json:"active"`
	ProductID        string   `json:"product_id"`
	EngagementID     string   `json:"engagement_id"`
	ComponentName    string   `json:"component_name"`
	ComponentVersion string   `json:"component_version"`
	FilePath         string   `json:"file_path"`
	HashCode         string   `json:"hash_code"`
	Tags             []string `json:"tags"`
	UpdatedAt        string   `json:"updated_at"`
}

// Indexer manages Elasticsearch document indexing and searching.
type Indexer struct {
	client *elasticsearch.Client
}

// New creates an Indexer connecting to the given Elasticsearch address.
func New(address string) (*Indexer, error) {
	cfg := elasticsearch.Config{Addresses: []string{address}}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: new client: %w", err)
	}
	return &Indexer{client: client}, nil
}

// IndexFinding creates or updates a single finding document.
func (i *Indexer) IndexFinding(ctx context.Context, doc *FindingDocument) error {
	body, _ := json.Marshal(doc)
	resp, err := i.client.Index("findings",
		bytes.NewReader(body),
		i.client.Index.WithDocumentID(doc.ID),
		i.client.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("index finding: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("index finding: ES error: %s", resp.Status())
	}
	return nil
}

// BulkIndexFindings indexes multiple findings using the ES bulk API.
func (i *Indexer) BulkIndexFindings(ctx context.Context, docs []*FindingDocument) error {
	if len(docs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, doc := range docs {
		meta, _ := json.Marshal(map[string]interface{}{
			"index": map[string]string{"_id": doc.ID},
		})
		body, _ := json.Marshal(doc)
		buf.Write(meta)
		buf.WriteByte('\n')
		buf.Write(body)
		buf.WriteByte('\n')
	}

	resp, err := i.client.Bulk(bytes.NewReader(buf.Bytes()),
		i.client.Bulk.WithIndex("findings"),
		i.client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("bulk index: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("bulk index: ES error: %s", resp.Status())
	}
	return nil
}

// SearchQuery represents a full-text + filter search request.
type SearchQuery struct {
	Query    string
	Type     string // "finding" | "product" | "engagement" (empty = all)
	Severity string
	UserID   string
	Limit    int
	Offset   int
}

// SearchResult holds a list of matching documents.
type SearchResult struct {
	Total int64
	Hits  []json.RawMessage
}

// Search performs a multi-match full-text query with optional severity filter.
func (i *Indexer) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	index := "findings"
	if q.Type != "" && q.Type != "finding" {
		index = q.Type + "s"
	}

	if q.Limit <= 0 {
		q.Limit = 25
	}

	must := []interface{}{
		map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  q.Query,
				"fields": []string{"title^3", "description", "cve^2", "component_name"},
			},
		},
	}
	if q.Severity != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]string{"severity": q.Severity},
		})
	}

	esQuery, _ := json.Marshal(map[string]interface{}{
		"from": q.Offset,
		"size": q.Limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"must": must},
		},
		"sort": []map[string]interface{}{
			{"_score": "desc"},
			{"updated_at": "desc"},
		},
	})

	resp, err := i.client.Search(
		i.client.Search.WithIndex(index),
		i.client.Search.WithBody(bytes.NewReader(esQuery)),
		i.client.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Hits struct {
			Total struct{ Value int64 } `json:"total"`
			Hits  []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	hits := make([]json.RawMessage, 0, len(result.Hits.Hits))
	for _, h := range result.Hits.Hits {
		hits = append(hits, h.Source)
	}
	return &SearchResult{Total: result.Hits.Total.Value, Hits: hits}, nil
}
