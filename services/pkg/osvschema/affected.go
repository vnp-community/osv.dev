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

package osvschema

// Affected describes how a package is affected by the vulnerability.
type Affected struct {
	Package           Package            `json:"package"`
	Severity          []Severity         `json:"severity,omitempty"`
	Ranges            []Range            `json:"ranges,omitempty"`
	Versions          []string           `json:"versions,omitempty"`
	EcosystemSpecific interface{}        `json:"ecosystem_specific,omitempty"`
	DatabaseSpecific  interface{}        `json:"database_specific,omitempty"`
}

// Package identifies the package affected by the vulnerability.
type Package struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	PURL      string `json:"purl,omitempty"`
}

// Range describes a version range in which the vulnerability exists.
type Range struct {
	Type             string      `json:"type"` // SEMVER | ECOSYSTEM | GIT
	Repo             string      `json:"repo,omitempty"`
	Events           []Event     `json:"events"`
	DatabaseSpecific interface{} `json:"database_specific,omitempty"`
}

// Event marks a version where the vulnerability was introduced or fixed.
type Event struct {
	Introduced   string `json:"introduced,omitempty"`
	Fixed        string `json:"fixed,omitempty"`
	LastAffected string `json:"last_affected,omitempty"`
	Limit        string `json:"limit,omitempty"`
}
