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
"""Search indexing, affected version computation, and entity population.

This module contains:
  - _tokenize(): builds search index tokens from an ID/name
  - ListedVulnerability.from_vulnerability(): classmethod that builds the
    website listing entity from a Vulnerability proto
  - affected_from_proto() / diff_affected_versions(): compute AffectedVersions
    entities from a proto
  - populate_entities_from_bug(): the main hook called after Bug.put()
  - normalize_repo_package(): strips scheme/suffix from repo URLs
"""

import logging
import re
from urllib.parse import urlparse

from google.cloud import ndb

from osv import ecosystems
from osv import gcs
from osv import pubsub
from osv import vulnerability_pb2
from osv.models.entities import (
    AffectedEvent,
    AffectedVersions,
    ListedVulnerability,
    Severity,
    Vulnerability,
)

# Coarse version sentinel constants (imported from ecosystems helper)
from osv.ecosystems.ecosystems_base import (
    coarse_version_from_ints,
    MAX_COARSE_PART,
    MAX_COARSE_EPOCH,
)

MIN_COARSE_VERSION = coarse_version_from_ints((0,), 0)
MAX_COARSE_VERSION = coarse_version_from_ints((MAX_COARSE_PART + 1,),
                                              MAX_COARSE_EPOCH + 1)

_EVENT_ORDER = {
    'introduced': 0,
    'last_affected': 1,
    'fixed': 2,
    'limit': 3,
}


# ---------------------------------------------------------------------------
# Tokenization
# ---------------------------------------------------------------------------


def _tokenize(value: str) -> set[str]:
  """Tokenize value for search indexing.

  Decomposes the value into alphanumeric tokens and hyphen-delimited
  sub-sequences so that partial IDs (e.g. searching 'CVE-123' finds
  'UBUNTU-CVE-123').
  """
  if not value:
    return set()

  value_lower = value.lower()
  tokens = {token for token in re.split(r'[^a-zA-Z0-9]+', value_lower) if token}
  tokens.add(value_lower)

  # Add contiguous sub-sequences of hyphen-split parts.
  # e.g. 'a-b-c-d' → ['a-b', 'b-c', 'c-d', 'a-b-c', 'b-c-d', 'a-b-c-d']
  parts = value_lower.split('-')
  num_parts = len(parts)
  for length in range(2, num_parts + 1):
    for i in range(num_parts - length + 1):
      tokens.add('-'.join(parts[i:i + length]))

  return tokens


# ---------------------------------------------------------------------------
# ListedVulnerability construction
# ---------------------------------------------------------------------------


