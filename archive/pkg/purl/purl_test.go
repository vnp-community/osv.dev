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

package purl_test

import (
	"testing"

	"github.com/osv/pkg/purl"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input     string
		wantType  string
		wantName  string
		wantNS    string
		wantVer   string
		wantErr   bool
	}{
		{
			input:    "pkg:pypi/requests@2.28.0",
			wantType: "pypi", wantName: "requests", wantVer: "2.28.0",
		},
		{
			input:    "pkg:golang/github.com/gin-gonic/gin@v1.9.0",
			wantType: "golang", wantNS: "github.com/gin-gonic", wantName: "gin", wantVer: "v1.9.0",
		},
		{
			input:    "pkg:npm/%40angular/core@15.0.0",
			wantType: "npm", wantName: "@angular/core", wantVer: "15.0.0",
		},
		{
			input:   "noprefix:foo/bar",
			wantErr: true,
		},
		{
			input:   "pkg:noslash",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			p, err := purl.Parse(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Type != tc.wantType {
				t.Errorf("Type: got %q, want %q", p.Type, tc.wantType)
			}
			if p.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", p.Name, tc.wantName)
			}
			if p.Namespace != tc.wantNS {
				t.Errorf("Namespace: got %q, want %q", p.Namespace, tc.wantNS)
			}
			if p.Version != tc.wantVer {
				t.Errorf("Version: got %q, want %q", p.Version, tc.wantVer)
			}
		})
	}
}

func TestEcosystem(t *testing.T) {
	cases := []struct{ purl, want string }{
		{"pkg:pypi/requests@1.0", "PyPI"},
		{"pkg:golang/github.com/foo/bar@v1.0", "Go"},
		{"pkg:npm/lodash@4.17", "npm"},
		{"pkg:maven/com.google.guava/guava@30.0", "Maven"},
		{"pkg:cargo/serde@1.0", "crates.io"},
	}
	for _, c := range cases {
		p, _ := purl.Parse(c.purl)
		if got := p.Ecosystem(); got != c.want {
			t.Errorf("Ecosystem(%q) = %q, want %q", c.purl, got, c.want)
		}
	}
}
