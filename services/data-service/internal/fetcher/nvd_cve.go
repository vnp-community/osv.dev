// Package fetcher — NVD CVE API v2.0 fetcher.
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const nvdCVEBaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

// NVDCVEFetcher fetches CVE data from NIST NVD API v2.0.
type NVDCVEFetcher struct {
	client    *http.Client
	apiKey    string
	db        *mongo.Database
	startYear int
}

// NewNVDCVEFetcher creates a new NVD CVE fetcher.
func NewNVDCVEFetcher(db *mongo.Database, apiKey string, startYear int) *NVDCVEFetcher {
	if startYear == 0 {
		startYear = 2002
	}
	return &NVDCVEFetcher{
		client:    &http.Client{Timeout: 60 * time.Second},
		apiKey:    apiKey,
		db:        db,
		startYear: startYear,
	}
}

func (f *NVDCVEFetcher) Name() string { return string(SourceNVD) }

func (f *NVDCVEFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	col := f.db.Collection("cves")
	total := 0

	if opts.ManualDays > 0 {
		end := time.Now().UTC()
		start := end.AddDate(0, 0, -opts.ManualDays)
		return f.fetchPage(ctx, col, map[string]string{
			"lastModStartDate": start.Format(time.RFC3339),
			"lastModEndDate":   end.Format(time.RFC3339),
		})
	}

	// Full import: year by year
	currentYear := time.Now().Year()
	for year := f.startYear; year <= currentYear; year++ {
		if ctx.Err() != nil {
			break
		}
		log.Info().Int("year", year).Msg("NVD: fetching CVEs")
		count, err := f.fetchPage(ctx, col, map[string]string{
			"pubStartDate": fmt.Sprintf("%d-01-01T00:00:00.000", year),
			"pubEndDate":   fmt.Sprintf("%d-12-31T23:59:59.999", year),
		})
		if err != nil {
			log.Error().Err(err).Int("year", year).Msg("NVD fetch year failed, continuing")
			continue
		}
		total += count
	}
	return total, nil
}

func (f *NVDCVEFetcher) fetchPage(ctx context.Context, col *mongo.Collection, extra map[string]string) (int, error) {
	total := 0
	startIndex, pageSize := 0, 2000

	// Rate limit: 600ms with API key, 6500ms without
	delay := 6500 * time.Millisecond
	if f.apiKey != "" {
		delay = 600 * time.Millisecond
	}

	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}

		params := url.Values{
			"startIndex":     {strconv.Itoa(startIndex)},
			"resultsPerPage": {strconv.Itoa(pageSize)},
		}
		for k, v := range extra {
			params.Set(k, v)
		}

		apiURL := nvdCVEBaseURL + "?" + params.Encode()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if f.apiKey != "" {
			req.Header.Set("apiKey", f.apiKey)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return total, fmt.Errorf("NVD API: %w", err)
		}

		var result struct {
			TotalResults    int `json:"totalResults"`
			Vulnerabilities []struct {
				CVE json.RawMessage `json:"cve"`
			} `json:"vulnerabilities"`
		}
		json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
		resp.Body.Close()

		for _, item := range result.Vulnerabilities {
			doc := transformNVDCVE(item.CVE)
			if doc == nil {
				continue
			}
			id, _ := doc["id"].(string)
			if id == "" {
				continue
			}
			_, err := col.UpdateOne(ctx,
				bson.M{"id": id},
				bson.M{"$set": doc},
				mongoopts.Update().SetUpsert(true),
			)
			if err != nil {
				log.Error().Err(err).Str("cve_id", id).Msg("upsert CVE failed")
				continue
			}
			total++
		}

		startIndex += pageSize
		if startIndex >= result.TotalResults {
			break
		}
		time.Sleep(delay)
	}
	return total, nil
}

// transformNVDCVE converts NVD API v2.0 response to cve-search MongoDB document.
func transformNVDCVE(raw json.RawMessage) bson.M {
	var cveData struct {
		ID           string `json:"id"`
		Published    string `json:"published"`
		LastModified string `json:"lastModified"`
		VulnStatus   string `json:"vulnStatus"`
		Descriptions []struct {
			Lang  string `json:"lang"`
			Value string `json:"value"`
		} `json:"descriptions"`
		Metrics struct {
			CVSSMetricV31 []struct {
				CVSSData struct {
					BaseScore    float64 `json:"baseScore"`
					VectorString string  `json:"vectorString"`
				} `json:"cvssData"`
			} `json:"cvssMetricV31"`
			CVSSMetricV2 []struct {
				CVSSData struct {
					BaseScore    float64 `json:"baseScore"`
					VectorString string  `json:"vectorString"`
				} `json:"cvssData"`
			} `json:"cvssMetricV2"`
		} `json:"metrics"`
		Weaknesses []struct {
			Description []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"description"`
		} `json:"weaknesses"`
		Configurations []struct {
			Nodes []struct {
				CPEMatch []struct {
					Criteria string `json:"criteria"`
				} `json:"cpeMatch"`
			} `json:"nodes"`
		} `json:"configurations"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
	}

	if err := json.Unmarshal(raw, &cveData); err != nil {
		return nil
	}

	doc := bson.M{
		"id":     cveData.ID,
		"status": cveData.VulnStatus,
	}

	if t, err := time.Parse(time.RFC3339, cveData.Published); err == nil {
		doc["published"] = t
	}
	if t, err := time.Parse(time.RFC3339, cveData.LastModified); err == nil {
		doc["modified"] = t
	}

	for _, d := range cveData.Descriptions {
		if d.Lang == "en" {
			doc["summary"] = d.Value
			break
		}
	}

	if len(cveData.Metrics.CVSSMetricV31) > 0 {
		m := cveData.Metrics.CVSSMetricV31[0].CVSSData
		doc["cvss3"] = m.BaseScore
		doc["cvss3Vector"] = m.VectorString
	}
	if len(cveData.Metrics.CVSSMetricV2) > 0 {
		m := cveData.Metrics.CVSSMetricV2[0].CVSSData
		doc["cvss"] = m.BaseScore
		doc["cvssVector"] = m.VectorString
	}

	var cwes []string
	for _, w := range cveData.Weaknesses {
		for _, d := range w.Description {
			if d.Lang == "en" && d.Value != "NVD-CWE-Other" && d.Value != "NVD-CWE-noinfo" {
				cwes = append(cwes, d.Value)
			}
		}
	}
	if len(cwes) > 0 {
		doc["cwe"] = cwes
	}

	var cpes, vendors, products []string
	seen := make(map[string]bool)
	for _, cfg := range cveData.Configurations {
		for _, node := range cfg.Nodes {
			for _, match := range node.CPEMatch {
				cpe := match.Criteria
				if seen[cpe] {
					continue
				}
				seen[cpe] = true
				cpes = append(cpes, cpe)
				parts := strings.Split(cpe, ":")
				if len(parts) >= 5 {
					vendor, product := parts[3], parts[4]
					if vendor != "*" && vendor != "-" {
						vendors = append(vendors, vendor)
					}
					if product != "*" && product != "-" {
						products = append(products, product)
					}
				}
			}
		}
	}
	if len(cpes) > 0 {
		doc["vulnerable_configuration"] = cpes
	}
	if len(vendors) > 0 {
		doc["vendors"] = uniqueStrings(vendors)
	}
	if len(products) > 0 {
		doc["products"] = uniqueStrings(products)
	}

	var refs []string
	for _, r := range cveData.References {
		refs = append(refs, r.URL)
	}
	if len(refs) > 0 {
		doc["references"] = refs
	}

	return doc
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
