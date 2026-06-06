package parsers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/osv/scanner/internal/parsers"
)

// writeFile creates a temp file with given content.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
	return path
}

// ── FindParser ─────────────────────────────────────────────────────────────────

func TestFindParser(t *testing.T) {
	tests := []struct {
		filename string
		found    bool
	}{
		{"go.mod", true},
		{"requirements.txt", true},
		{"package.json", true},
		{"package-lock.json", true},
		{"Cargo.lock", true},
		{"Cargo.toml", true},
		{"pom.xml", true},
		{"build.gradle", true},
		{"Gemfile.lock", true},
		{"conanfile.txt", true},
		{"random.bin", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		p := parsers.FindParser(tt.filename)
		if tt.found && p == nil {
			t.Errorf("FindParser(%q): expected parser, got nil", tt.filename)
		}
		if !tt.found && p != nil {
			t.Errorf("FindParser(%q): expected nil, got %T", tt.filename, p)
		}
	}
}

// ── Golang Parser ──────────────────────────────────────────────────────────────

const goModContent = `module github.com/myapp

go 1.21

require (
	github.com/rs/zerolog v1.33.0
	golang.org/x/crypto v0.17.0
	github.com/pkg/errors v0.9.1 // indirect
)
`

func TestGolangParser(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "go.mod", goModContent)

	p := &parsers.GolangParser{}
	if !p.Supports("go.mod") {
		t.Error("Supports(go.mod) should be true")
	}
	if p.Supports("go.sum") {
		t.Error("Supports(go.sum) should be false")
	}

	results, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Find zerolog
	found := false
	for _, r := range results {
		if r.Product == "zerolog" {
			found = true
			if r.Version != "1.33.0" {
				t.Errorf("zerolog version: want 1.33.0, got %s", r.Version)
			}
			if r.PURL != "pkg:golang/github.com/rs/zerolog@1.33.0" {
				t.Errorf("zerolog PURL: %s", r.PURL)
			}
		}
	}
	if !found {
		t.Error("expected zerolog in results")
	}
}

// ── Python Parser ──────────────────────────────────────────────────────────────

const requirementsTxt = `Django==4.2.0
requests>=2.28.0  # range: skip
numpy~=1.24.0
flask             # no version: skip
pytest==7.4.0
`

func TestPythonParser(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "requirements.txt", requirementsTxt)

	p := &parsers.PythonParser{}
	results, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Should get Django, numpy (~=), pytest — skip requests (>=) and flask (no version)
	byName := make(map[string]string)
	for _, r := range results {
		byName[r.Product] = r.Version
	}

	if v, ok := byName["django"]; !ok || v != "4.2.0" {
		t.Errorf("django: want 4.2.0, got %v", byName["django"])
	}
	if v, ok := byName["numpy"]; !ok || v != "1.24.0" {
		t.Errorf("numpy: want 1.24.0, got %v", v)
	}
	if v, ok := byName["pytest"]; !ok || v != "7.4.0" {
		t.Errorf("pytest: want 7.4.0, got %v", v)
	}
	if _, ok := byName["requests"]; ok {
		t.Error("requests (>= range) should be skipped")
	}
	if _, ok := byName["flask"]; ok {
		t.Error("flask (no version) should be skipped")
	}
}

// ── Node.js Parser ─────────────────────────────────────────────────────────────

const packageLockJSON = `{
  "lockfileVersion": 2,
  "packages": {
    "node_modules/express": {"version": "4.18.2"},
    "node_modules/lodash": {"version": "4.17.21"},
    "": {"name": "myapp", "version": "1.0.0"}
  }
}`

func TestNodeJSParser_PackageLock(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "package-lock.json", packageLockJSON)

	p := &parsers.NodeJSParser{}
	results, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	byName := make(map[string]string)
	for _, r := range results {
		byName[r.Product] = r.Version
	}
	if byName["express"] != "4.18.2" {
		t.Errorf("express: want 4.18.2, got %v", byName["express"])
	}
	if byName["lodash"] != "4.17.21" {
		t.Errorf("lodash: want 4.17.21, got %v", byName["lodash"])
	}
}

const packageJSON = `{
  "dependencies": {
    "express": "^4.18.2",
    "lodash": "4.17.21",
    "some-pkg": ">=1.0.0"
  }
}`

func TestNodeJSParser_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "package.json", packageJSON)

	p := &parsers.NodeJSParser{}
	results, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	byName := make(map[string]string)
	for _, r := range results {
		byName[r.Product] = r.Version
	}
	if byName["express"] != "4.18.2" {
		t.Errorf("express: want 4.18.2 (stripped ^), got %v", byName["express"])
	}
	if byName["lodash"] != "4.17.21" {
		t.Errorf("lodash: want 4.17.21, got %v", byName["lodash"])
	}
	if _, ok := byName["some-pkg"]; ok {
		t.Error("some-pkg (>= range) should be skipped")
	}
}

// ── Rust Parser ────────────────────────────────────────────────────────────────

const cargoLock = `[[package]]
name = "openssl"
version = "0.10.55"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "tokio"
version = "1.35.0"
`

func TestRustParser_CargoLock(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "Cargo.lock", cargoLock)

	p := &parsers.RustParser{}
	results, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	byName := make(map[string]string)
	for _, r := range results {
		byName[r.Product] = r.Version
	}
	if byName["openssl"] != "0.10.55" {
		t.Errorf("openssl: want 0.10.55, got %v", byName["openssl"])
	}
	if byName["tokio"] != "1.35.0" {
		t.Errorf("tokio: want 1.35.0, got %v", byName["tokio"])
	}

	// PURL check
	for _, r := range results {
		if r.Product == "openssl" && r.PURL != "pkg:cargo/openssl@0.10.55" {
			t.Errorf("openssl PURL: want pkg:cargo/openssl@0.10.55, got %s", r.PURL)
		}
	}
}

// ── Java Parser ────────────────────────────────────────────────────────────────

const pomXML = `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <groupId>com.example</groupId>
  <artifactId>myapp</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>org.springframework</groupId>
      <artifactId>spring-core</artifactId>
      <version>6.0.0</version>
    </dependency>
    <dependency>
      <groupId>com.google.guava</groupId>
      <artifactId>guava</artifactId>
      <version>${guava.version}</version>
    </dependency>
  </dependencies>
</project>`

func TestJavaParser_POM(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "pom.xml", pomXML)

	p := &parsers.JavaParser{}
	results, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	byArtifact := make(map[string]string)
	for _, r := range results {
		byArtifact[r.Product] = r.Version
	}

	if byArtifact["spring-core"] != "6.0.0" {
		t.Errorf("spring-core: want 6.0.0, got %v", byArtifact["spring-core"])
	}
	// guava has ${} placeholder, should be skipped
	if _, ok := byArtifact["guava"]; ok {
		t.Error("guava with ${} placeholder should be skipped")
	}
}

// ── Parser No-Panic on Empty ───────────────────────────────────────────────────

func TestParsers_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "requirements.txt", "Cargo.lock", "pom.xml"} {
		path := writeFile(t, dir, name, "")
		p := parsers.FindParser(name)
		if p == nil {
			t.Errorf("no parser for %s", name)
			continue
		}
		results, err := p.Parse(context.Background(), path)
		if err != nil {
			t.Errorf("%s empty file: unexpected error: %v", name, err)
		}
		if len(results) != 0 {
			t.Errorf("%s empty file: expected 0 results, got %d", name, len(results))
		}
	}
}
