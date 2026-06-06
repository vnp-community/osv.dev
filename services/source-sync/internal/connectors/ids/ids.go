// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package ids provides utilities for assigning OSV IDs to vulnerability records.
// Migrated from external/cmd/ids/ — see external/README.md for original implementation.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// fileFormat is the output format for vulnerability records.
type fileFormat string

const (
	// FileFormatJSON writes OSV records as JSON.
	FileFormatJSON = fileFormat("json")
	// FileFormatYAML writes OSV records as YAML.
	FileFormatYAML = fileFormat("yaml")

	conflictFile       = ".id-allocator"
	conflictMarkerSize = 32
)

// VulnRecord is a minimal vulnerability record for ID assignment.
type VulnRecord struct {
	ID      string `json:"id"`
	RawData any    `json:"-"` // original parsed content
}

// AllocateIDs scans dir for vulnerability files lacking IDs and assigns new ones
// using the given prefix. format must be FileFormatJSON or FileFormatYAML.
func AllocateIDs(prefix, dir string, format fileFormat) error {
	if prefix == "" || dir == "" {
		return fmt.Errorf("ids: prefix and dir are required")
	}
	if format != FileFormatJSON && format != FileFormatYAML {
		return fmt.Errorf("ids: unsupported format %q", format)
	}

	ext := map[fileFormat]string{
		FileFormatJSON: ".json",
		FileFormatYAML: ".yaml",
	}[format]

	maxID, err := findMaxID(dir, prefix)
	if err != nil {
		return fmt.Errorf("ids: find max ID: %w", err)
	}

	nextID := maxID + 1
	year := time.Now().Year()

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ext) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("ids: read %s: %w", path, err)
		}

		// Quick check — does file already have an "id" field?
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil // skip non-JSON / YAML files
		}
		if id, ok := raw["id"].(string); ok && id != "" {
			return nil // already has ID
		}

		id := fmt.Sprintf("%s-%d-%04d", prefix, year, nextID)
		nextID++
		raw["id"] = id

		updated, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return fmt.Errorf("ids: marshal %s: %w", path, err)
		}
		return os.WriteFile(path, updated, 0644)
	})
}

func findMaxID(dir, prefix string) (int, error) {
	max := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		if !strings.HasPrefix(name, prefix+"-") {
			return nil
		}
		parts := strings.Split(name, "-")
		if len(parts) < 3 {
			return nil
		}
		n, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil && n > max {
			max = n
		}
		return nil
	})
	return max, err
}

// GenerateConflictMarker creates a random hex string for conflict detection.
func GenerateConflictMarker() (string, error) {
	b := make([]byte, conflictMarkerSize/2)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CheckConflict returns true if the directory has an unresolved ID conflict.
func CheckConflict(dir string) (bool, error) {
	markerPath := filepath.Join(dir, conflictFile)
	data, err := os.ReadFile(markerPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "<<<<<"), nil
}
