package extractor

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ulikunitz_xz "github.com/ulikunitz/xz"
	"github.com/klauspost/compress/zstd"
)

// TarExtractor handles .tar, .tar.gz, .tgz, .tar.bz2, .tar.xz, .tar.zst
type TarExtractor struct{}

func (e *TarExtractor) Supports(filename string, header []byte) bool {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".tar"),
		strings.HasSuffix(lower, ".tar.gz"),
		strings.HasSuffix(lower, ".tgz"),
		strings.HasSuffix(lower, ".tar.bz2"),
		strings.HasSuffix(lower, ".tar.xz"),
		strings.HasSuffix(lower, ".tar.zst"):
		return true
	}
	// gzip magic
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return true
	}
	// bzip2 magic
	if len(header) >= 3 && header[0] == 0x42 && header[1] == 0x5a && header[2] == 0x68 {
		return true
	}
	// xz magic
	if len(header) >= 6 && header[0] == 0xfd && header[1] == 0x37 &&
		header[2] == 0x7a && header[3] == 0x58 && header[4] == 0x5a {
		return true
	}
	return false
}

func (e *TarExtractor) Extract(ctx context.Context, archivePath string, _ int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("tar: open %s: %w", archivePath, err)
	}
	defer f.Close() //nolint:errcheck

	inner, err := newDecompressor(f, archivePath)
	if err != nil {
		return nil, err
	}
	if rc, ok := inner.(io.Closer); ok {
		defer rc.Close() //nolint:errcheck
	}

	return readTarEntries(ctx, tar.NewReader(inner))
}

func newDecompressor(f *os.File, filename string) (io.Reader, error) {
	hdr := make([]byte, 8)
	n, _ := f.Read(hdr)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	hdr = hdr[:n]

	lower := strings.ToLower(filename)

	if (len(hdr) >= 2 && hdr[0] == 0x1f && hdr[1] == 0x8b) ||
		strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		return gr, nil
	}
	if (len(hdr) >= 2 && hdr[0] == 0x42 && hdr[1] == 0x5a) || strings.HasSuffix(lower, ".bz2") {
		return bzip2.NewReader(f), nil
	}
	if (len(hdr) >= 6 && hdr[0] == 0xfd && hdr[1] == 0x37) || strings.HasSuffix(lower, ".xz") {
		xr, err := ulikunitz_xz.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("xz: %w", err)
		}
		return xr, nil
	}
	if (len(hdr) >= 4 && hdr[0] == 0x28 && hdr[1] == 0xb5) || strings.HasSuffix(lower, ".zst") {
		zr, err := zstd.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("zstd: %w", err)
		}
		return zr, nil
	}
	return f, nil // plain tar
}

func readTarEntries(ctx context.Context, tr *tar.Reader) ([]FileEntry, error) {
	baseDir, _ := filepath.Abs(".")
	var entries []FileEntry
	for {
		if ctx.Err() != nil {
			return entries, ctx.Err()
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return entries, fmt.Errorf("tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if _, serr := safePath(baseDir, hdr.Name); serr != nil {
			continue
		}
		if hdr.Size > MaxFileSize {
			continue
		}
		content, err := limitedRead(tr, MaxFileSize)
		if err != nil {
			continue
		}
		entries = append(entries, FileEntry{
			Path:    filepath.Clean(hdr.Name),
			Content: content,
			Size:    int64(len(content)),
		})
	}
	return entries, nil
}

// ZipExtractor handles .zip archives.
type ZipExtractor struct{}

func (e *ZipExtractor) Supports(filename string, header []byte) bool {
	if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		return true
	}
	return len(header) >= 4 && header[0] == 0x50 && header[1] == 0x4b &&
		(header[2] == 0x03 || header[2] == 0x05)
}

func (e *ZipExtractor) Extract(ctx context.Context, archivePath string, _ int) ([]FileEntry, error) {
	info, err := os.Stat(archivePath)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	defer zr.Close() //nolint:errcheck

	baseDir, _ := filepath.Abs(".")
	_ = info
	var entries []FileEntry

	for _, f := range zr.File {
		if ctx.Err() != nil {
			return entries, ctx.Err()
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if _, serr := safePath(baseDir, f.Name); serr != nil {
			continue
		}
		if f.UncompressedSize64 > MaxFileSize {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, err := limitedRead(rc, MaxFileSize)
		rc.Close() //nolint:errcheck
		if err != nil {
			continue
		}
		entries = append(entries, FileEntry{
			Path:    filepath.Clean(f.Name),
			Content: content,
			Size:    int64(f.UncompressedSize64),
		})
	}
	return entries, nil
}

// GzipExtractor handles single .gz files.
type GzipExtractor struct{}

func (e *GzipExtractor) Supports(filename string, header []byte) bool {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".gz") && !strings.HasSuffix(lower, ".tar.gz") {
		return true
	}
	return len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b &&
		!strings.Contains(strings.ToLower(filename), "tar")
}

func (e *GzipExtractor) Extract(ctx context.Context, archivePath string, _ int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close() //nolint:errcheck

	outName := strings.TrimSuffix(filepath.Base(archivePath), ".gz")
	content, err := limitedRead(gr, MaxFileSize)
	if err != nil {
		return nil, err
	}
	return []FileEntry{{Path: outName, Content: content, Size: int64(len(content))}}, nil
}

// Bzip2Extractor handles single .bz2 files.
type Bzip2Extractor struct{}

func (e *Bzip2Extractor) Supports(filename string, header []byte) bool {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".bz2") && !strings.HasSuffix(lower, ".tar.bz2") {
		return true
	}
	return false
}

func (e *Bzip2Extractor) Extract(ctx context.Context, archivePath string, _ int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	br := bzip2.NewReader(f)
	outName := strings.TrimSuffix(filepath.Base(archivePath), ".bz2")
	content, err := limitedRead(br, MaxFileSize)
	if err != nil {
		return nil, err
	}
	return []FileEntry{{Path: outName, Content: content, Size: int64(len(content))}}, nil
}

// XzExtractor handles single .xz files.
type XzExtractor struct{}

func (e *XzExtractor) Supports(filename string, header []byte) bool {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".xz") && !strings.HasSuffix(lower, ".tar.xz") {
		return true
	}
	return false
}

func (e *XzExtractor) Extract(ctx context.Context, archivePath string, _ int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	xr, err := ulikunitz_xz.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("xz: %w", err)
	}
	outName := strings.TrimSuffix(filepath.Base(archivePath), ".xz")
	content, err := limitedRead(xr, MaxFileSize)
	if err != nil {
		return nil, err
	}
	return []FileEntry{{Path: outName, Content: content, Size: int64(len(content))}}, nil
}

// ZstdExtractor handles single .zst files.
type ZstdExtractor struct{}

func (e *ZstdExtractor) Supports(filename string, header []byte) bool {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".zst") && !strings.HasSuffix(lower, ".tar.zst") {
		return true
	}
	return len(header) >= 4 && header[0] == 0x28 && header[1] == 0xb5 &&
		header[2] == 0x2f && header[3] == 0xfd &&
		!strings.Contains(strings.ToLower(filename), "tar")
}

func (e *ZstdExtractor) Extract(ctx context.Context, archivePath string, _ int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("zstd: %w", err)
	}
	defer zr.Close()

	outName := strings.TrimSuffix(filepath.Base(archivePath), ".zst")
	content, err := limitedRead(zr, MaxFileSize)
	if err != nil {
		return nil, err
	}
	return []FileEntry{{Path: outName, Content: content, Size: int64(len(content))}}, nil
}
