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

// Package entity contains entities for the Ingestion Service.
package entity

import "time"

// ImportFinding records a data quality issue found during import.
// Stored in Firestore at: import-findings/{source}/{bug_id}
type ImportFinding struct {
	BugID       string    `firestore:"bug_id"`
	Source      string    `firestore:"source"`
	FindingType string    `firestore:"finding_type"` // ImportFindingType
	Message     string    `firestore:"message"`
	FirstSeen   time.Time `firestore:"first_seen"`
	LastAttempt time.Time `firestore:"last_attempt"`
	Count       int       `firestore:"count"`
}
