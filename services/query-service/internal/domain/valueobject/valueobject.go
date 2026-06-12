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

// Package valueobject contains value objects for the Vulnerability Query Service.
package valueobject

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// QueryCursor is a base64url-encoded opaque pagination cursor.
type QueryCursor struct {
	QueryNumber int    `json:"query_number"` // index of the sub-query (for batch queries)
	NDBCursor   string `json:"ndb_cursor"`   // Firestore pagination cursor
	Page        int    `json:"page"`
}

// Encode encodes the cursor as a base64url JSON token.
func (c QueryCursor) Encode() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("cursor encode: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DecodeCursor decodes a base64url token into a QueryCursor.
func DecodeCursor(token string) (QueryCursor, error) {
	if token == "" {
		return QueryCursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return QueryCursor{}, fmt.Errorf("cursor decode: invalid base64: %w", err)
	}
	var c QueryCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return QueryCursor{}, fmt.Errorf("cursor decode: invalid JSON: %w", err)
	}
	return c, nil
}

// CacheKey uniquely identifies a cached query result.
type CacheKey struct {
	QueryType string // "by_id" | "by_package_version" | "by_commit"
	Params    string // sha256(normalized query params)
	Page      int
}

// QuerySpec is the specification for a vulnerability query.
type QuerySpec struct {
	// At most one of these should be set
	VulnID      string // GetByID
	PackageName string // QueryAffected
	Ecosystem   string
	Version     string
	CommitHash  string // QueryByCommit
	PURL        string // QueryByPURL
}
