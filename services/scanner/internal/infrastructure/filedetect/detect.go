// Package filedetect detects file types from magic bytes and extension.
package filedetect

import (
	"path/filepath"
	"strings"

	"github.com/osv/scanner/internal/domain/entity"
)

// Magic byte signatures for common file types.
var signatures = []struct {
	ft     entity.FileType
	offset int
	magic  []byte
}{
	{entity.FileTypeELF, 0, []byte{0x7f, 0x45, 0x4c, 0x46}},               // ELF
	{entity.FileTypePE, 0, []byte{0x4d, 0x5a}},                             // PE (MZ)
	{entity.FileTypeMachO, 0, []byte{0xca, 0xfe, 0xba, 0xbe}},              // Mach-O fat
	{entity.FileTypeMachO, 0, []byte{0xfe, 0xed, 0xfa, 0xce}},              // Mach-O 32-bit
	{entity.FileTypeMachO, 0, []byte{0xfe, 0xed, 0xfa, 0xcf}},              // Mach-O 64-bit
	{entity.FileTypeTarGZ, 0, []byte{0x1f, 0x8b}},                          // gzip
	{entity.FileTypeTarBZ2, 0, []byte{0x42, 0x5a, 0x68}},                   // bzip2
	{entity.FileTypeTarXZ, 0, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}}, // xz
	{entity.FileTypeZstd, 0, []byte{0x28, 0xb5, 0x2f, 0xfd}},               // zstd
	{entity.FileTypeZip, 0, []byte{0x50, 0x4b, 0x03, 0x04}},                // zip
	{entity.FileTypeZip, 0, []byte{0x50, 0x4b, 0x05, 0x06}},                // zip (empty)
	{entity.FileTypeRPM, 0, []byte{0xed, 0xab, 0xee, 0xdb}},                // RPM
	{entity.FileTypeDeb, 0, []byte{0x21, 0x3c, 0x61, 0x72, 0x63, 0x68}},   // ar "!<arch>"
	{entity.FileTypeCpio, 0, []byte{0x30, 0x37, 0x30, 0x37, 0x30}},         // cpio newc "07070"
}

// DetectType returns the FileType based on magic bytes and/or filename.
// header should contain at least the first 512 bytes.
func DetectType(header []byte, filename string) entity.FileType {
	// Check magic bytes first
	for _, sig := range signatures {
		if matchesMagic(header, sig.offset, sig.magic) {
			// Refine gzip: check extension to distinguish .tar.gz from .gz
			if sig.ft == entity.FileTypeTarGZ {
				ext := strings.ToLower(filename)
				if strings.HasSuffix(ext, ".tar.gz") || strings.HasSuffix(ext, ".tgz") {
					return entity.FileTypeTarGZ
				}
				return entity.FileTypeGzip
			}
			// Refine bzip2
			if sig.ft == entity.FileTypeTarBZ2 {
				ext := strings.ToLower(filename)
				if strings.HasSuffix(ext, ".tar.bz2") {
					return entity.FileTypeTarBZ2
				}
				return entity.FileTypeBzip2
			}
			// Refine xz
			if sig.ft == entity.FileTypeTarXZ {
				ext := strings.ToLower(filename)
				if strings.HasSuffix(ext, ".tar.xz") {
					return entity.FileTypeTarXZ
				}
				return entity.FileTypeXz
			}
			// Refine zstd
			if sig.ft == entity.FileTypeZstd {
				ext := strings.ToLower(filename)
				if strings.HasSuffix(ext, ".tar.zst") {
					return entity.FileTypeZstd // tar.zst
				}
				return entity.FileTypeZstd
			}
			return sig.ft
		}
	}

	// Fallback to extension
	return detectByExtension(filename)
}

// IsBinary returns true for binary file types.
func IsBinary(ft entity.FileType) bool {
	switch ft {
	case entity.FileTypeELF, entity.FileTypePE, entity.FileTypeMachO:
		return true
	}
	return false
}

// IsArchive returns true for archive/container file types.
func IsArchive(ft entity.FileType) bool {
	switch ft {
	case entity.FileTypeTarGZ, entity.FileTypeTarBZ2, entity.FileTypeTarXZ, entity.FileTypeTar,
		entity.FileTypeZip, entity.FileTypeRPM, entity.FileTypeDeb, entity.FileTypeCpio,
		entity.FileTypeGzip, entity.FileTypeBzip2, entity.FileTypeXz, entity.FileTypeZstd:
		return true
	}
	return false
}

func matchesMagic(data []byte, offset int, magic []byte) bool {
	if len(data) < offset+len(magic) {
		return false
	}
	for i, b := range magic {
		if data[offset+i] != b {
			return false
		}
	}
	return true
}

func detectByExtension(filename string) entity.FileType {
	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(base, ".tar.gz"), strings.HasSuffix(base, ".tgz"):
		return entity.FileTypeTarGZ
	case strings.HasSuffix(base, ".tar.bz2"):
		return entity.FileTypeTarBZ2
	case strings.HasSuffix(base, ".tar.xz"):
		return entity.FileTypeTarXZ
	case strings.HasSuffix(base, ".tar.zst"):
		return entity.FileTypeZstd
	case ext == ".tar":
		return entity.FileTypeTar
	case ext == ".zip":
		return entity.FileTypeZip
	case ext == ".gz":
		return entity.FileTypeGzip
	case ext == ".bz2":
		return entity.FileTypeBzip2
	case ext == ".xz":
		return entity.FileTypeXz
	case ext == ".zst":
		return entity.FileTypeZstd
	case ext == ".rpm":
		return entity.FileTypeRPM
	case ext == ".deb":
		return entity.FileTypeDeb
	}
	return entity.FileTypeUnknown
}