def build_listed_vulnerability(
    vulnerability: vulnerability_pb2.Vulnerability) -> ListedVulnerability:
  """Construct a ListedVulnerability from a complete Vulnerability proto.

  Extracted from ListedVulnerability.from_vulnerability() for easier testing.
  """
  import datetime  # local import to avoid top-level cost
  published = vulnerability.published.ToDatetime(datetime.UTC)
  summary = vulnerability.summary

  all_ecosystems: set[str] = set()
  all_packages: set[str] = set()
  is_fixed = False
  severities: set[tuple[str, str]] = set()

  for sev in vulnerability.severity:
    severities.add(
        (vulnerability_pb2.Severity.Type.Name(sev.type), sev.score))

  search_indices: set[str] = set()
  search_indices.update(_tokenize(vulnerability.id))
  pkg_search_indices: set[str] = set()
  repo_search_indices: set[str] = set()
  autocomplete_tags: set[str] = {vulnerability.id.lower()}

  for affected in vulnerability.affected:
    if affected.package.name:
      pkg_search_indices.add(affected.package.name.lower())
      autocomplete_tags.add(affected.package.name.lower())
      all_packages.add(affected.package.ecosystem + '/' +
                       affected.package.name)
    if affected.package.ecosystem:
      all_ecosystems.add(affected.package.ecosystem)
    for sev in affected.severity:
      severities.add(
          (vulnerability_pb2.Severity.Type.Name(sev.type), sev.score))
    for r in affected.ranges:
      if r.type == vulnerability_pb2.Range.Type.GIT:
        all_ecosystems.add('GIT')
        repo_search_indices.add(r.repo.lower())
        autocomplete_tags.add(r.repo.lower())
        split = r.repo.lower().split('//')
        if len(split) >= 2:
          all_packages.add(split[1])
        else:
          all_packages.add(r.repo.lower())
      if any(e.fixed or e.limit for e in r.events):
        is_fixed = True

  max_indices = ListedVulnerability._MAX_INDICES

  # Exact matches first, then tokenized
  search_indices.update(repo_search_indices)
  search_indices.update(pkg_search_indices)
  for repo in repo_search_indices:
    if len(search_indices) >= max_indices:
      break
    split = repo.split('//')
    if len(split) >= 2:
      no_http = split[1]
      search_indices.add(no_http)
      for part in no_http.split('/')[1:]:
        if len(search_indices) >= max_indices:
          break
        search_indices.add(part)

  for pkg in pkg_search_indices:
    if len(search_indices) >= max_indices:
      break
    for t in _tokenize(pkg):
      if len(search_indices) >= max_indices:
        break
      search_indices.add(t)

  for eco in all_ecosystems:
    for t in _tokenize(eco):
      if len(search_indices) >= max_indices:
        break
      search_indices.add(t)
    if (e := ecosystems.remove_variants(eco)) is not None:
      for t in _tokenize(e):
        if len(search_indices) >= max_indices:
          break
        search_indices.add(t)

  for alias in vulnerability.aliases:
    for t in _tokenize(alias):
      if len(search_indices) >= max_indices:
        break
      search_indices.add(t)
  for upstream in vulnerability.upstream:
    for t in _tokenize(upstream):
      if len(search_indices) >= max_indices:
        break
      search_indices.add(t)

  ecos = sorted({ecosystems.normalize(e) for e in all_ecosystems})
  pkgs = sorted(all_packages)
  sevs = [Severity(type=t, score=s) for t, s in sorted(severities)]
  search_indices_list = sorted(search_indices)[:max_indices]
  autocomplete_tags_list = sorted(autocomplete_tags)[:max_indices]

  return ListedVulnerability(
      id=vulnerability.id,
      published=published,
      ecosystems=ecos,
      packages=pkgs,
      summary=summary,
      is_fixed=is_fixed,
      severities=sevs,
      autocomplete_tags=autocomplete_tags_list,
      search_indices=search_indices_list,
  )


# ---------------------------------------------------------------------------
# Repo URL normalization
# ---------------------------------------------------------------------------


def normalize_repo_package(repo_url: str) -> str:
  """Normalize repo_url for GIT AffectedVersions entities.

  Strips scheme (http/https/git), .git suffix, and trailing slashes so that
  URLs pointing to the same repo compare equal.

  Examples:
    'http://git.musl-libc.org/git/musl'  → 'git.musl-libc.org/git/musl'
    'https://github.com/curl/curl.git'   → 'github.com/curl/curl'
  """
  if not repo_url:
    return repo_url
  try:
    parsed = urlparse(repo_url)
    normalized = parsed.netloc + parsed.path
    normalized = normalized.rstrip('/')
    normalized = normalized.removesuffix('.git')
    return normalized
  except Exception:  # pylint: disable=broad-except
    return repo_url


# ---------------------------------------------------------------------------
# AffectedVersions computation
# ---------------------------------------------------------------------------


