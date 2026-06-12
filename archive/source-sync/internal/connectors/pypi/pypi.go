// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package pypi provides a source-sync connector for PyPI vulnerability data.
// Migrated from external/cmd/pypi/ — see external/README.md.
//
// This connector fetches CVE records affecting PyPI packages and converts them
// to OSV format for ingestion into the vulnerability database.
package pypi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Connector implements the source-sync connector interface for PyPI.
type Connector struct {
	linksURL    string
	versionsURL string
	httpClient  *http.Client
	log         zerolog.Logger
}

// Config holds PyPI connector configuration.
type Config struct {
	// LinksURL is the URL to download pypi_links.json
	// Default: https://osv-pypi-links.storage.googleapis.com/pypi_links.json
	LinksURL string
	// VersionsURL is the URL to download pypi_versions.json
	// Default: https://osv-pypi-links.storage.googleapis.com/pypi_versions.json
	VersionsURL string
}

// DefaultConfig returns connector config with default URLs.
func DefaultConfig() Config {
	return Config{
		LinksURL:    "https://osv-pypi-links.storage.googleapis.com/pypi_links.json",
		VersionsURL: "https://osv-pypi-links.storage.googleapis.com/pypi_versions.json",
	}
}

// NewConnector creates a new PyPI connector.
func NewConnector(cfg Config, log zerolog.Logger) *Connector {
	if cfg.LinksURL == "" {
		cfg = DefaultConfig()
	}
	return &Connector{
		linksURL:    cfg.LinksURL,
		versionsURL: cfg.VersionsURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		log: log,
	}
}

// Name returns the connector name.
func (c *Connector) Name() string { return "pypi" }

// FetchLinks downloads the PyPI links data used for package matching.
func (c *Connector) FetchLinks(ctx context.Context) ([]byte, error) {
	return c.fetchURL(ctx, c.linksURL)
}

// FetchVersions downloads the PyPI versions data.
func (c *Connector) FetchVersions(ctx context.Context) ([]byte, error) {
	return c.fetchURL(ctx, c.versionsURL)
}

func (c *Connector) fetchURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypi connector: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypi connector: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi connector: unexpected status %d from %s", resp.StatusCode, url)
	}

	buf := make([]byte, 0, resp.ContentLength)
	tmp := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}
