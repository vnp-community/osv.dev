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

// Package errors defines domain error types shared across OSV microservices.
package errors

import "fmt"

// Sentinel errors for domain-level error classification.
var (
	ErrNotFound         = fmt.Errorf("not found")
	ErrAlreadyExists    = fmt.Errorf("already exists")
	ErrValidation       = fmt.Errorf("validation error")
	ErrUnknownEcosystem = fmt.Errorf("unknown ecosystem")
	ErrStaleResult      = fmt.Errorf("stale result")
)

// DeletionSafetyError is returned when a bulk deletion would exceed the safety
// threshold (default 10% of total records).
type DeletionSafetyError struct {
	Message        string
	ToDeleteCount  int
	TotalCount     int
	ThresholdPct   float64
}

func (e *DeletionSafetyError) Error() string {
	return fmt.Sprintf("deletion safety: %s (would delete %d/%d = %.1f%%, threshold %.1f%%)",
		e.Message, e.ToDeleteCount, e.TotalCount,
		float64(e.ToDeleteCount)/float64(e.TotalCount)*100,
		e.ThresholdPct)
}

// NewDeletionSafetyError constructs a DeletionSafetyError.
func NewDeletionSafetyError(toDelete, total int, threshold float64) *DeletionSafetyError {
	return &DeletionSafetyError{
		Message:       "bulk deletion blocked",
		ToDeleteCount: toDelete,
		TotalCount:    total,
		ThresholdPct:  threshold,
	}
}

// ValidationError is returned when input fails validation.
type ValidationError struct {
	Field   string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on field %q: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

func (e *ValidationError) Unwrap() error { return e.Cause }

// Is supports errors.Is(err, ErrValidation) matching.
func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}

// NewValidationError constructs a ValidationError.
func NewValidationError(field, message string, cause error) *ValidationError {
	return &ValidationError{Field: field, Message: message, Cause: cause}
}
