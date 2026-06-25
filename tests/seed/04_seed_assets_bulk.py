#!/usr/bin/env python3
"""
04_seed_assets_bulk.py — Seed 1000+ assets to /api/v1/assets (individual POST).

This script generates and pushes assets directly to the server in small
batches (default 50 per batch) with checkpoint/resume support, so that
large seeding runs are safe against network interruptions.

Why not use /api/v1/assets/bulk?
  The bulk endpoint may not exist or may have a size limit. This script
  falls back to individual POSTs and tracks progress via a checkpoint file.

Usage:
    # Generate and seed 1200 assets (default)
    python 04_seed_assets_bulk.py

    # Seed a specific count
    python 04_seed_assets_bulk.py --count 2000

    # Resume from checkpoint (skip already-created assets)
    python 04_seed_assets_bulk.py --resume

    # Dry-run (print what would be sent, no actual HTTP calls)
    python 04_seed_assets_bulk.py --dry-run

    # Generate data file only, do not push
    python 04_seed_assets_bulk.py --generate-only

    # Use pre-generated data file
    python 04_seed_assets_bulk.py --data-file ./data/assets/assets.json
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
import time
from pathlib import Path
from typing import Any

sys.path.insert(0, str(Path(__file__).parent))
from seed_config import SeedConfig  # noqa: E402
from seed_client import SeedClient  # noqa: E402

# ---------------------------------------------------------------------------
# Inline generator import (avoids requiring full 01_generate_seed_data import)
# ---------------------------------------------------------------------------

try:
    # Try importing from the main generate script
    import importlib.util as _ilu
    _spec = _ilu.spec_from_file_location(
        "generate",
        Path(__file__).parent / "01_generate_seed_data.py",
    )
    _mod = _ilu.module_from_spec(_spec)  # type: ignore[arg-type]
    _spec.loader.exec_module(_mod)  # type: ignore[union-attr]
    gen_assets = _mod.gen_assets  # type: ignore[attr-defined]
except Exception as _e:
    raise SystemExit(
        f"Cannot import gen_assets from 01_generate_seed_data.py: {_e}"
    ) from _e

logger = logging.getLogger("seed.assets_bulk")


# ---------------------------------------------------------------------------
# Checkpoint — tracks which assets were already successfully pushed
# ---------------------------------------------------------------------------

class Checkpoint:
    """Persist pushed asset IDs to allow safe resume on restart."""

    def __init__(self, path: Path) -> None:
        self._path = path
        self._done: set[str] = set()
        self._server_ids: dict[str, str] = {}  # local_id → server_id
        if path.exists():
            try:
                data = json.loads(path.read_text(encoding="utf-8"))
                self._done = set(data.get("done", []))
                self._server_ids = data.get("server_ids", {})
                logger.info(
                    "Checkpoint loaded: %d assets already pushed", len(self._done)
                )
            except Exception:
                logger.warning("Could not load checkpoint — starting fresh")

    def is_done(self, local_id: str) -> bool:
        return local_id in self._done

    def mark_done(self, local_id: str, server_id: str) -> None:
        self._done.add(local_id)
        self._server_ids[local_id] = server_id

    def save(self) -> None:
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._path.write_text(
            json.dumps(
                {"done": list(self._done), "server_ids": self._server_ids},
                indent=2,
            ),
            encoding="utf-8",
        )

    @property
    def count(self) -> int:
        return len(self._done)


# ---------------------------------------------------------------------------
# Bulk push helpers
# ---------------------------------------------------------------------------

def _try_bulk_endpoint(
    client: SeedClient,
    batch: list[dict],
    checkpoint: Checkpoint,
) -> tuple[int, int, int]:
    """Attempt POST /api/v1/assets/bulk.  Returns (created, updated, failed)."""
    payload = {
        "assets": [
            {
                "ip_address": a["ip_address"],
                "hostname": a.get("hostname", ""),
                "os": a.get("os", ""),
                "mac_address": a.get("mac_address", ""),
                "services": a.get("services", []),
                "tags": a.get("tags", []),
                "labels": a.get("labels", {}),
            }
            for a in batch
        ],
        "update_existing": True,
    }
    resp = client.post("/api/v1/assets/bulk", body=payload)
    if resp.status_code not in (200, 201, 207):
        return -1, -1, -1  # Signal: try individual fallback

    data = resp.json()
    results = data.get("results", [])
    created = updated = failed = 0
    for i, item in enumerate(results):
        if i >= len(batch):
            break
        local_id = batch[i]["_id"]
        server_id = item.get("id", "")
        status = item.get("status", "created")
        if status in ("created", "updated"):
            checkpoint.mark_done(local_id, server_id)
            if status == "created":
                created += 1
            else:
                updated += 1
        else:
            failed += 1
    return created, updated, failed


def _push_individual(
    client: SeedClient,
    asset: dict,
    checkpoint: Checkpoint,
    dry_run: bool,
) -> bool:
    """POST a single asset. Returns True on success."""
    local_id = asset["_id"]
    if checkpoint.is_done(local_id):
        return True  # already pushed

    payload = {
        "ip_address": asset["ip_address"],
        "hostname": asset.get("hostname", ""),
        "os": asset.get("os", ""),
        "mac_address": asset.get("mac_address", ""),
        "services": asset.get("services", []),
        "tags": asset.get("tags", []),
        "labels": asset.get("labels", {}),
    }

    if dry_run:
        logger.debug("[DRY-RUN] POST /api/v1/assets %s", asset["ip_address"])
        checkpoint.mark_done(local_id, f"dry-{local_id[:8]}")
        return True

    resp = client.post("/api/v1/assets", body=payload)
    if resp.status_code in (200, 201):
        data = resp.json()
        server_id = data.get("id") or data.get("data", {}).get("id", "")
        checkpoint.mark_done(local_id, server_id)
        return True
    elif resp.status_code == 409:
        # Already exists — record as done to avoid retry
        checkpoint.mark_done(local_id, "existing")
        return True
    else:
        logger.warning(
            "  ✗ asset %s failed: HTTP %d — %s",
            asset["ip_address"],
            resp.status_code,
            resp.text[:150],
        )
        return False


# ---------------------------------------------------------------------------
# Main seeding loop
# ---------------------------------------------------------------------------

def seed_assets_bulk(
    client: SeedClient,
    assets: list[dict],
    checkpoint: Checkpoint,
    *,
    batch_size: int = 50,
    dry_run: bool = False,
    use_bulk_endpoint: bool = True,
) -> dict[str, int]:
    """Push assets to the server in batches.

    Returns summary counters: created, updated, skipped, failed.
    """
    total = len(assets)
    created = updated = skipped = failed = 0
    batch_num = 0
    start_time = time.time()

    # Split into batches
    batches = [assets[i:i + batch_size] for i in range(0, total, batch_size)]
    total_batches = len(batches)

    logger.info(
        "Starting bulk asset seed: %d assets, %d batches of %d",
        total, total_batches, batch_size,
    )

    for batch in batches:
        batch_num += 1

        # Skip already-done assets
        pending = [a for a in batch if not checkpoint.is_done(a["_id"])]
        already_done = len(batch) - len(pending)
        skipped += already_done

        if not pending:
            logger.debug("  Batch %d/%d — all %d skipped (already pushed)", batch_num, total_batches, len(batch))
            continue

        # --- Try bulk endpoint first ---
        if use_bulk_endpoint and not dry_run:
            bc, bu, bf = _try_bulk_endpoint(client, pending, checkpoint)
            if bc >= 0:
                # Bulk succeeded
                created += bc
                updated += bu
                failed += bf
                logger.info(
                    "  Batch %d/%d — bulk: created=%d updated=%d failed=%d skip=%d",
                    batch_num, total_batches, bc, bu, bf, already_done,
                )
                checkpoint.save()
                continue
            else:
                logger.debug(
                    "  Batch %d/%d — bulk endpoint unavailable, falling back to individual POST",
                    batch_num, total_batches,
                )
                use_bulk_endpoint = False  # Disable for subsequent batches

        # --- Individual POST fallback ---
        batch_created = batch_failed = 0
        for asset in pending:
            ok = _push_individual(client, asset, checkpoint, dry_run)
            if ok:
                batch_created += 1
            else:
                batch_failed += 1

        created += batch_created
        failed += batch_failed

        elapsed = time.time() - start_time
        total_done = created + updated + skipped
        rate = total_done / elapsed if elapsed > 0 else 0
        eta = (total - total_done) / rate if rate > 0 else 0

        logger.info(
            "  Batch %d/%d — created=%d skip=%d fail=%d | total=%d/%d rate=%.1f/s ETA=%.0fs",
            batch_num, total_batches,
            batch_created, already_done, batch_failed,
            total_done, total,
            rate, eta,
        )
        checkpoint.save()

    return {
        "total": total,
        "created": created,
        "updated": updated,
        "skipped": skipped,
        "failed": failed,
    }


# ---------------------------------------------------------------------------
# Verify
# ---------------------------------------------------------------------------

def verify_asset_count(client: SeedClient, expected_min: int = 1000) -> None:
    """GET /api/v1/assets to verify count is >= expected_min."""
    resp = client.get("/api/v1/assets", params={"page": 1, "page_size": 1})
    if not resp.ok:
        logger.warning("Cannot verify asset count: HTTP %d", resp.status_code)
        return
    data = resp.json()
    total = data.get("total", 0)
    logger.info("=== Verification: server reports %d total assets ===", total)
    if total >= expected_min:
        logger.info("  ✓ Target met: %d >= %d", total, expected_min)
    else:
        logger.warning(
            "  ✗ Target NOT met: %d < %d (may still be indexing)", total, expected_min
        )


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Seed 1000+ assets to OSV Platform via /api/v1/assets"
    )
    p.add_argument("--env", default=None, help="Path to .env file")
    p.add_argument(
        "--count",
        type=int,
        default=1200,
        help="Number of assets to generate and push (default: 1200)",
    )
    p.add_argument(
        "--batch-size",
        type=int,
        default=50,
        help="Assets per request batch (default: 50)",
    )
    p.add_argument(
        "--data-file",
        default=None,
        help="Path to pre-generated assets JSON file (skips generation step)",
    )
    p.add_argument(
        "--generate-only",
        action="store_true",
        help="Generate data file only; do not push to server",
    )
    p.add_argument(
        "--resume",
        action="store_true",
        help="Resume from checkpoint (skip assets already pushed)",
    )
    p.add_argument(
        "--dry-run",
        action="store_true",
        help="Print what would be sent without making real HTTP calls",
    )
    p.add_argument(
        "--no-bulk",
        action="store_true",
        help="Disable bulk endpoint; always use individual POST",
    )
    p.add_argument(
        "--verify-only",
        action="store_true",
        help="Only verify the current asset count on the server",
    )
    return p.parse_args()


def main() -> None:
    args = parse_args()
    cfg = SeedConfig(env_file=args.env)
    cfg.setup_logging()

    client = SeedClient(cfg)

    # --- Verify-only mode ---
    if args.verify_only:
        client.authenticate()
        verify_asset_count(client)
        return

    # --- Determine data source ---
    if args.data_file:
        data_path = Path(args.data_file)
        if not data_path.exists():
            raise SystemExit(f"Data file not found: {data_path}")
        logger.info("Loading assets from %s …", data_path)
        assets = json.loads(data_path.read_text(encoding="utf-8"))
        if not isinstance(assets, list):
            raise SystemExit("Data file must be a JSON array of asset objects")
        logger.info("Loaded %d assets from file", len(assets))
    else:
        logger.info("Generating %d assets …", args.count)
        assets = gen_assets(n=args.count)
        logger.info("Generated %d assets", len(assets))

        # Save generated data for reference
        out_path = cfg.seed_data_dir / "assets" / "assets.json"
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(
            json.dumps(assets, indent=2, ensure_ascii=False), encoding="utf-8"
        )
        logger.info("Saved generated assets → %s", out_path)

    if args.generate_only:
        logger.info("--generate-only: stopping before push")
        logger.info("Assets saved to: %s", cfg.seed_data_dir / "assets" / "assets.json")
        return

    # --- Checkpoint ---
    checkpoint_path = cfg.seed_data_dir / "output" / "assets_checkpoint.json"
    if not args.resume and checkpoint_path.exists():
        logger.info("Removing old checkpoint (use --resume to keep it)")
        checkpoint_path.unlink()
    checkpoint = Checkpoint(checkpoint_path)

    # --- Authenticate ---
    if not args.dry_run:
        client.authenticate()

    # --- Push ---
    summary = seed_assets_bulk(
        client,
        assets,
        checkpoint,
        batch_size=args.batch_size,
        dry_run=args.dry_run,
        use_bulk_endpoint=not args.no_bulk,
    )

    logger.info(
        "=== Seed complete: total=%d created=%d updated=%d skipped=%d failed=%d ===",
        summary["total"],
        summary["created"],
        summary["updated"],
        summary["skipped"],
        summary["failed"],
    )

    if not args.dry_run:
        verify_asset_count(client, expected_min=1000)

    if summary["failed"] > 0:
        logger.warning(
            "%d assets failed to push — run again with --resume to retry",
            summary["failed"],
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
