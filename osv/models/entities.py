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
"""NDB Datastore entity (Model) definitions for OSV.

This module contains all Google Cloud Datastore entity classes. Each class
maps to a Datastore *kind*. Business logic and proto conversion live in
sibling modules (conversion.py, indexing.py).
"""

import datetime
import enum
import os
import re

from google.cloud import ndb


# ---------------------------------------------------------------------------
# Shared validators (used as ndb property validators)
# ---------------------------------------------------------------------------

def _check_valid_severity(prop, value):
  """Check valid severity."""
  del prop
  if value not in ('NEGLIGIBLE', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'):
    raise ValueError('Invalid severity: ' + value)


def _check_valid_range_type(prop, value):
  """Check valid range type."""
  del prop
  if value not in ('GIT', 'SEMVER', 'ECOSYSTEM'):
    raise ValueError('Invalid range type: ' + value)


def _check_valid_event_type(prop, value):
  """Check valid event type."""
  del prop
  if value not in ('introduced', 'fixed', 'last_affected', 'limit'):
    raise ValueError('Invalid event type: ' + value)


def utcnow() -> datetime.datetime:
  """Return current UTC time. Extracted for mocking in tests."""
  return datetime.datetime.now(datetime.UTC)


# ---------------------------------------------------------------------------
# OSS-Fuzz result entities
# ---------------------------------------------------------------------------


class IDCounter(ndb.Model):
  """Counter for ID allocations."""
  next_id: int = ndb.IntegerProperty()


class RegressResult(ndb.Model):
  """Regression results."""
  commit: str = ndb.StringProperty(default='')
  summary: str = ndb.TextProperty()
  details: str = ndb.TextProperty()
  error: str = ndb.StringProperty()
  issue_id: str = ndb.StringProperty()
  project: str = ndb.StringProperty()
  ecosystem: str = ndb.StringProperty()
  repo_url: str = ndb.StringProperty()
  severity: str = ndb.StringProperty(validator=_check_valid_severity)
  reference_urls: list[str] = ndb.StringProperty(repeated=True)
  timestamp: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)


class FixResult(ndb.Model):
  """Fix results."""
  commit: str = ndb.StringProperty(default='')
  summary: str = ndb.TextProperty()
  details: str = ndb.TextProperty()
  error: str = ndb.StringProperty()
  issue_id: str = ndb.StringProperty()
  project: str = ndb.StringProperty()
  ecosystem: str = ndb.StringProperty()
  repo_url: str = ndb.StringProperty()
  severity: str = ndb.StringProperty(validator=_check_valid_severity)
  reference_urls: list[str] = ndb.StringProperty(repeated=True)
  timestamp: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)


# ---------------------------------------------------------------------------
# Core OSV Bug entities
# ---------------------------------------------------------------------------


class AffectedEvent(ndb.Model):
  """A single version event (introduced / fixed / last_affected / limit)."""
  type: str = ndb.StringProperty(validator=_check_valid_event_type)
  value: str = ndb.StringProperty()


class AffectedRange2(ndb.Model):
  """Affected version range for a package."""
  type: str = ndb.StringProperty(validator=_check_valid_range_type)
  repo_url: str = ndb.StringProperty()
  events: list[AffectedEvent] = ndb.LocalStructuredProperty(
      AffectedEvent, repeated=True)
  database_specific: dict = ndb.JsonProperty()


class SourceOfTruth(enum.IntEnum):
  """Source of truth for a Bug."""
  NONE = 0
  INTERNAL = 1       # Internal to OSV (e.g. private OSS-Fuzz bugs).
  SOURCE_REPO = 2    # Vulnerabilities available in a public repo.


class Package(ndb.Model):
  """Package identifier."""
  ecosystem: str = ndb.StringProperty()
  name: str = ndb.StringProperty()
  purl: str = ndb.StringProperty()


class Severity(ndb.Model):
  """Severity score entry."""
  type: str = ndb.StringProperty()
  score: str = ndb.StringProperty()


class AffectedPackage(ndb.Model):
  """Affected package with its ranges and versions."""
  package: Package = ndb.StructuredProperty(Package)
  ranges: list[AffectedRange2] = ndb.LocalStructuredProperty(
      AffectedRange2, repeated=True)
  versions: list[str] = ndb.TextProperty(repeated=True)
  database_specific: dict = ndb.JsonProperty()
  ecosystem_specific: dict = ndb.JsonProperty()
  severities: list[Severity] = ndb.LocalStructuredProperty(
      Severity, repeated=True)


class Credit(ndb.Model):
  """Credit entry."""
  name: str = ndb.StringProperty()
  contact: list[str] = ndb.StringProperty(repeated=True)
  type: str = ndb.StringProperty()