def _get_coarse_min_max(events: list[AffectedEvent],
                        e_helper,
                        db_id: str) -> tuple[str, str]:
  """Get coarse min and max from sorted events."""
  coarse_min = MIN_COARSE_VERSION
  coarse_max = MAX_COARSE_VERSION
  try:
    for e in events:
      if e.type == 'introduced':
        coarse_min = e_helper.coarse_version(e.value)
        last = events[-1]
        if last.type != 'introduced':
          coarse_max = e_helper.coarse_version(last.value)
        break
  except NotImplementedError:
    pass
  except ValueError:
    logging.warning('Invalid version in %s %s', db_id, events)
    coarse_min = MIN_COARSE_VERSION
    coarse_max = MAX_COARSE_VERSION
  return coarse_min, coarse_max


def _affected_versions_from_affected_proto(
    affected: vulnerability_pb2.Affected,
    db_id: str) -> list[AffectedVersions]:
  """Compute AffectedVersions for a single affected package."""
  affected_versions = []
  pkg_ecosystem = affected.package.ecosystem
  all_pkg_ecosystems = {pkg_ecosystem, ecosystems.normalize(pkg_ecosystem)}
  if (e := ecosystems.remove_variants(pkg_ecosystem)) is not None:
    all_pkg_ecosystems.add(e)

  pkg_name = ecosystems.maybe_normalize_package_names(affected.package.name,
                                                      pkg_ecosystem)
  e_helper = ecosystems.get(pkg_ecosystem)
  repo_url = ''
  pkg_has_affected = False

  for r in affected.ranges:
    if r.type == vulnerability_pb2.Range.Type.GIT:
      if not repo_url:
        repo_url = r.repo
      continue
    if r.type not in (vulnerability_pb2.Range.Type.SEMVER,
                      vulnerability_pb2.Range.Type.ECOSYSTEM):
      logging.warning('Unknown range type "%d" in %s', r.type, db_id)
      continue
    if not r.events:
      continue

    events = []
    for e in r.events:
      if e.introduced:
        events.append(AffectedEvent(type='introduced', value=e.introduced))
      elif e.fixed:
        events.append(AffectedEvent(type='fixed', value=e.fixed))
      elif e.limit:
        events.append(AffectedEvent(type='limit', value=e.limit))
      elif e.last_affected:
        events.append(
            AffectedEvent(type='last_affected', value=e.last_affected))

    pkg_has_affected = True
    coarse_min = MIN_COARSE_VERSION
    coarse_max = MAX_COARSE_VERSION
    if e_helper is not None:
      events.sort(key=lambda e, sort_key=e_helper.sort_key:
                  (sort_key(e.value), _EVENT_ORDER.get(e.type, -1)))
      coarse_min, coarse_max = _get_coarse_min_max(events, e_helper, db_id)

    for e in all_pkg_ecosystems:
      affected_versions.append(
          AffectedVersions(
              vuln_id=db_id,
              ecosystem=e,
              name=pkg_name,
              coarse_min=coarse_min,
              coarse_max=coarse_max,
              events=events,
          ))

  if pkg_name and affected.versions:
    pkg_has_affected = True
    coarse_min = MIN_COARSE_VERSION
    coarse_max = MAX_COARSE_VERSION
    if e_helper is not None:
      try:
        all_coarse = [e_helper.coarse_version(v) for v in affected.versions]
        coarse_min = min(all_coarse)
        coarse_max = max(all_coarse)
      except NotImplementedError:
        pass
      except ValueError:
        logging.warning('Invalid version in %s', db_id)
    for e in all_pkg_ecosystems:
      affected_versions.append(
          AffectedVersions(
              vuln_id=db_id,
              ecosystem=e,
              name=pkg_name,
              versions=list(affected.versions),
              coarse_min=coarse_min,
              coarse_max=coarse_max,
          ))

  if pkg_name and not pkg_has_affected:
    logging.warning('Vuln has empty affected ranges and versions: %s, %s/%s',
                    db_id, pkg_ecosystem, pkg_name)
    for e in all_pkg_ecosystems:
      affected_versions.append(
          AffectedVersions(vuln_id=db_id, ecosystem=e, name=pkg_name))

  if repo_url:
    affected_versions.append(
        AffectedVersions(
            vuln_id=db_id,
            ecosystem='GIT',
            name=normalize_repo_package(repo_url),
            versions=list(affected.versions),
        ))

  return affected_versions


