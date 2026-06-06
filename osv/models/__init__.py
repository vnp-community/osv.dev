# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""OSV models subpackage.

Re-exports all public symbols so existing code using `from osv import models`
or `osv.models.Bug` continues to work without changes.

Internal structure:
  entities.py  — NDB Model class definitions (Datastore schema)
  indexing.py  — Search indexing, affected version computation, GCS population
  (conversion logic remains in osv/models.py via Bug.to_vulnerability etc.)
"""

# Re-export all entity classes
from osv.models.entities import (
    # Validators / utils
    utcnow,
    _check_valid_severity,
    _check_valid_range_type,
    _check_valid_event_type,
    # OSS-Fuzz
    IDCounter,
    RegressResult,
    FixResult,
    # Core Bug
    AffectedEvent,
    AffectedRange2,
    SourceOfTruth,
    Package,
    Severity,
    AffectedPackage,
    Credit,
    # New-style Vulnerability
    Vulnerability,
    JobData,
    # Affected matching
    AffectedCommits,
    AffectedVersions,
    # Website listing
    ListedVulnerability,
    # Indexer
    RepoIndex,
    FileResult,
    RepoIndexBucket,
    # Source repository
    SourceRepositoryType,
    SourceRepository,
    # Groups
    AliasGroup,
    AliasAllowListEntry,
    AliasDenyListEntry,
    UpstreamGroup,
    RelatedGroup,
    # Import findings
    ImportFindings,
    ImportFinding,
)

# Re-export indexing helpers
from osv.models.indexing import (
    MIN_COARSE_VERSION,
    MAX_COARSE_VERSION,
    _tokenize,
    _EVENT_ORDER,
    build_listed_vulnerability,
    normalize_repo_package,
    affected_from_proto,
    diff_affected_versions,
    put_entities,
    populate_entities_from_bug,
)

__all__ = [
    # validators
    'utcnow',
    '_check_valid_severity',
    '_check_valid_range_type',
    '_check_valid_event_type',
    # OSS-Fuzz
    'IDCounter',
    'RegressResult',
    'FixResult',
    # Core
    'AffectedEvent',
    'AffectedRange2',
    'SourceOfTruth',
    'Package',
    'Severity',
    'AffectedPackage',
    'Credit',
    # Vulnerability
    'Vulnerability',
    'JobData',
    # Affected
    'AffectedCommits',
    'AffectedVersions',
    # Website
    'ListedVulnerability',
    # Indexer
    'RepoIndex',
    'FileResult',
    'RepoIndexBucket',
    # Source repo
    'SourceRepositoryType',
    'SourceRepository',
    # Groups
    'AliasGroup',
    'AliasAllowListEntry',
    'AliasDenyListEntry',
    'UpstreamGroup',
    'RelatedGroup',
    # Import
    'ImportFindings',
    'ImportFinding',
    # Indexing helpers
    'MIN_COARSE_VERSION',
    'MAX_COARSE_VERSION',
    '_tokenize',
    '_EVENT_ORDER',
    'build_listed_vulnerability',
    'normalize_repo_package',
    'affected_from_proto',
    'diff_affected_versions',
    'put_entities',
    'populate_entities_from_bug',
]
