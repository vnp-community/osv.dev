package extractor

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// RPMExtractor extracts the CPIO payload from .rpm files.
// RPM format: Lead (96 bytes) + Signature Header + Main Header + Payload (cpio.gz or cpio.zst)
type RPMExtractor struct{}

func (e *RPMExtractor) Supports(filename string, header []byte) bool {
	if strings.HasSuffix(strings.ToLower(filename), ".rpm") {
		return true
	}
	// RPM magic: 0xED 0xAB 0xEE 0xDB
	return len(header) >= 4 &&
		header[0] == 0xed && header[1] == 0xab && header[2] == 0xee && header[3] == 0xdb
}

func (e *RPMExtractor) Extract(ctx context.Context, archivePath string, maxDepth int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("rpm: open: %w", err)
	}
	defer f.Close() //nolint:errcheck

	// Skip RPM Lead (96 bytes)
	if _, err := f.Seek(96, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rpm: seek lead: %w", err)
	}

	// Skip headers (signature + main)
	for i := 0; i < 2; i++ {
		if err := skipRPMHeader(f); err != nil {
			return nil, fmt.Errorf("rpm: skip header %d: %w", i, err)
		}
	}

	// Remaining bytes: gzip-compressed CPIO
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("rpm: payload gzip: %w", err)
	}
	defer gr.Close() //nolint:errcheck

	return readCPIOEntries(ctx, gr)
}

// skipRPMHeader skips one RPM header section (magic + index + data).
// RPM header: 8-byte magic + 4-byte nindex + 4-byte hsize + (16*nindex) entries + hsize data
func skipRPMHeader(r io.ReadSeeker) error {
	// Read magic (8 bytes)
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return err
	}
	// 0x8e 0xad 0xe8 0x01 for RPM headers
	if magic[0] != 0x8e || magic[1] != 0xad || magic[2] != 0xe8 {
		return fmt.Errorf("invalid RPM header magic")
	}

	// nindex (uint32 BE) + hsize (uint32 BE)
	var nindex, hsize uint32
	if err := binary.Read(r, binary.BigEndian, &nindex); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &hsize); err != nil {
		return err
	}

	// Skip index entries (16 bytes each) + data
	skip := int64(nindex)*16 + int64(hsize)
	if _, err := r.(io.Seeker).Seek(skip, io.SeekCurrent); err != nil {
		return err
	}

	// Align to 8-byte boundary
	pos, err := r.(io.Seeker).Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if pad := pos % 8; pad != 0 {
		r.(io.Seeker).Seek(8-pad, io.SeekCurrent) //nolint:errcheck
	}

	return nil
}

// readCPIOEntries reads cpio newc format files.
// cpio newc header starts with "070701" (6 bytes) followed by hex fields.
func readCPIOEntries(ctx context.Context, r io.Reader) ([]FileEntry, error) {
	baseDir := "/"
	var entries []FileEntry

	for {
		if ctx.Err() != nil {
			return entries, ctx.Err()
		}

		// Read cpio header (110 bytes for newc format)
		hdr := make([]byte, 110)
		if _, err := io.ReadFull(r, hdr); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return entries, fmt.Errorf("cpio header: %w", err)
		}

		magic := string(hdr[:6])
		if magic != "070701" && magic != "070702" {
			break // not cpio newc
		}

		// Parse namesize and filesize from cpio header (hex fields)
		namesize := hexField(hdr, 94, 8)
		filesize := hexField(hdr, 54, 8)
		filename := readCPIOName(r, namesize)

		// Align after header + name to 4-byte boundary
		hdrNameLen := 110 + namesize
		if pad := hdrNameLen % 4; pad != 0 {
			io.ReadFull(r, make([]byte, 4-pad)) //nolint:errcheck
		}

		if filename == "TRAILER!!!" {
			break
		}

		if filesize == 0 || strings.HasSuffix(filename, "/") {
			continue
		}

		// Zip-slip check
		if _, serr := safePath(baseDir, filename); serr != nil {
			io.ReadFull(r, make([]byte, filesize)) //nolint:errcheck
			continue
		}

		if int64(filesize) > MaxFileSize {
			io.ReadFull(r, make([]byte, filesize)) //nolint:errcheck
			continue
		}

		content := make([]byte, filesize)
		if _, err := io.ReadFull(r, content); err != nil {
			break
		}

		// Align after file data to 4-byte boundary
		if pad := filesize % 4; pad != 0 {
			io.ReadFull(r, make([]byte, 4-pad)) //nolint:errcheck
		}

		entries = append(entries, FileEntry{
			Path:    filename,
			Content: content,
			Size:    int64(filesize),
		})
	}

	return entries, nil
}

