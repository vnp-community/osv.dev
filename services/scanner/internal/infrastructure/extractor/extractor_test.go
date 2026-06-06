package extractor_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/osv/scanner/internal/infrastructure/extractor"
)

// createTarGZ creates a test .tar.gz file with given file entries.
func createTarGZ(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
			Mode:     0o644,
		}
		tw.WriteHeader(hdr)  //nolint:errcheck
		tw.Write([]byte(content)) //nolint:errcheck
	}
	tw.Close()  //nolint:errcheck
	gw.Close()  //nolint:errcheck
	return path
}

// createZip creates a test .zip file.
func createZip(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content)) //nolint:errcheck
	}
	zw.Close() //nolint:errcheck
	return path
}

// ── TarExtractor ──────────────────────────────────────────────────────────────

func TestTarExtractor_Supports(t *testing.T) {
	e := &extractor.TarExtractor{}
	cases := []struct {
		filename string
		header   []byte
		want     bool
	}{
		{"test.tar.gz", nil, true},
		{"test.tgz", nil, true},
		{"test.tar.bz2", nil, true},
		{"test.tar.xz", nil, true},
		{"test.tar", nil, true},
		{"test.bin", nil, false},
		{"test.gz", []byte{0x1f, 0x8b, 0, 0, 0, 0, 0, 0}, true}, // gzip magic
	}
	for _, tt := range cases {
		got := e.Supports(tt.filename, tt.header)
		if got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}

func TestTarExtractor_Extract_TarGZ(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"usr/bin/openssl":       "OpenSSL 3.0.7 binary stub",
		"usr/lib/libssl.so.3":   "libssl content",
		"etc/nginx/nginx.conf":  "nginx/1.24.0 config",
	}
	path := createTarGZ(t, dir, files)

	e := &extractor.TarExtractor{}
	entries, err := e.Extract(context.Background(), path, extractor.DefaultMaxDepth)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	byPath := make(map[string]string)
	for _, en := range entries {
		byPath[en.Path] = string(en.Content)
	}
	if byPath["usr/bin/openssl"] == "" {
		t.Error("expected usr/bin/openssl in entries")
	}
}

func TestTarExtractor_ZipSlipPrevention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evil.tar.gz")

	f, _ := os.Create(path)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Path traversal entry
	tw.WriteHeader(&tar.Header{ //nolint:errcheck
		Name:     "../../etc/passwd",
		Size:     6,
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte("hacked")) //nolint:errcheck
	tw.Close()                 //nolint:errcheck
	gw.Close()                 //nolint:errcheck
	f.Close()                  //nolint:errcheck

	e := &extractor.TarExtractor{}
	entries, err := e.Extract(context.Background(), path, 1)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Malicious entry should be skipped
	for _, en := range entries {
		if en.Path == "../../etc/passwd" {
			t.Errorf("zip-slip entry should have been filtered")
		}
	}
}

func TestTarExtractor_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	// Create a valid archive
	files := map[string]string{"a": "aaa", "b": "bbb"}
	path := createTarGZ(t, dir, files)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	e := &extractor.TarExtractor{}
	// Should not panic; may return partial results
	_, _ = e.Extract(ctx, path, 1)
}

// ── ZipExtractor ──────────────────────────────────────────────────────────────

func TestZipExtractor_Extract(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"usr/bin/curl": "curl/7.88.1 binary",
		"README.txt":   "readme content",
	}
	path := createZip(t, dir, files)

	e := &extractor.ZipExtractor{}
	if !e.Supports("test.zip", nil) {
		t.Error("Supports(.zip) should be true")
	}

	entries, err := e.Extract(context.Background(), path, 1)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

// ── Dispatch ─────────────────────────────────────────────────────────────────

func TestDispatch(t *testing.T) {
	tests := []struct {
		filename string
		header   []byte
		found    bool
	}{
		{"test.tar.gz", nil, true},
		{"test.zip", nil, true},
		{"test.tar.bz2", nil, true},
		{"test.rpm", nil, true},
		{"test.deb", []byte("!<arch>\n"), true},
		{"test.bin", []byte{0x00, 0x01, 0x02}, false},
	}
	for _, tt := range tests {
		hdr := tt.header
		if hdr == nil {
			hdr = []byte{}
		}
		_, found := extractor.Dispatch(tt.filename, hdr)
		if found != tt.found {
			t.Errorf("Dispatch(%q): found=%v, want %v", tt.filename, found, tt.found)
		}
	}
}

// ── GzipExtractor ────────────────────────────────────────────────────────────

func TestGzipExtractor_SingleFile(t *testing.T) {
	dir := t.TempDir()
	// Create a .gz file (not tar.gz)
	path := filepath.Join(dir, "hello.gz")
	f, _ := os.Create(path)
	gw := gzip.NewWriter(f)
	gw.Write([]byte("hello world")) //nolint:errcheck
	gw.Close()                      //nolint:errcheck
	f.Close()                       //nolint:errcheck

	e := &extractor.GzipExtractor{}
	header := []byte{0x1f, 0x8b}
	if !e.Supports("hello.gz", header) {
		t.Error("GzipExtractor should support .gz")
	}
	// Should NOT support .tar.gz
	if e.Supports("archive.tar.gz", header) {
		t.Error("GzipExtractor should NOT support .tar.gz (TarExtractor handles it)")
	}

	entries, err := e.Extract(context.Background(), path, 1)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !bytes.Equal(entries[0].Content, []byte("hello world")) {
		t.Errorf("content mismatch: %q", entries[0].Content)
	}
}
