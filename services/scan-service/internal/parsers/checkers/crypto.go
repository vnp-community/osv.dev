// Cryptography checkers: OpenSSL, BoringSSL, LibreSSL, MbedTLS
package checkers

import parserentity "github.com/osv/scan-service/internal/parsers/entity"

func init() {
	// OpenSSL
	Register(CheckerDef{
		Name: "openssl",
		ContainsPatterns: []string{
			`OpenSSL\s+[\d.]+[a-z]?`,
			`libssl\.so`,
			`libcrypto\.so`,
		},
		VersionPatterns: []string{
			`OpenSSL\s+([\d.]+[a-z]?)(?:\s+\d+\s+\w+\s+\d{4})?`,
		},
		FilenamePatterns: []string{
			`(?i)^libssl\.so`,
			`(?i)^libcrypto\.so`,
			`(?i)^libssl-\d`,
			`(?i)^libcrypto-\d`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openssl", Product: "openssl"},
		},
		IgnorePatterns: []string{
			`OpenSSL Project Authors`,
			`#\s*define\s+OPENSSL`,
			`\*\s*OpenSSL`,
		},
	})

	// OpenSSL 1.0.x / legacy patterns
	Register(CheckerDef{
		Name: "openssl_legacy",
		ContainsPatterns: []string{
			`SSLeay\s[\d.]+`,
			`OpenSSL\s0\.[89]`,
			`OpenSSL\s1\.0`,
		},
		VersionPatterns: []string{
			`OpenSSL\s([\d.]+[a-z]?)`,
			`SSLeay\s([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libssl\.so\.0`,
			`(?i)^libcrypto\.so\.0`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openssl", Product: "openssl"},
		},
	})

	// BoringSSL
	Register(CheckerDef{
		Name: "boringssl",
		ContainsPatterns: []string{
			`BoringSSL`,
			`BORINGSSL_VERSION_NUMBER`,
		},
		VersionPatterns: []string{
			`BoringSSL\s+([\d.]+)`,
			`BORINGSSL_VERSION_NUMBER\s+([\d]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^boringssl`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "google", Product: "boringssl"},
		},
	})

	// LibreSSL
	Register(CheckerDef{
		Name: "libressl",
		ContainsPatterns: []string{
			`LibreSSL\s[\d.]+`,
			`LIBRESSL_VERSION_TEXT`,
		},
		VersionPatterns: []string{
			`LibreSSL\s([\d.]+)`,
		},
		FilenamePatterns: []string{
			`(?i)^libressl`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "openbsd", Product: "libressl"},
		},
	})

	// MbedTLS / PolarSSL
	Register(CheckerDef{
		Name: "mbedtls",
		ContainsPatterns: []string{
			`Mbed TLS version\s[\d.]+`,
			`mbed TLS`,
			`MBEDTLS_VERSION_STRING`,
		},
		VersionPatterns: []string{
			`Mbed TLS version\s([\d.]+)`,
			`MBEDTLS_VERSION_STRING\s+"([\d.]+)"`,
		},
		FilenamePatterns: []string{
			`(?i)^libmbedtls`,
			`(?i)^libpolarssl`,
		},
		VendorProduct: []parserentity.VendorProduct{
			{Vendor: "arm", Product: "mbed_tls"},
		},
	})
}
