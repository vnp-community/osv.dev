// Package asset — crud.go
// SEED-005-B: AssetCRUDUseCase provides Create, BulkCreate, Delete, AddVulnerabilities.
package asset

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/google/osv.dev/services/asset-service/internal/domain/entity"
)

// CRUDRepository extends AssetRepository with bulk and vulnerability ops.
type CRUDRepository interface {
	AssetRepository
	Create(ctx context.Context, asset *entity.Asset) error
	CreateBulk(ctx context.Context, assets []*entity.Asset, updateExisting bool) ([]entity.BulkAssetResult, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error
	ListVulnerabilities(ctx context.Context, assetID uuid.UUID) ([]entity.Vulnerability, error)
}

// EventPublisher publishes asset domain events.
type EventPublisher interface {
	Publish(subject string, payload map[string]any) error
}

// AssetCRUDUseCase provides SEED-005 create/bulk/delete/vuln operations.
type AssetCRUDUseCase struct {
	repo     CRUDRepository
	eventPub EventPublisher
}

// NewAssetCRUDUseCase creates a new AssetCRUDUseCase.
func NewAssetCRUDUseCase(repo CRUDRepository, pub EventPublisher) *AssetCRUDUseCase {
	return &AssetCRUDUseCase{repo: repo, eventPub: pub}
}

// ImportResult is the summary of an asset import operation.
type ImportResult struct {
	CreatedCount int                    `json:"created_count"`
	UpdatedCount int                    `json:"updated_count"`
	SkippedCount int                    `json:"skipped_count"`
	FailedCount  int                    `json:"failed_count"`
	Results      []entity.BulkAssetResult `json:"results"`
}

// ── CRUD Operations ───────────────────────────────────────────────────────────

// Create creates a single asset. Returns 409-like error if IP already exists.
func (uc *AssetCRUDUseCase) Create(ctx context.Context, in entity.AssetCreateInput) (*entity.Asset, error) {
	if err := validateIPAddress(in.IPAddress); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	a := &entity.Asset{
		ID:         uuid.New(),
		IPAddress:  in.IPAddress,
		Hostname:   in.Hostname,
		OS:         in.OS,
		MACAddress: in.MACAddress,
		Services:   in.Services,
		Tags:       defaultStrings(in.Tags),
		Labels:     in.Labels,
		Status:     entity.AssetStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := uc.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	if uc.eventPub != nil {
		_ = uc.eventPub.Publish("asset.created", map[string]any{
			"asset_id": a.ID, "ip": a.IPAddress,
		})
	}
	return a, nil
}

// BulkCreate creates multiple assets. Returns per-item results.
func (uc *AssetCRUDUseCase) BulkCreate(ctx context.Context, inputs []entity.AssetCreateInput, updateExisting bool) ([]entity.BulkAssetResult, error) {
	if len(inputs) > 500 {
		return nil, fmt.Errorf("bulk limit exceeded: max 500 assets per request")
	}

	assets := make([]*entity.Asset, 0, len(inputs))
	for _, in := range inputs {
		if err := validateIPAddress(in.IPAddress); err != nil {
			// Return error result for invalid IPs without aborting
			assets = append(assets, nil) // placeholder
		} else {
			now := time.Now().UTC()
			assets = append(assets, &entity.Asset{
				ID:         uuid.New(),
				IPAddress:  in.IPAddress,
				Hostname:   in.Hostname,
				OS:         in.OS,
				MACAddress: in.MACAddress,
				Services:   in.Services,
				Tags:       defaultStrings(in.Tags),
				Labels:     in.Labels,
				Status:     entity.AssetStatusActive,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}

	// Build valid asset slice (filter out nil placeholders with error)
	var validAssets []*entity.Asset
	var invalidResults []entity.BulkAssetResult
	for i, a := range assets {
		if a == nil {
			invalidResults = append(invalidResults, entity.BulkAssetResult{
				IPAddress: inputs[i].IPAddress,
				Status:    "error",
				Message:   "invalid IP address",
			})
		} else {
			validAssets = append(validAssets, a)
		}
	}

	results, err := uc.repo.CreateBulk(ctx, validAssets, updateExisting)
	if err != nil {
		return nil, err
	}

	// Append invalid results at the end
	if len(invalidResults) > 0 {
		results = append(results, invalidResults...)
	}

	if uc.eventPub != nil {
		created := 0
		for _, r := range results {
			if r.Status == "created" || r.Status == "updated" {
				created++
			}
		}
		_ = uc.eventPub.Publish("asset.batch_created", map[string]any{"count": created})
	}
	return results, nil
}

// Delete removes an asset by ID.
func (uc *AssetCRUDUseCase) Delete(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}

// Get retrieves a single asset by ID. Returns the asset or an error if not found.
// FIX BUG-H2-001: exposes repo.FindByID through use case layer.
func (uc *AssetCRUDUseCase) Get(ctx context.Context, id uuid.UUID) (*entity.Asset, error) {
	return uc.repo.FindByID(ctx, id)
}

// AddVulnerabilities injects CVE vulnerabilities for an asset and updates finding_count.
func (uc *AssetCRUDUseCase) AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error {
	return uc.repo.AddVulnerabilities(ctx, assetID, vulns)
}

// ImportFromJSON parses a JSON array of AssetCreateInput and calls BulkCreate.
func (uc *AssetCRUDUseCase) ImportFromJSON(ctx context.Context, r io.Reader, updateExisting bool) (*ImportResult, error) {
	var inputs []entity.AssetCreateInput
	if err := json.NewDecoder(r).Decode(&inputs); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return uc.bulkAndSummarize(ctx, inputs, updateExisting)
}

// ImportFromCSV parses CSV rows into AssetCreateInput and calls BulkCreate.
// Expected header: ip_address,hostname,os,mac_address,tags
func (uc *AssetCRUDUseCase) ImportFromCSV(ctx context.Context, r io.Reader, updateExisting bool) (*ImportResult, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	headers, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	col := func(row []string, name string) string {
		if i, ok := idx[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	var inputs []entity.AssetCreateInput
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse CSV row: %w", err)
		}
		var tags []string
		if t := col(row, "tags"); t != "" {
			for _, tag := range strings.Split(t, ";") {
				if tag = strings.TrimSpace(tag); tag != "" {
					tags = append(tags, tag)
				}
			}
		}
		inputs = append(inputs, entity.AssetCreateInput{
			IPAddress:  col(row, "ip_address"),
			Hostname:   col(row, "hostname"),
			OS:         col(row, "os"),
			MACAddress: col(row, "mac_address"),
			Tags:       tags,
		})
	}
	return uc.bulkAndSummarize(ctx, inputs, updateExisting)
}

func (uc *AssetCRUDUseCase) bulkAndSummarize(ctx context.Context, inputs []entity.AssetCreateInput, updateExisting bool) (*ImportResult, error) {
	results, err := uc.BulkCreate(ctx, inputs, updateExisting)
	if err != nil {
		return nil, err
	}
	out := &ImportResult{Results: results}
	for _, r := range results {
		switch r.Status {
		case "created":
			out.CreatedCount++
		case "updated":
			out.UpdatedCount++
		case "skipped":
			out.SkippedCount++
		case "error":
			out.FailedCount++
		}
	}
	return out, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func validateIPAddress(ip string) error {
	if ip == "" {
		return fmt.Errorf("ip_address is required")
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid ip_address: %s", ip)
	}
	return nil
}

func defaultStrings(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}
