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

// Package repository defines the repository interfaces for the Ingestion Service.
package repository

import (
	"context"

	"github.com/osv/ingestion/internal/domain/aggregate/vulnerability"
	"github.com/osv/ingestion/internal/domain/entity"
)

// VulnerabilityWriter persists vulnerability aggregates to the write store.
type VulnerabilityWriter interface {
	// Upsert creates or updates a vulnerability in Firestore.
	// The read model projection is derived from the aggregate.
	Upsert(ctx context.Context, agg *vulnerability.VulnerabilityAggregate) error

	// GetByID retrieves a vulnerability aggregate by ID.
	// Returns pkg/errors.ErrNotFound if not found.
	GetByID(ctx context.Context, id string) (*vulnerability.VulnerabilityAggregate, error)
}

// ImportFindingRepo persists import quality findings.
type ImportFindingRepo interface {
	// Save persists an ImportFinding record.
	Save(ctx context.Context, finding *entity.ImportFinding) error

	// ListBySource returns all findings for the given source.
	ListBySource(ctx context.Context, source string, limit int) ([]*entity.ImportFinding, error)
}
