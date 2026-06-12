// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package purl provides PURL (Package URL) parsing and validation.
// Format: pkg:type/namespace/name@version?qualifiers#subpath
package purl

import (
	"fmt"
	"net/url"
	"strings"
)

// PURL represents a parsed Package URL.
type PURL struct {
	Type       string
	Namespace  string
	Name       string
	Version    string
	Qualifiers map[string]string
	Subpath    string
	Raw        string
}

// Parse parses a PURL string into a PURL struct.
// Returns an error if the string is not a valid PURL.
func Parse(raw string) (*PURL, error) {
	if !strings.HasPrefix(raw, "pkg:") {
		return nil, fmt.Errorf("purl: must start with 'pkg:' scheme, got %q", raw)
	}

	// Strip "pkg:" prefix
	rest := raw[4:]

	// Split off subpath (#...)
	subpath := ""
	if idx := strings.Index(rest, "#"); idx >= 0 {
		subpath = rest[idx+1:]
		rest = rest[:idx]
	}

	// Split off qualifiers (?...)
	qualifiers := map[string]string{}
	if idx := strings.Index(rest, "?"); idx >= 0 {
		qs := rest[idx+1:]
		rest = rest[:idx]
		for _, kv := range strings.Split(qs, "&") {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				qualifiers[parts[0]] = parts[1]
			}
		}
	}

	// Split type from the rest: type/namespace/name@version
	slashIdx := strings.Index(rest, "/")
	if slashIdx < 0 {
		return nil, fmt.Errorf("purl: missing type/name separator in %q", raw)
	}
	purlType := strings.ToLower(rest[:slashIdx])
	rest = rest[slashIdx+1:]

	// Split version (@...)
	version := ""
	if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
		version = rest[atIdx+1:]
		rest = rest[:atIdx]
	}

	// URL-decode the remaining path (namespace/name) before splitting
	decodedRest, err := url.PathUnescape(rest)
	if err != nil {
		return nil, fmt.Errorf("purl: invalid URL encoding in %q: %w", rest, err)
	}

	// Split namespace/name
	namespace := ""
	name := decodedRest
	if lastSlash := strings.LastIndex(decodedRest, "/"); lastSlash >= 0 {
		namespace = decodedRest[:lastSlash]
		name = decodedRest[lastSlash+1:]
	}

	// For npm scoped packages, the namespace starts with '@' (e.g. "@angular").
	// By convention, we flatten namespace+name into a single name: "@angular/core".
	if strings.HasPrefix(namespace, "@") {
		name = namespace + "/" + name
		namespace = ""
	}

	if purlType == "" || name == "" {
		return nil, fmt.Errorf("purl: type and name are required, got type=%q name=%q", purlType, name)
	}

	return &PURL{
		Type:       purlType,
		Namespace:  namespace,
		Name:       name,
		Version:    version,
		Qualifiers: qualifiers,
		Subpath:    subpath,
		Raw:        raw,
	}, nil
}

// String returns the canonical PURL string representation.
func (p *PURL) String() string {
	var sb strings.Builder
	sb.WriteString("pkg:")
	sb.WriteString(p.Type)
	sb.WriteString("/")
	if p.Namespace != "" {
		sb.WriteString(p.Namespace)
		sb.WriteString("/")
	}
	sb.WriteString(url.PathEscape(p.Name))
	if p.Version != "" {
		sb.WriteString("@")
		sb.WriteString(p.Version)
	}
	// Qualifiers
	if len(p.Qualifiers) > 0 {
		sb.WriteString("?")
		first := true
		for k, v := range p.Qualifiers {
			if !first {
				sb.WriteString("&")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(v)
			first = false
		}
	}
	if p.Subpath != "" {
		sb.WriteString("#")
		sb.WriteString(p.Subpath)
	}
	return sb.String()
}

// Ecosystem returns the OSV ecosystem name for this PURL type.
func (p *PURL) Ecosystem() string {
	switch strings.ToLower(p.Type) {
	case "pypi":
		return "PyPI"
	case "golang":
		return "Go"
	case "npm":
		return "npm"
	case "maven":
		return "Maven"
	case "cargo":
		return "crates.io"
	case "gem":
		return "RubyGems"
	case "nuget":
		return "NuGet"
	case "composer":
		return "Packagist"
	case "pub":
		return "Pub"
	case "hex":
		return "Hex"
	default:
		return p.Type
	}
}
