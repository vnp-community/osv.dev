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

// Package pagination provides cursor-based pagination utilities.
// Cursors are base64url-encoded JSON objects.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Cursor is an opaque pagination token that wraps arbitrary data.
type Cursor struct {
	data map[string]interface{}
}

// NewCursor creates a cursor from a map of key-value pairs.
func NewCursor(data map[string]interface{}) *Cursor {
	return &Cursor{data: data}
}

// Encode encodes the cursor as a base64url-encoded JSON string.
func (c *Cursor) Encode() (string, error) {
	if c == nil || len(c.data) == 0 {
		return "", nil
	}
	b, err := json.Marshal(c.data)
	if err != nil {
		return "", fmt.Errorf("pagination: cursor encode: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Get retrieves a value from the cursor data.
func (c *Cursor) Get(key string) interface{} {
	if c == nil {
		return nil
	}
	return c.data[key]
}

// GetString retrieves a string value from the cursor data.
func (c *Cursor) GetString(key string) string {
	v := c.Get(key)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetInt retrieves an int value from the cursor data.
func (c *Cursor) GetInt(key string) int {
	v := c.Get(key)
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n) // JSON numbers are float64
	}
	return 0
}

// Decode decodes a base64url-encoded cursor token into a Cursor.
func Decode(token string) (*Cursor, error) {
	if token == "" {
		return &Cursor{data: map[string]interface{}{}}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("pagination: cursor decode: invalid base64: %w", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("pagination: cursor decode: invalid JSON: %w", err)
	}
	return &Cursor{data: data}, nil
}

// Page holds a page of results with a next cursor token.
type Page[T any] struct {
	Items      []T
	NextCursor string
	HasMore    bool
}

// NewPage creates a Page from a slice, limiting to pageSize and computing next cursor.
// cursorFn is called on the last element to produce the next cursor token.
func NewPage[T any](items []T, pageSize int, cursorFn func(T) (string, error)) (*Page[T], error) {
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	p := &Page[T]{Items: items, HasMore: hasMore}
	if hasMore && len(items) > 0 && cursorFn != nil {
		cursor, err := cursorFn(items[len(items)-1])
		if err != nil {
			return nil, fmt.Errorf("pagination: cursor generation: %w", err)
		}
		p.NextCursor = cursor
	}
	return p, nil
}
