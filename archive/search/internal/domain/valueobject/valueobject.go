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

// Package valueobject holds value objects for the Search domain.
package valueobject

import "time"

// SearchQuery is the parsed and structured form of a user query.
type SearchQuery struct {
	Raw        string
	Keywords   string
	Ecosystems []string
	Severities []string
	DateFrom   time.Time
	DateTo     time.Time
	Withdrawn  *bool
	Sources    []string
	// SortOrder controls result ordering.
	SortOrder SortOrder
	PageSize  int
	PageToken string
}

// SortOrder controls how search results are ordered.
type SortOrder int

const (
	SortRelevance SortOrder = iota
	SortDateDesc
	SortDateAsc
	SortSeverity
)
