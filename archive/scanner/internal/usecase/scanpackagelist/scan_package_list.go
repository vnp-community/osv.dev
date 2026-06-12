// Package scanpackagelist parses a package manifest file to extract product/version info.
package scanpackagelist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/osv/scanner/internal/domain/entity"
	"github.com/osv/scanner/internal/parsers"
)

// Input for ScanPackageList.
type Input struct {
	FileBytes []byte
	FileName  string
}

// Output from ScanPackageList.
type Output struct {
	Products     []entity.ScanInfo
	DetectedType string // "go" | "pip" | "npm" | "cargo" | "maven" | "gem" | "conan"
}

// UseCase dispatches to the correct LanguageParser based on filename.
type UseCase struct{}

// NewUseCase creates a new ScanPackageList use case.
func NewUseCase() *UseCase { return &UseCase{} }

// Execute finds the right parser and extracts products.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	parser := parsers.FindParser(in.FileName)
	if parser == nil {
		return nil, fmt.Errorf("no parser for file: %s", in.FileName)
	}

	// Write to temp file
	tmpFile, err := writeTempFile(in.FileBytes, in.FileName)
	if err != nil {
		return nil, fmt.Errorf("scanpackagelist: temp file: %w", err)
	}
	defer os.Remove(tmpFile) //nolint:errcheck

	products, err := parser.Parse(ctx, tmpFile)
	if err != nil {
		return nil, fmt.Errorf("scanpackagelist: parse: %w", err)
	}

	infos := make([]entity.ScanInfo, 0, len(products))
	for _, p := range products {
		infos = append(infos, entity.ScanInfo{
			FilePath:    in.FileName,
			ProductInfo: p,
		})
	}

	return &Output{
		Products:     infos,
		DetectedType: detectEcosystem(in.FileName),
	}, nil
}

func detectEcosystem(filename string) string {
	base := filepath.Base(filename)
	switch base {
	case "go.mod", "go.sum":
		return "go"
	case "requirements.txt", "requirements-dev.txt", "Pipfile", "setup.cfg":
		return "pip"
	case "package.json", "package-lock.json":
		return "npm"
	case "Cargo.lock", "Cargo.toml":
		return "cargo"
	case "pom.xml", "build.gradle", "build.gradle.kts":
		return "maven"
	case "Gemfile.lock", "Gemfile":
		return "gem"
	case "conanfile.txt", "conanfile.py":
		return "conan"
	}
	return "unknown"
}

func writeTempFile(data []byte, filename string) (string, error) {
	ext := filepath.Ext(filename)
	f, err := os.CreateTemp("", "scanpkg-*"+ext)
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
