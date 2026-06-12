// Package fetcher — MITRE CWE XML fetcher.
// Downloads CWE weakness catalog from MITRE and stores in MongoDB "cwe" collection.
package fetcher

import (
	"archive/zip"
	"bytes"
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

const (
	mitreDefaultCWEURL   = "https://cwe.mitre.org/data/xml/cwec_latest.xml.zip"
)

// MITRECWEFetcher downloads and stores CWE data from MITRE.
type MITRECWEFetcher struct {
	db     *mongo.Database
	url    string
	client *http.Client
}

// NewMITRECWEFetcher creates a CWE fetcher.
func NewMITRECWEFetcher(db *mongo.Database) *MITRECWEFetcher {
	return &MITRECWEFetcher{
		db:     db,
		url:    mitreDefaultCWEURL,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (f *MITRECWEFetcher) Name() string { return "cwe" }

// FetchAndStore downloads CWE XML catalog and upserts into MongoDB "cwe" collection.
func (f *MITRECWEFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	log.Info().Str("url", f.url).Msg("Fetching MITRE CWE catalog")

	xmlData, err := f.download(ctx)
	if err != nil {
		return 0, fmt.Errorf("download CWE catalog: %w", err)
	}

	weaknesses, err := parseCWEXML(xmlData)
	if err != nil {
		return 0, fmt.Errorf("parse CWE XML: %w", err)
	}

	col := f.db.Collection("cwe")
	count := 0
	for _, w := range weaknesses {
		doc := bson.M{
			"id":          w.ID,
			"name":        w.Name,
			"description": w.Description,
			"status":      w.Status,
			"capec":       w.CAPECIDs,
		}
		_, err := col.UpdateOne(ctx,
			bson.M{"id": w.ID},
			bson.M{"$set": doc},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Warn().Err(err).Str("id", w.ID).Msg("upsert CWE failed")
			continue
		}
		count++
	}

	log.Info().Int("count", count).Msg("MITRE CWE fetch complete")
	return count, nil
}

func (f *MITRECWEFetcher) download(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MITRE CWE returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// The file is ZIP-compressed — extract first .xml file
	if strings.HasSuffix(f.url, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			// Assume raw XML if not a valid ZIP
			return body, nil
		}
		for _, zf := range zr.File {
			if strings.HasSuffix(zf.Name, ".xml") {
				rc, err := zf.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
	}
	return body, nil
}

// cweXMLWeakness is a minimal representation of a CWE XML entry.
type cweXMLWeakness struct {
	ID          string
	Name        string
	Description string
	Status      string
	CAPECIDs    []string
}

// parseCWEXML parses the MITRE CWE XML catalog.
func parseCWEXML(data []byte) ([]cweXMLWeakness, error) {
	type xmlRelatedAttackPattern struct {
		CAPECID string `xml:"CAPEC_ID,attr"`
	}
	type xmlWeakness struct {
		ID                    string                    `xml:"ID,attr"`
		Name                  string                    `xml:"Name,attr"`
		Status                string                    `xml:"Status,attr"`
		Description           string                    `xml:"Description>Description_Summary"`
		RelatedAttackPatterns []xmlRelatedAttackPattern `xml:"Related_Attack_Patterns>Related_Attack_Pattern"`
	}
	type xmlCatalog struct {
		Weaknesses []xmlWeakness `xml:"Weaknesses>Weakness"`
	}

	var catalog xmlCatalog
	if err := xml.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("unmarshal CWE XML: %w", err)
	}

	result := make([]cweXMLWeakness, 0, len(catalog.Weaknesses))
	for _, w := range catalog.Weaknesses {
		capecIDs := make([]string, 0, len(w.RelatedAttackPatterns))
		for _, rap := range w.RelatedAttackPatterns {
			if rap.CAPECID != "" {
				capecIDs = append(capecIDs, rap.CAPECID)
			}
		}
		result = append(result, cweXMLWeakness{
			ID:          w.ID,
			Name:        w.Name,
			Description: strings.TrimSpace(w.Description),
			Status:      w.Status,
			CAPECIDs:    capecIDs,
		})
	}
	return result, nil
}
