package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
)

// SearchWithAggregations performs a search and optionally returns aggregations.
// This is ADDITIVE — it does NOT modify the existing SearchCVEs method.
func (c *LegacyClient) SearchWithAggregations(ctx context.Context, req *SearchCVEsRequest, includeAggs bool) (map[string]interface{}, error) {
	query := c.buildQuery(req)

	if includeAggs {
		query["aggs"] = map[string]interface{}{
			"by_severity": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "severity",
					"size":  10,
				},
			},
			"top_vendors": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "vendor_name",
					"size":  10,
				},
			},
			"by_year": map[string]interface{}{
				"date_histogram": map[string]interface{}{
					"field":             "published_at",
					"calendar_interval": "year",
					"format":            "yyyy",
				},
			},
		}
	}

	body, _ := json.Marshal(query)
	from := req.Page * req.Limit

	res, err := c.os.Search(
		c.os.Search.WithContext(ctx),
		c.os.Search.WithIndex(c.index),
		c.os.Search.WithBody(bytes.NewReader(body)),
		c.os.Search.WithFrom(from),
		c.os.Search.WithSize(req.Limit),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var raw map[string]interface{}
	json.NewDecoder(res.Body).Decode(&raw) //nolint:errcheck
	return raw, nil
}

// ParseSearchAggregations extracts and maps aggregation results from a raw OpenSearch response.
// Returns nil if no aggregations are present.
func ParseSearchAggregations(osResp map[string]interface{}) *SearchAggregations {
	aggs, ok := osResp["aggregations"].(map[string]interface{})
	if !ok {
		return nil
	}

	bySeverity := make(map[string]int)
	if sev, ok := aggs["by_severity"].(map[string]interface{}); ok {
		if buckets, ok := sev["buckets"].([]interface{}); ok {
			for _, b := range buckets {
				bucket := b.(map[string]interface{})
				bySeverity[bucket["key"].(string)] = int(bucket["doc_count"].(float64))
			}
		}
	}

	var topVendors []VendorAggCount
	if v, ok := aggs["top_vendors"].(map[string]interface{}); ok {
		if buckets, ok := v["buckets"].([]interface{}); ok {
			for _, b := range buckets {
				bucket := b.(map[string]interface{})
				topVendors = append(topVendors, VendorAggCount{
					Vendor: bucket["key"].(string),
					Count:  int(bucket["doc_count"].(float64)),
				})
			}
		}
	}

	var byYear []YearAggCount
	if y, ok := aggs["by_year"].(map[string]interface{}); ok {
		if buckets, ok := y["buckets"].([]interface{}); ok {
			for _, b := range buckets {
				bucket := b.(map[string]interface{})
				byYear = append(byYear, YearAggCount{
					Year:  bucket["key_as_string"].(string),
					Count: int(bucket["doc_count"].(float64)),
				})
			}
		}
	}

	return &SearchAggregations{
		BySeverity: bySeverity,
		TopVendors: topVendors,
		ByYear:     byYear,
	}
}

// SearchAggregations holds faceted aggregation results.
type SearchAggregations struct {
	BySeverity map[string]int `json:"by_severity"`
	TopVendors []VendorAggCount `json:"top_vendors"`
	ByYear     []YearAggCount   `json:"by_year"`
}

// VendorAggCount is a vendor with a CVE count.
type VendorAggCount struct {
	Vendor string `json:"vendor"`
	Count  int    `json:"count"`
}

// YearAggCount is a year with a CVE count.
type YearAggCount struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}