# ---------------------------------------------------------------------------
# Vulnerability entities (new-style, decoupled from Bug)
# ---------------------------------------------------------------------------


class Vulnerability(ndb.Model):
  """A lightweight Vulnerability entry for the API.

  Contains minimal information of an OSV record: the overall modified date
  and some raw fields that are overwritten by enrichment workers.
  The entity key/id is the OSV ID.
  """
  source_id: str = ndb.StringProperty()
  modified: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)
  is_withdrawn: bool = ndb.BooleanProperty()
  modified_raw: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)
  alias_raw: list[str] = ndb.StringProperty(repeated=True)
  related_raw: list[str] = ndb.StringProperty(repeated=True)
  upstream_raw: list[str] = ndb.StringProperty(repeated=True)


class JobData(ndb.Expando):
  """Generic job data (key-value expando)."""


# ---------------------------------------------------------------------------
# Affected version matching entities
# ---------------------------------------------------------------------------

class AffectedCommits(ndb.Model):
  """Affected Git commits for a vulnerability."""
  MAX_COMMITS_PER_ENTITY = 10000

  bug_id: str = ndb.StringProperty()
  commits: list[bytes] = ndb.BlobProperty(repeated=True, indexed=True)
  public: bool = ndb.BooleanProperty()
  page: int = ndb.IntegerProperty(indexed=False)


class AffectedVersions(ndb.Model):
  """AffectedVersions — used for API version-matching queries."""
  vuln_id: str = ndb.StringProperty()
  ecosystem: str = ndb.StringProperty()
  name: str = ndb.StringProperty()
  versions: list[str] = ndb.TextProperty(repeated=True)
  events: list[AffectedEvent] = ndb.LocalStructuredProperty(
      AffectedEvent, repeated=True)
  # Coarse version bounds for pre-filtering.
  coarse_min: str = ndb.StringProperty()
  coarse_max: str = ndb.StringProperty()

  def sort_key(self):
    """Key for comparison and deduplication."""
    return (self.vuln_id, self.ecosystem, self.name, tuple(self.versions),
            tuple((e.type, e.value) for e in self.events))


# ---------------------------------------------------------------------------
# Website listing entity
# ---------------------------------------------------------------------------


class ListedVulnerability(ndb.Model):
  """ListedVulnerability — used for the /list page on the website."""
  published: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)
  ecosystems: list[str] = ndb.StringProperty(repeated=True)
  packages: list[str] = ndb.TextProperty(repeated=True)
  summary: str = ndb.TextProperty()
  is_fixed: bool = ndb.BooleanProperty(indexed=False)
  severities: list[Severity] = ndb.LocalStructuredProperty(
      Severity, repeated=True)
  autocomplete_tags: list[str] = ndb.StringProperty(repeated=True)
  search_indices: list[str] = ndb.StringProperty(repeated=True)
  _MAX_INDICES = 2000


# ---------------------------------------------------------------------------
# Indexer entities
# ---------------------------------------------------------------------------


class RepoIndex(ndb.Model):
  """RepoIndex entry."""
  name: str = ndb.StringProperty()
  base_cpe: str = ndb.StringProperty()
  commit: bytes = ndb.BlobProperty()
  repo_addr: str = ndb.StringProperty()
  file_exts: list[str] = ndb.StringProperty(repeated=True)
  file_hash_type: str = ndb.StringProperty()
  repo_type: str = ndb.StringProperty()
  empty_bucket_bitmap: bytes = ndb.BlobProperty()
  file_count: int = ndb.IntegerProperty()
  tag: str = ndb.StringProperty()


class FileResult(ndb.Model):
  """FileResult — path and hash of a single file."""
  hash: bytes = ndb.BlobProperty(indexed=True)
  path: str = ndb.TextProperty()


class RepoIndexBucket(ndb.Model):
  """RepoIndexBucket — aggregated hash values."""
  node_hash: bytes = ndb.BlobProperty(indexed=True)
  files_contained: int = ndb.IntegerProperty()


# ---------------------------------------------------------------------------
# Source repository configuration
# ---------------------------------------------------------------------------


class SourceRepositoryType(enum.IntEnum):
  """SourceRepository type."""
  GIT = 0
  BUCKET = 1
  REST_ENDPOINT = 2


