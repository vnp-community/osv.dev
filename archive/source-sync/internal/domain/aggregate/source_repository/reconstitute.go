// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package source_repository

import (
	"github.com/osv/source-sync/internal/domain/valueobject"
)

// ReconstitueFromStore rebuilds a SourceRepository aggregate from stored data.
// Used by the Firestore repository to reconstruct domain objects.
func ReconstitueFromStore(
	name, sourceType, repoURL, bucket, restAPIURL, dirPath, ext, lastHash string,
	strictValidation bool,
) *SourceRepository {
	return &SourceRepository{
		name:             name,
		sourceType:       valueobject.SourceType(sourceType),
		repoURL:          repoURL,
		bucket:           bucket,
		restAPIURL:       restAPIURL,
		directoryPath:    dirPath,
		extension:        ext,
		lastSyncedHash:   lastHash,
		strictValidation: strictValidation,
	}
}

// BucketName returns the GCS bucket name (alias for Bucket() for backward compat).
func (s *SourceRepository) BucketName() string { return s.bucket }

// RESTURL returns the REST API URL (alias for RESTAPIURL() for backward compat).
func (s *SourceRepository) RESTURL() string { return s.restAPIURL }

// StrictValidation returns whether strict validation is enabled.
func (s *SourceRepository) StrictValidation() bool { return s.strictValidation }
