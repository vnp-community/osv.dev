// Package entity — Product and scan result types.
package entity

// ProductInfo describes a detected software package.
type ProductInfo struct {
	Vendor  string // CPE vendor (lowercase)
	Product string // CPE product (lowercase)
	Version string // detected version string
	PURL    string // Package URL (optional, set by parsers)
	Source  string // "checker"|"parser" — detection method
}

// ScanInfo is emitted by CheckerService for each detected product per file.
type ScanInfo struct {
	FilePath string
	ProductInfo
}

// FileType represents a detected file type (binary, archive, etc.)
type FileType string

const (
	FileTypeUnknown FileType = ""
	FileTypeELF     FileType = "elf"
	FileTypePE      FileType = "pe"
	FileTypeMachO   FileType = "macho"
	FileTypeTarGZ   FileType = "tar.gz"
	FileTypeTarBZ2  FileType = "tar.bz2"
	FileTypeTarXZ   FileType = "tar.xz"
	FileTypeTar     FileType = "tar"
	FileTypeZip     FileType = "zip"
	FileTypeRPM     FileType = "rpm"
	FileTypeDeb     FileType = "deb"
	FileTypeCpio    FileType = "cpio"
	FileTypeGzip    FileType = "gzip"
	FileTypeBzip2   FileType = "bzip2"
	FileTypeXz      FileType = "xz"
	FileTypeZstd    FileType = "zstd"
	FileTypeText    FileType = "text"
)
