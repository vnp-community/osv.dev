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

package source_repository_test

import (
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/osv/pkg/errors"
	"github.com/osv/source-sync/internal/domain/aggregate/source_repository"
	"github.com/osv/source-sync/internal/domain/valueobject"
)

func TestCheckDeletionSafety_BelowThreshold(t *testing.T) {
	sr, _ := source_repository.NewSourceRepository("ghsa", valueobject.SourceTypeGit)
	err := sr.CheckDeletionSafety(5, 100)
	if err != nil {
		t.Errorf("expected no error for 5%%, got: %v", err)
	}
}

func TestCheckDeletionSafety_AtThreshold(t *testing.T) {
	sr, _ := source_repository.NewSourceRepository("ghsa", valueobject.SourceTypeGit)
	err := sr.CheckDeletionSafety(10, 100)
	if err == nil {
		t.Fatal("expected error at 10% threshold")
	}
	var safetyErr *pkgerrors.DeletionSafetyError
	if !errors.As(err, &safetyErr) {
		t.Errorf("expected DeletionSafetyError, got %T", err)
	}
}

func TestCheckDeletionSafety_AboveThreshold(t *testing.T) {
	sr, _ := source_repository.NewSourceRepository("nvd", valueobject.SourceTypeBucket)
	err := sr.CheckDeletionSafety(50, 100)
	if err == nil {
		t.Fatal("expected error for 50% deletion")
	}
}

func TestCheckDeletionSafety_EmptySource(t *testing.T) {
	sr, _ := source_repository.NewSourceRepository("new", valueobject.SourceTypeGit)
	// totalCount=0: should not panic or error
	err := sr.CheckDeletionSafety(0, 0)
	if err != nil {
		t.Errorf("expected no error for empty source, got: %v", err)
	}
}

func TestMarkSynced(t *testing.T) {
	sr, _ := source_repository.NewSourceRepository("ghsa", valueobject.SourceTypeGit)
	if sr.LastSyncedHash() != "" {
		t.Error("expected empty hash before sync")
	}
	sr.MarkSynced("abc123", time.Now())
	if sr.LastSyncedHash() != "abc123" {
		t.Errorf("hash: got %q, want %q", sr.LastSyncedHash(), "abc123")
	}
}
