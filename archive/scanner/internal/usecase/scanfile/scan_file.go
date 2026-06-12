// Package scanfile implements the ScanFile use case.
package scanfile

import (
	"context"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/osv/scanner/internal/domain/entity"
	"github.com/osv/scanner/internal/domain/service"
	"github.com/osv/scanner/internal/infrastructure/extractor"
	"github.com/osv/scanner/internal/infrastructure/filedetect"
	"github.com/osv/scanner/internal/parsers"
)

// Input contains the parameters for scanning a single file.
type Input struct {
	FileBytes    []byte
	FileName     string
	Extract      bool     // auto-extract archives recursively
	SkipCheckers []string // checker names to skip
	RunCheckers  []string // if non-empty, only these checkers run
	MaxDepth     int      // max extraction depth (default: 10)
}

// Output contains the scan results.
type Output struct {
	Results      []entity.ScanInfo
	ScannedFiles int
}

// UseCase orchestrates file type detection, extraction, parsing, and checker execution.
type UseCase struct {
	checkerSvc *service.CheckerService
	extractors []extractor.Extractor
	parsers    []parsers.LanguageParser
}

// NewUseCase creates a new ScanFile use case.
func NewUseCase(
	checkerSvc *service.CheckerService,
	exts []extractor.Extractor,
	pars []parsers.LanguageParser,
) *UseCase {
	return &UseCase{
		checkerSvc: checkerSvc,
		extractors: exts,
		parsers:    pars,
	}
}

// Execute runs the ScanFile use case.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	maxDepth := in.MaxDepth
	if maxDepth <= 0 {
		maxDepth = extractor.DefaultMaxDepth
	}

	var allResults []entity.ScanInfo
	scannedFiles := 0

	if err := uc.scanBytes(ctx, in.FileBytes, in.FileName, in.Extract, maxDepth, in.SkipCheckers, in.RunCheckers, &allResults, &scannedFiles); err != nil {
		return nil, err
	}

	return &Output{
		Results:      dedup(allResults),
		ScannedFiles: scannedFiles,
	}, nil
}

// scanBytes is the recursive implementation that scans raw bytes.
func (uc *UseCase) scanBytes(
	ctx context.Context,
	data []byte,
	filename string,
	doExtract bool,
	depth int,
	skipCheckers, runCheckers []string,
	results *[]entity.ScanInfo,
	scannedFiles *int,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	*scannedFiles++

	// Detect file type from magic bytes + extension
	header := data
	if len(header) > 512 {
		header = header[:512]
	}
	ft := filedetect.DetectType(header, filename)

	// 1. Check if it's a language manifest file
	if parser := parsers.FindParser(filename); parser != nil {
		// Write to temp file for the parser
		tmpFile, err := writeTempFile(data, filename)
		if err != nil {
			return err
		}
		defer os.Remove(tmpFile) //nolint:errcheck

		products, err := parser.Parse(ctx, tmpFile)
		if err == nil {
			for _, p := range products {
				*results = append(*results, entity.ScanInfo{
					FilePath:    filename,
					ProductInfo: p,
				})
			}
		}
		return nil
	}

	// 2. If archive and extraction enabled
	if doExtract && filedetect.IsArchive(ft) && depth > 0 {
		tmpFile, err := writeTempFile(data, filename)
		if err != nil {
			return err
		}
		defer os.Remove(tmpFile) //nolint:errcheck

		// Find extractor
		ext, ok := extractor.Dispatch(filename, header)
		if ok {
			entries, err := ext.Extract(ctx, tmpFile, depth-1)
			if err == nil {
				for _, entry := range entries {
					if err := uc.scanBytes(ctx, entry.Content, entry.Path, doExtract, depth-1, skipCheckers, runCheckers, results, scannedFiles); err != nil {
						if ctx.Err() != nil {
							return err
						}
						// continue on non-fatal errors
					}
				}
				return nil
			}
		}
	}

	// 3. Run checkers on content (binary or text)
	content := toStringLossy(data)
	filtered := uc.filterCheckers(skipCheckers, runCheckers)
	scanInfos := filtered.RunCheckers(filename, content)
	*results = append(*results, scanInfos...)

	return nil
}

// filterCheckers returns a service view that respects skip/run filters.
func (uc *UseCase) filterCheckers(skip, run []string) *service.CheckerService {
	if len(skip) == 0 && len(run) == 0 {
		return uc.checkerSvc
	}

	allCheckers := uc.checkerSvc.Checkers()
	skipSet := toSet(skip)
	runSet := toSet(run)

	var filtered []*entity.Checker
	for _, c := range allCheckers {
		name := c.Name()
		if len(skipSet) > 0 && skipSet[name] {
			continue
		}
		if len(runSet) > 0 && !runSet[name] {
			continue
		}
		filtered = append(filtered, c)
	}
	return service.NewCheckerService(filtered)
}

// writeTempFile writes bytes to a temp file with appropriate extension.
func writeTempFile(data []byte, filename string) (string, error) {
	// Try to preserve filename suffix for better detection
	suffix := ""
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		suffix = filename[idx:]
		if len(suffix) > 20 {
			suffix = ""
		}
	}

	f, err := os.CreateTemp("", "scanner-*"+suffix)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name()) //nolint:errcheck
		return "", err
	}
	return f.Name(), nil
}

// toStringLossy converts bytes to string, replacing invalid UTF-8 with replacement char.
func toStringLossy(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		sb.WriteRune(r)
		b = b[size:]
	}
	return sb.String()
}

// dedup removes duplicate ScanInfo entries (same vendor+product+version+filepath).
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

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[strings.ToLower(s)] = true
	}
	return m
}