func hexField(data []byte, offset, length int) int {
	var v int
	for i := offset; i < offset+length; i++ {
		b := data[i]
		var digit int
		switch {
		case b >= '0' && b <= '9':
			digit = int(b - '0')
		case b >= 'a' && b <= 'f':
			digit = int(b-'a') + 10
		case b >= 'A' && b <= 'F':
			digit = int(b-'A') + 10
		}
		v = v*16 + digit
	}
	return v
}

func readCPIOName(r io.Reader, namesize int) string {
	if namesize <= 0 {
		return ""
	}
	buf := make([]byte, namesize)
	io.ReadFull(r, buf) //nolint:errcheck
	// Trim null terminator
	return strings.TrimRight(string(buf), "\x00")
}

// DebExtractor extracts data.tar.* from .deb (ar) archives.
type DebExtractor struct{}

func (e *DebExtractor) Supports(filename string, header []byte) bool {
	if strings.HasSuffix(strings.ToLower(filename), ".deb") {
		return true
	}
	// ar magic: "!<arch>\n"
	return len(header) >= 7 && string(header[:7]) == "!<arch>"
}

func (e *DebExtractor) Extract(ctx context.Context, archivePath string, maxDepth int) ([]FileEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("deb: open: %w", err)
	}
	defer f.Close() //nolint:errcheck

	// Skip ar global header "!<arch>\n" (8 bytes)
	globalHdr := make([]byte, 8)
	if _, err := io.ReadFull(f, globalHdr); err != nil {
		return nil, fmt.Errorf("deb: ar header: %w", err)
	}
	if string(globalHdr[:7]) != "!<arch>" {
		return nil, fmt.Errorf("deb: not an ar archive")
	}

	// Parse ar entries: each is 60-byte header + data
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		arHdr := make([]byte, 60)
		n, err := io.ReadFull(f, arHdr)
		if err != nil || n < 60 {
			break
		}

		// ar header fields (ASCII space-padded)
		filename := strings.TrimSpace(string(arHdr[0:16]))
		sizeStr := strings.TrimSpace(string(arHdr[48:58]))
		size := 0
		for _, ch := range sizeStr {
			if ch >= '0' && ch <= '9' {
				size = size*10 + int(ch-'0')
			}
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(f, data); err != nil {
			break
		}
		// Align to 2-byte boundary
		if size%2 != 0 {
			f.Seek(1, io.SeekCurrent) //nolint:errcheck
		}

		// Find data.tar.* (contains actual file payload)
		if strings.HasPrefix(filename, "data.tar") {
			// Extract the inner tar
			tmpFile, err := writeTempFile(data)
			if err != nil {
				return nil, err
			}
			defer os.Remove(tmpFile) //nolint:errcheck

			tarEx := &TarExtractor{}
			return tarEx.Extract(ctx, tmpFile, maxDepth)
		}
	}

	return nil, fmt.Errorf("deb: data.tar.* not found")
}

func writeTempFile(data []byte) (string, error) {
	f, err := os.CreateTemp("", "deb-data-*.tar.*")
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// Ensure TarExtractor satisfies interface check.
var _ Extractor = (*TarExtractor)(nil)
var _ Extractor = (*ZipExtractor)(nil)
var _ Extractor = (*RPMExtractor)(nil)
var _ Extractor = (*DebExtractor)(nil)

// cpio reader using archive/tar is not available directly; using custom above.
// Keep a reference to use tar.NewReader for any future nested extraction.
var _ = tar.NewReader
