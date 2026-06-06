// Package extractor provides archive extraction with zip-slip prevention
// and depth limiting.
package extractor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultMaxDepth is the default max archive nesting depth.
	DefaultMaxDepth = 10
	// MaxFileSize is the maximum size of a single extracted file (100MB).
	MaxFileSize = 100 * 1024 * 1024
)

// FileEntry is a file extracted from an archive.
type FileEntry struct {
	Path    string // relative path within archive
	Content []byte
	Size    int64
}

// Extractor extracts files from an archive.
type Extractor interface {
	// Supports returns true if this extractor handles the given file.
	// filename is the archive filename; header is first 512 bytes of content.
	Supports(filename string, header []byte) bool
	// Extract returns all files in the archive up to maxDepth nesting.
	Extract(ctx context.Context, archivePath string, maxDepth int) ([]FileEntry, error)
}

// Dispatch returns the right extractor for a file, or (nil, false) if unknown.
func Dispatch(filename string, header []byte) (Extractor, bool) {
	extractors := []Extractor{
		&TarExtractor{},
		&ZipExtractor{},
		&GzipExtractor{},
		&Bzip2Extractor{},
		&XzExtractor{},
		&ZstdExtractor{},
		&RPMExtractor{},
		&DebExtractor{},
	}
	for _, e := range extractors {
		if e.Supports(filename, header) {
			return e, true
		}
	}
	return nil, false
}

// safePath validates that an archive entry path does not escape baseDir.
// Prevents zip-slip / path traversal attacks.
func safePath(baseDir, entryPath string) (string, error) {
	// Normalize: strip leading slash, resolve ..
	cleaned := filepath.Clean(entryPath)
	// Reject absolute paths and paths starting with ..
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("zip slip: absolute path %q", entryPath)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, `..\`) {
		return "", fmt.Errorf("zip slip detected: %q escapes base directory", entryPath)
	}
	joined := filepath.Join(baseDir, cleaned)
	// Final safety check: joined must be under baseDir
	if !strings.HasPrefix(joined, baseDir) {
		return "", fmt.Errorf("zip slip detected: %q escapes base directory", entryPath)
	}
	return joined, nil
}

// readFileHeader reads the first 512 bytes of a file for magic detection.
func readFileHeader(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return buf[:n], nil
	}
	return buf[:n], nil
}

// limitedRead reads at most maxBytes from reader.
func limitedRead(r io.Reader, maxBytes int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: maxBytes + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if lr.N == 0 {
		return nil, fmt.Errorf("file exceeds size limit (%d bytes)", maxBytes)
	}
	return data, nil
}