def affected_from_proto(
    vuln_pb: vulnerability_pb2.Vulnerability) -> list[AffectedVersions]:
  """Compute the AffectedVersions from a Vulnerability proto."""
  affected_versions = []
  for affected in vuln_pb.affected:
    affected_versions.extend(
        _affected_versions_from_affected_proto(affected, vuln_pb.id))

  unique_affected_dict = {av.sort_key(): av for av in affected_versions}
  return sorted(unique_affected_dict.values(), key=AffectedVersions.sort_key)


def diff_affected_versions(
    old: list[AffectedVersions],
    new: list[AffectedVersions],
) -> tuple[list[AffectedVersions], list[AffectedVersions]]:
  """Find entities added/removed from `old` to reach `new` (ignoring IDs).

  Returns:
    (added, removed) — lists of AffectedVersions entities.
  """
  all_dict = {av.sort_key(): av for av in old + new}
  old_set = {av.sort_key() for av in old}
  new_set = {av.sort_key() for av in new}
  added = [all_dict[k] for k in new_set - old_set]
  removed = [all_dict[k] for k in old_set - new_set]
  return added, removed


# ---------------------------------------------------------------------------
# Entity population (called from Bug._post_put_hook)
# ---------------------------------------------------------------------------


def put_entities(ds_vuln: Vulnerability,
                 vuln_pb: vulnerability_pb2.Vulnerability) -> None:
  """Put Vulnerability, ListedVulnerability, and AffectedVersions to Datastore.

  Does NOT write to GCS. Call populate_entities_from_bug() for the full flow.
  """
  to_put = [ds_vuln]
  to_delete = []
  old_affected = AffectedVersions.query(
      AffectedVersions.vuln_id == vuln_pb.id).fetch()

  if ds_vuln.is_withdrawn:
    to_delete.append(ndb.Key(ListedVulnerability, vuln_pb.id))
    to_delete.extend(av.key for av in old_affected)
  else:
    to_put.append(build_listed_vulnerability(vuln_pb))
    new_affected = affected_from_proto(vuln_pb)
    added, removed = diff_affected_versions(old_affected, new_affected)
    to_put.extend(added)
    to_delete.extend(r.key for r in removed)

  ndb.put_multi(to_put)
  ndb.delete_multi(to_delete)


def populate_entities_from_bug(entity) -> None:
  """Write Datastore entities and GCS blob for a Bug after put().

  This is the main entry point called from Bug._post_put_hook.
  Skips private/unprocessed OSS-Fuzz bugs.
  """
  from osv import bug as bug_module  # lazy to avoid circular import
  if not entity.public or entity.status == bug_module.BugStatus.UNPROCESSED:
    return

  vuln_pb = entity.to_vulnerability(
      include_source=True, include_alias=True, include_upstream=True)

  def transaction():
    vuln = Vulnerability.get_by_id(entity.db_id)
    if vuln is None:
      vuln = Vulnerability(id=entity.db_id)
    if vuln.modified != vuln_pb.modified.ToDatetime(__import__('datetime').UTC):
      vuln.source_id = entity.source_id
      vuln.modified = vuln_pb.modified.ToDatetime(__import__('datetime').UTC)
      vuln.is_withdrawn = entity.withdrawn is not None
      vuln.modified_raw = entity.import_last_modified
      vuln.alias_raw = entity.aliases
      vuln.related_raw = entity.related
      vuln.upstream_raw = entity.upstream_raw
    put_entities(vuln, vuln_pb)

  ndb.transaction(transaction)
  try:
    gcs.upload_vulnerability(vuln_pb)
  except Exception:  # pylint: disable=broad-except
    logging.error('Writing to bucket failed for %s', entity.db_id)
    data = vuln_pb.SerializeToString(deterministic=True)
    pubsub.publish_failure(data, type='gcs_retry')