class SourceRepository(ndb.Model):
  """Source repository configuration."""
  type: int = ndb.IntegerProperty()
  name: str = ndb.StringProperty()
  repo_url: str = ndb.StringProperty()
  repo_username: str = ndb.StringProperty()
  repo_branch: str = ndb.StringProperty()
  rest_api_url: str = ndb.StringProperty()
  bucket: str = ndb.StringProperty()
  directory_path: str = ndb.StringProperty()
  last_synced_hash: str = ndb.StringProperty()
  last_update_date: datetime.datetime = ndb.DateTimeProperty(
      tzinfo=datetime.UTC)
  ignore_patterns: list[str] = ndb.StringProperty(repeated=True)
  editable: bool = ndb.BooleanProperty(default=False)
  extension: str = ndb.StringProperty(default='.yaml')
  key_path: str = ndb.StringProperty()
  ignore_git: bool = ndb.BooleanProperty(default=False)
  accepted_ecosystems: list[str] = ndb.StringProperty(repeated=True)
  detect_cherrypicks: bool = ndb.BooleanProperty(default=True)
  consider_all_branches: bool = ndb.BooleanProperty(default=False)
  versions_from_repo: bool = ndb.BooleanProperty(default=True)
  ignore_last_import_time: bool = ndb.BooleanProperty(default=False)
  ignore_deletion_threshold: bool = ndb.BooleanProperty(default=False)
  link: str = ndb.StringProperty()
  human_link: str = ndb.StringProperty()
  db_prefix: list[str] = ndb.StringProperty(repeated=True)
  strict_validation: bool = ndb.BooleanProperty(default=False)
  work_pool: str = ndb.StringProperty()

  def ignore_file(self, file_path):
    """Return whether or not we should be ignoring a file."""
    if not self.ignore_patterns:
      return False
    file_name = os.path.basename(file_path)
    for pattern in self.ignore_patterns:
      if re.match(pattern, file_name):
        return True
    return False

  def _pre_put_hook(self):  # pylint: disable=arguments-differ
    """Pre-put hook for validation."""
    if self.type == SourceRepositoryType.BUCKET and self.editable:
      raise ValueError('BUCKET SourceRepository cannot be editable.')


# ---------------------------------------------------------------------------
# Alias, Upstream, Related groups
# ---------------------------------------------------------------------------


class AliasGroup(ndb.Model):
  """Group of vulnerability IDs that are aliases of each other."""
  bug_ids: list[str] = ndb.StringProperty(repeated=True)
  last_modified: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)


class AliasAllowListEntry(ndb.Model):
  """Alias group allow list entry."""
  bug_id: str = ndb.StringProperty()


class AliasDenyListEntry(ndb.Model):
  """Alias group deny list entry."""
  bug_id: str = ndb.StringProperty()


class UpstreamGroup(ndb.Model):
  """Upstream group for storing transitive upstreams of a Bug.

  Kept separate from Bug to prevent race conditions — only the worker
  modifies Bug directly.
  """
  db_id: str = ndb.StringProperty()
  upstream_ids: list[str] = ndb.StringProperty(repeated=True)
  upstream_hierarchy: str = ndb.JsonProperty()
  last_modified: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)


class RelatedGroup(ndb.Model):
  """Related group for storing related IDs of a Vulnerability."""
  related_ids: list[str] = ndb.StringProperty(repeated=True)
  modified: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)


# ---------------------------------------------------------------------------
# Import quality findings
# ---------------------------------------------------------------------------


class ImportFindings(enum.IntEnum):
  """Quality findings about an individual record."""
  UNKNOWN = -1
  NONE = 0
  DELETED = 1
  INVALID_JSON = 2
  INVALID_PACKAGE = 3
  INVALID_PURL = 4
  INVALID_VERSION = 5
  INVALID_COMMIT = 6
  INVALID_RANGE = 7
  INVALID_RECORD = 8
  INVALID_ALIASES = 9
  INVALID_UPSTREAM = 10
  INVALID_RELATED = 11
  BAD_ALIASED_CVE = 12


class ImportFinding(ndb.Model):
  """Quality findings about an individual record."""
  bug_id: str = ndb.StringProperty()
  source: str = ndb.StringProperty()
  findings: list[ImportFindings] = ndb.IntegerProperty(repeated=True)
  first_seen: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)
  last_attempt: datetime.datetime = ndb.DateTimeProperty(tzinfo=datetime.UTC)

  def _pre_put_hook(self):  # pylint: disable=arguments-differ
    """Pre-put hook for setting key."""
    if not self.key:  # pylint: disable=access-member-before-definition
      self.key = ndb.Key(ImportFinding, self.bug_id)

  def to_proto(self):
    """Converts to ImportFinding proto."""
    from osv import importfinding_pb2  # lazy import to avoid circular deps
    return importfinding_pb2.ImportFinding(
        bug_id=self.bug_id,
        source=self.source,
        findings=self.findings,  # type: ignore
        first_seen=self.first_seen.timestamp_pb(),  # type: ignore
        last_attempt=self.last_attempt.timestamp_pb(),  # type: ignore
    )
