// Package mergereport merges multiple intermediate JSON scan reports.
package mergereport

import (
	"encoding/json"
	"fmt"

	"github.com/osv/scanner/internal/domain/entity"
)

// reportJSON is the JSON representation of a scan report.
type reportJSON struct {
	Results []entity.ScanInfo `json:"results"`
}

// Input for MergeReport.
type Input struct {
	Reports [][]byte // JSON-encoded report bytes
}

// Output from MergeReport.
type Output struct {
	Results   []entity.ScanInfo
	TotalCVEs int
}

// UseCase merges multiple intermediate scan reports with deduplication.
type UseCase struct{}

// NewUseCase creates a new MergeReport use case.
func NewUseCase() *UseCase { return &UseCase{} }

// Execute merges reports and deduplicates results.
func (uc *UseCase) Execute(in Input) (*Output, error) {
	var all []entity.ScanInfo

	for i, raw := range in.Reports {
		var report reportJSON
		if err := json.Unmarshal(raw, &report); err != nil {
			return nil, fmt.Errorf("mergereport: report[%d]: %w", i, err)
		}
		all = append(all, report.Results...)
	}

	deduped := dedup(all)
	return &Output{
		Results:   deduped,
		TotalCVEs: len(deduped),
	}, nil
}

// ToJSON serializes the output as a merged report JSON.
func (o *Output) ToJSON() ([]byte, error) {
	return json.Marshal(reportJSON{Results: o.Results})
}

// dedup removes duplicate ScanInfo entries.
func dedup(in []entity.ScanInfo) []entity.ScanInfo {
	seen := make(map[string]struct{}, len(in))
	out := make([]entity.ScanInfo, 0, len(in))
	for _, s := range in {
		key := s.FilePath + "|" + s.Vendor + "|" + s.Product + "|" + s.Version
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
