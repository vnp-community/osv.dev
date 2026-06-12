// Package fetcher — MITRE CAPEC XML fetcher.
// Downloads CAPEC (Common Attack Pattern Enumeration and Classification) from MITRE.
package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const mitreCAPECURL = "https://capec.mitre.org/data/xml/capec_latest.xml"

// MITRECAPECFetcher downloads and stores CAPEC data from MITRE.
type MITRECAPECFetcher struct {
	db     *mongo.Database
	url    string
	client *http.Client
}

// NewMITRECAPECFetcher creates a CAPEC fetcher.
func NewMITRECAPECFetcher(db *mongo.Database) *MITRECAPECFetcher {
	return &MITRECAPECFetcher{
		db:     db,
		url:    mitreCAPECURL,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (f *MITRECAPECFetcher) Name() string { return "capec" }

// FetchAndStore downloads CAPEC XML and upserts into MongoDB "capec" collection.
func (f *MITRECAPECFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	log.Info().Str("url", f.url).Msg("Fetching MITRE CAPEC catalog")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download CAPEC: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("MITRE CAPEC returned %d", resp.StatusCode)
	}

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read CAPEC body: %w", err)
	}

	patterns, err := parseCAPECXML(xmlData)
	if err != nil {
		return 0, fmt.Errorf("parse CAPEC XML: %w", err)
	}

	col := f.db.Collection("capec")
	count := 0
	for _, p := range patterns {
		doc := bson.M{
			"id":               p.ID,
			"name":             p.Name,
			"summary":          p.Summary,
			"severity":         p.Severity,
			"related_weakness": p.RelatedCWEs,
		}
		_, err := col.UpdateOne(ctx,
			bson.M{"id": p.ID},
			bson.M{"$set": doc},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Warn().Err(err).Str("id", p.ID).Msg("upsert CAPEC failed")
			continue
		}
		count++
	}

	log.Info().Int("count", count).Msg("MITRE CAPEC fetch complete")
	return count, nil
}

// capecPattern is a minimal CAPEC pattern representation.
type capecPattern struct {
	ID          string
	Name        string
	Summary     string
	Severity    string
	RelatedCWEs []string
}

// parseCAPECXML parses the MITRE CAPEC XML catalog.
func parseCAPECXML(data []byte) ([]capecPattern, error) {
	type xmlRelatedWeakness struct {
		CWEID string `xml:"CWE_ID,attr"`
	}
	type xmlAttackPattern struct {
		ID             string               `xml:"ID,attr"`
		Name           string               `xml:"Name,attr"`
		Status         string               `xml:"Status,attr"`
		Severity       string               `xml:"Typical_Severity"`
		Description    string               `xml:"Description>Summary"`
		RelatedWeakness []xmlRelatedWeakness `xml:"Related_Weaknesses>Related_Weakness"`
	}
	type xmlCatalog struct {
		AttackPatterns []xmlAttackPattern `xml:"Attack_Patterns>Attack_Pattern"`
	}

	var catalog xmlCatalog
	if err := xml.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("unmarshal CAPEC XML: %w", err)
	}

	result := make([]capecPattern, 0, len(catalog.AttackPatterns))
	for _, p := range catalog.AttackPatterns {
		// Skip deprecated/historical
		if p.Status == "Deprecated" || p.Status == "Obsolete" {
			continue
		}

		cwes := make([]string, 0, len(p.RelatedWeakness))
		for _, rw := range p.RelatedWeakness {
			if rw.CWEID != "" {
				cwes = append(cwes, rw.CWEID)
			}
		}

		result = append(result, capecPattern{
			ID:          p.ID,
			Name:        p.Name,
			Summary:     strings.TrimSpace(p.Description),
			Severity:    strings.TrimSpace(p.Severity),
			RelatedCWEs: cwes,
		})
	}
	return result, nil
}
