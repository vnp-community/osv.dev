// Compression/encoding checkers: zlib, libpng, libjpeg, libxml2, expat
package checkers

import parserentity "github.com/osv/scan-service/internal/parsers/entity"

func init() {
	// zlib
	Register(CheckerDef{
		Name: "zlib",
		ContainsPatterns: []string{
			`zlib version[: ]+\d`,
			`zlib\s+\d[\d.]+`,
			`ZLIB_VERSION`,
		},
		VersionPatterns: []string{
			`zlib version[: ]+([\d.]+)`,
			`zlib\s+([\d.]+)`,
			`ZLIB_VERSION\s+"([\d.]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^libz\.so`,
			`(?i)^libzlib`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "zlib", Product: "zlib"},
		},
		IgnorePatterns: []string{
			`#\s*define\s+ZLIB_VERSION`,
		},
	})

	// libpng
	Register(CheckerDef{
		Name: "libpng",
		ContainsPatterns: []string{
			`libpng version \d`,
			`PNG_LIBPNG_VER_STRING`,
		},
		VersionPatterns: []string{
			`libpng\s+([\d.]+)`,
			`PNG_LIBPNG_VER_STRING\s+"([\d.]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^libpng\.so`,
			`(?i)^libpng\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "libpng", Product: "libpng"},
		},
	})

	// libjpeg / libjpeg-turbo
	Register(CheckerDef{
		Name: "libjpeg",
		ContainsPatterns: []string{
			`libjpeg`,
			`JPEG.*Library.*version\s\d`,
			`IJG JPEG`,
		},
		VersionPatterns: []string{
			`JPEG.*Library.*version ([\d]+)`,
			`libjpeg-turbo\s+([\d.]+)`,
			`release ([\d]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libjpeg\.so`,
			`(?i)^libjpeg-turbo`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "ijg", Product: "libjpeg"},
			{Vendor: "libjpeg-turbo", Product: "libjpeg-turbo"},
		},
	})

	// libxml2
	Register(CheckerDef{
		Name: "libxml2",
		ContainsPatterns: []string{
			`libxml version: \d`,
			`LIBXML_DOTTED_VERSION`,
		},
		VersionPatterns: []string{
			`libxml version: ([\d.]+)`,
			`LIBXML_DOTTED_VERSION\s+"([\d.]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^libxml2\.so`,
			`(?i)^libxml2-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "xmlsoft", Product: "libxml2"},
		},
	})

	// expat (libexpat)
	Register(CheckerDef{
		Name: "expat",
		ContainsPatterns: []string{
			`expat_\d`,
			`XML_MAJOR_VERSION`,
			`Expat/\d`,
		},
		VersionPatterns: []string{
			`expat_([\d.]+)`,
			`Expat/([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libexpat\.so`,
			`(?i)^libexpat-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "libexpat", Product: "libexpat"},
		},
	})
}
