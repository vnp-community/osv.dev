package checkers_test

import (
	"strings"
	"testing"

	"github.com/osv/scanner/internal/checkers"
	"github.com/osv/scanner/internal/domain/entity"
)

// ── Registry Tests ────────────────────────────────────────────────────────────

func TestBuildAll_NoErrors(t *testing.T) {
	compiled, err := checkers.BuildAll()
	if err != nil {
		t.Fatalf("BuildAll() error: %v", err)
	}
	if len(compiled) == 0 {
		t.Error("expected at least one checker")
	}
	t.Logf("Built %d checkers", len(compiled))
}

func TestCount_AtLeast20(t *testing.T) {
	count := checkers.Count()
	if count < 20 {
		t.Errorf("expected at least 20 checkers registered, got %d", count)
	}
}

// ── CheckerDef Validation Tests ───────────────────────────────────────────────

func TestCheckerDef_EmptyName(t *testing.T) {
	def := checkers.CheckerDef{
		Name:          "",
		VendorProduct: []entity.VendorProduct{{Vendor: "x", Product: "y"}},
	}
	_, err := def.Build()
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestCheckerDef_EmptyVendorProduct(t *testing.T) {
	def := checkers.CheckerDef{Name: "test", VendorProduct: nil}
	_, err := def.Build()
	if err == nil {
		t.Error("expected error for empty VendorProduct")
	}
}

func TestCheckerDef_UppercaseVendor(t *testing.T) {
	def := checkers.CheckerDef{
		Name:          "test",
		VendorProduct: []entity.VendorProduct{{Vendor: "OpenSSL", Product: "openssl"}},
	}
	_, err := def.Build()
	if err == nil {
		t.Error("expected error for uppercase vendor")
	}
}

func TestCheckerDef_DuplicateVendorProduct(t *testing.T) {
	def := checkers.CheckerDef{
		Name: "test",
		VendorProduct: []entity.VendorProduct{
			{Vendor: "a", Product: "b"},
			{Vendor: "a", Product: "b"},
		},
	}
	_, err := def.Build()
	if err == nil {
		t.Error("expected error for duplicate vendor/product")
	}
}

func TestCheckerDef_InvalidRegex(t *testing.T) {
	def := checkers.CheckerDef{
		Name:             "test",
		ContainsPatterns: []string{`[invalid`},
		VendorProduct:    []entity.VendorProduct{{Vendor: "a", Product: "b"}},
	}
	_, err := def.Build()
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

// ── Checker Behavior Tests ─────────────────────────────────────────────────────

func buildChecker(t *testing.T, def checkers.CheckerDef) *entity.Checker {
	t.Helper()
	c, err := def.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	return c
}

func TestChecker_MatchFilename_CaseInsensitive(t *testing.T) {
	c := buildChecker(t, checkers.CheckerDef{
		Name:             "test",
		FilenamePatterns: []string{`(?i)^libssl\.so`},
		VendorProduct:    []entity.VendorProduct{{Vendor: "openssl", Product: "openssl"}},
	})
	tests := []struct {
		fname string
		want  bool
	}{
		{"libssl.so.3", true},
		{"LIBSSL.SO.3", true},
		{"/usr/lib/libssl.so.3", true},
		{"libcrypto.so.3", false},
	}
	for _, tt := range tests {
		got := c.MatchFilename(tt.fname)
		if got != tt.want {
			t.Errorf("MatchFilename(%q) = %v, want %v", tt.fname, got, tt.want)
		}
	}
}

func TestChecker_MatchContent_IgnorePatterns(t *testing.T) {
	c := buildChecker(t, checkers.CheckerDef{
		Name:             "test",
		ContainsPatterns: []string{`OpenSSL\s[\d.]+`},
		IgnorePatterns:   []string{`OpenSSL Project Authors`},
		VendorProduct:    []entity.VendorProduct{{Vendor: "openssl", Product: "openssl"}},
	})

	// Normal match
	if !c.MatchContent("OpenSSL 3.0.0 binary") {
		t.Error("expected match for 'OpenSSL 3.0.0 binary'")
	}
	// Ignored by ignore pattern
	if c.MatchContent("Copyright OpenSSL Project Authors") {
		t.Error("expected NO match (ignore pattern)")
	}
}

func TestChecker_ExtractVersion(t *testing.T) {
	c := buildChecker(t, checkers.CheckerDef{
		Name:            "test",
		VersionPatterns: []string{`OpenSSL\s([\d.]+[a-z]?)`},
		VendorProduct:   []entity.VendorProduct{{Vendor: "openssl", Product: "openssl"}},
	})

	tests := []struct {
		content string
		version string
	}{
		{"OpenSSL 3.0.7 01 Nov 2022", "3.0.7"},
		{"OpenSSL 1.1.1k  25 Mar 2021", "1.1.1k"},
		{"something else entirely", ""},
	}
	for _, tt := range tests {
		got := c.ExtractVersion(tt.content)
		if got != tt.version {
			t.Errorf("ExtractVersion(%q) = %q, want %q", tt.content, got, tt.version)
		}
	}
}

// ── Spot Tests for Key Checkers ────────────────────────────────────────────────

func findChecker(t *testing.T, name string) *entity.Checker {
	t.Helper()
	all, err := checkers.BuildAll()
	if err != nil {
		t.Fatalf("BuildAll: %v", err)
	}
	for _, c := range all {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("checker %q not found", name)
	return nil
}

var checkerTests = []struct {
	checker string
	content string
	version string
}{
	{"openssl", "OpenSSL 3.0.7 01 Nov 2022", "3.0.7"},
	{"openssl", "OpenSSL 1.1.1k  25 Mar 2021", "1.1.1k"},
	{"curl", "curl/7.88.1", "7.88.1"},
	{"libcurl", "libcurl/7.88.1 OpenSSL/3.0.7", "7.88.1"},
	{"openssh", "OpenSSH_9.0p1 Ubuntu-1ubuntu8.5", "9.0p1"},
	{"nginx", "nginx/1.24.0", "1.24.0"},
	{"apache", "Apache/2.4.57 (Debian)", "2.4.57"},
	{"zlib", "zlib version: 1.2.13", "1.2.13"},
	{"sqlite", "SQLite version 3.40.0", "3.40.0"},
	{"busybox", "BusyBox v1.36.1 (2023-06-23)", "1.36.1"},
	{"bash", "GNU bash, version 5.2.15(1)-release", "5.2.15"},
	{"sudo", "Sudo version 1.9.13p3", "1.9.13p3"},
	{"mbedtls", "Mbed TLS version 3.4.0", "3.4.0"},
	{"lighttpd", "lighttpd/1.4.71", "1.4.71"},
	{"libssh2", "libssh2/1.11.0", "1.11.0"},
}

func TestCheckerSpotTests(t *testing.T) {
	for _, tt := range checkerTests {
		tt := tt
		t.Run(tt.checker, func(t *testing.T) {
			c := findChecker(t, tt.checker)
			if !c.MatchContent(tt.content) {
				t.Errorf("checker %q: expected match for %q", tt.checker, tt.content)
			}
			got := c.ExtractVersion(tt.content)
			if got != tt.version {
				t.Errorf("checker %q: ExtractVersion(%q) = %q, want %q", tt.checker, tt.content, got, tt.version)
			}
		})
	}
}

func TestCheckerNegativeMatch(t *testing.T) {
	c := findChecker(t, "openssl")
	randomContent := "this is a random binary with no SSL strings"
	if c.MatchContent(randomContent) {
		t.Errorf("expected NO match for random content")
	}
	if v := c.ExtractVersion(randomContent); v != "" {
		t.Errorf("expected empty version for non-matching content, got %q", v)
	}
}

func TestAllCheckersHaveLowercaseVendorProduct(t *testing.T) {
	all, err := checkers.BuildAll()
	if err != nil {
		t.Fatalf("BuildAll: %v", err)
	}
	for _, c := range all {
		for _, vp := range c.VendorProducts() {
			if vp.Vendor != strings.ToLower(vp.Vendor) {
				t.Errorf("checker %q: vendor %q is not lowercase", c.Name(), vp.Vendor)
			}
			if vp.Product != strings.ToLower(vp.Product) {
				t.Errorf("checker %q: product %q is not lowercase", c.Name(), vp.Product)
			}
		}
	}
}
