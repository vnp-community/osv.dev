// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package publisher provides NATS event publishing for the converter service.
package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"github.com/rs/zerolog"
)

// ConvertedEvent is the NATS event published after a successful conversion.
type ConvertedEvent struct {
	Source      string    `json:"source"`      // "cvelistV5", "nvd", "alpine", etc.
	Format      string    `json:"format"`      // "cve5", "nvd", "osv"
	VulnID      string    `json:"vuln_id"`
	OSVJSON     string    `json:"osv_json"`    // Serialized OSV vulnerability
	Checksum    string    `json:"checksum"`    // SHA256 of raw input for dedup
	ConvertedAt time.Time `json:"converted_at"`
}

// SubjectForFormat returns the NATS subject for a given input format.
func SubjectForFormat(format string) string {
	switch format {
	case "cve5":
		return "raw.cve.cve5"
	case "nvd":
		return "raw.cve.nvd"
	case "alpine":
		return "raw.cve.alpine"
	case "osv":
		return "raw.cve.osv"
	default:
		return "raw.cve.unknown"
	}
}

// NATSPublisher publishes ConvertedEvent messages to NATS.
type NATSPublisher struct {
	nc  *natsgo.Conn
	log zerolog.Logger
}

// NewNATSPublisher creates a publisher backed by a NATS connection.
func NewNATSPublisher(nc *natsgo.Conn, log zerolog.Logger) *NATSPublisher {
	return &NATSPublisher{nc: nc, log: log}
}

// Publish serializes the converted vulnerability and publishes it to NATS.
// The subject is derived from the format field.
func (p *NATSPublisher) Publish(_ context.Context, vuln *osvschema.Vulnerability, source, format string, rawInput []byte) error {
	osvJSON, err := json.Marshal(vuln)
	if err != nil {
		return fmt.Errorf("marshal osv: %w", err)
	}

	checksum := fmt.Sprintf("sha256:%x", sha256.Sum256(rawInput))

	event := ConvertedEvent{
		Source:      source,
		Format:      format,
		VulnID:      vuln.Id,
		OSVJSON:     string(osvJSON),
		Checksum:    checksum,
		ConvertedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	subject := SubjectForFormat(format)
	if err := p.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("nats publish to %s: %w", subject, err)
	}

	p.log.Debug().
		Str("subject", subject).
		Str("vuln_id", vuln.Id).
		Str("checksum", checksum).
		Msg("converted event published")

	return nil
}

// PublishBatch publishes multiple converted vulnerabilities.
// Non-fatal errors are collected and returned after all events are attempted.
func (p *NATSPublisher) PublishBatch(ctx context.Context, vulns []*osvschema.Vulnerability, source, format string, rawInputs [][]byte) []error {
	var errs []error
	for i, v := range vulns {
		var rawInput []byte
		if i < len(rawInputs) {
			rawInput = rawInputs[i]
		}
		if err := p.Publish(ctx, v, source, format, rawInput); err != nil {
			errs = append(errs, fmt.Errorf("publish %s: %w", v.Id, err))
		}
	}
	return errs
}
