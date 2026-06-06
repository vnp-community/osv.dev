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

// Package repository defines repository interfaces for the Source Sync Service.
package repository

import (
	"context"

	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
)

// SourceRepositoryRepo persists SourceRepository aggregates.
type SourceRepositoryRepo interface {
	// GetByName returns the SourceRepository with the given name.
	GetByName(ctx context.Context, name string) (*source_repository.SourceRepository, error)

	// Save persists (creates or updates) a SourceRepository.
	Save(ctx context.Context, source *source_repository.SourceRepository) error

	// List returns all registered source repositories.
	List(ctx context.Context) ([]*source_repository.SourceRepository, error)
}
