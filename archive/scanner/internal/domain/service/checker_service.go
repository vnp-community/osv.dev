// Package service implements the CheckerService domain service.
package service

import (
	"strings"

	"github.com/osv/scanner/internal/domain/entity"
)

// CheckerService runs all registered checkers against file content.
type CheckerService struct {
	checkers []*entity.Checker
}

// NewCheckerService creates a CheckerService with the given checkers.
func NewCheckerService(checkers []*entity.Checker) *CheckerService {
	return &CheckerService{checkers: checkers}
}

// RunCheckers scans file content against all checkers.
// Returns ScanInfo entries for each detected vendor/product/version.
// A checker triggers if:
//  1. The filename matches a filename pattern, OR
//  2. The content matches a contains pattern (and no ignore pattern matches)
//
// If triggered, version is extracted from content.
// One ScanInfo is emitted per VendorProduct pair (if version found).
func (s *CheckerService) RunCheckers(filePath, content string) []entity.ScanInfo {
	var results []entity.ScanInfo

	for _, c := range s.checkers {
		filenameMatch := c.MatchFilename(filePath)
		contentMatch := c.MatchContent(content)

		if !filenameMatch && !contentMatch {
			continue
		}

		version := c.ExtractVersion(content)
		if version == "" {
			continue
		}

		for _, vp := range c.VendorProducts() {
			results = append(results, entity.ScanInfo{
				FilePath: filePath,
				ProductInfo: entity.ProductInfo{
					Vendor:  vp.Vendor,
					Product: vp.Product,
					Version: version,
					Source:  "checker",
				},
			})
		}
	}

	return results
}

// CheckerCount returns the number of loaded checkers.
func (s *CheckerService) CheckerCount() int { return len(s.checkers) }

// Checkers returns a copy of the loaded checkers slice.
func (s *CheckerService) Checkers() []*entity.Checker {
	result := make([]*entity.Checker, len(s.checkers))
	copy(result, s.checkers)
	return result
}

// FindByName returns the checker with the given name, or nil.
func (s *CheckerService) FindByName(name string) *entity.Checker {
	for _, c := range s.checkers {
		if strings.EqualFold(c.Name(), name) {
			return c
		}
	}
	return nil
}
