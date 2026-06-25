"""
seed_config.py — Shared configuration loader for OSV seed scripts.

Reads settings from a .env file (python-dotenv) and exposes them as
typed attributes.  All three seed scripts import this module.
"""

from __future__ import annotations

import logging
import os
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Optional dotenv support — install with: pip install python-dotenv
# ---------------------------------------------------------------------------
try:
    from dotenv import load_dotenv  # type: ignore[import-untyped]

    _DOTENV_AVAILABLE = True
except ImportError:
    _DOTENV_AVAILABLE = False


def load_env(env_file: str | Path | None = None) -> None:
    """Load environment variables from a .env file.

    Searches for .env in the current directory and the directory of this
    script if *env_file* is not explicitly provided.
    """
    if not _DOTENV_AVAILABLE:
        print(
            "[WARN] python-dotenv is not installed — reading from OS environment only.\n"
            "       Install with: pip install python-dotenv",
            file=sys.stderr,
        )
        return

    candidates: list[Path] = []
    if env_file:
        candidates.append(Path(env_file))
    else:
        candidates = [
            Path.cwd() / ".env",
            Path(__file__).parent / ".env",
        ]

    for p in candidates:
        if p.exists():
            load_dotenv(p, override=False)
            print(f"[CONFIG] Loaded .env from: {p}")
            return

    print(
        "[WARN] No .env file found — using OS environment variables only.",
        file=sys.stderr,
    )


# ---------------------------------------------------------------------------
# Configuration class
# ---------------------------------------------------------------------------


class SeedConfig:
    """Holds all seed script configuration read from environment variables."""

    # ------------------------------------------------------------------
    # Constructor
    # ------------------------------------------------------------------
    def __init__(self, env_file: str | Path | None = None) -> None:
        load_env(env_file)
        self._load()

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------
    def _get(self, key: str, default: str = "") -> str:
        return os.environ.get(key, default).strip()

    def _get_int(self, key: str, default: int) -> int:
        try:
            return int(os.environ.get(key, str(default)))
        except ValueError:
            return default

    def _get_bool(self, key: str, default: bool = False) -> bool:
        val = os.environ.get(key, str(default)).strip().lower()
        return val in ("1", "true", "yes")

    # ------------------------------------------------------------------
    # Load all config values
    # ------------------------------------------------------------------
    def _load(self) -> None:
        # Gateway
        self.gateway_url: str = self._get("GATEWAY_URL", "http://localhost:8080").rstrip("/")

        # Admin auth
        self.admin_email: str = self._get("ADMIN_EMAIL", "admin@company.com")
        self.admin_password: str = self._get("ADMIN_PASSWORD", "")
        self.admin_api_key: str = self._get("ADMIN_API_KEY", "")

        # Data directories
        seed_dir = Path(__file__).parent
        self.seed_data_dir: Path = Path(self._get("SEED_DATA_DIR", str(seed_dir / "data")))
        self.seed_output_dir: Path = Path(self._get("SEED_OUTPUT_DIR", str(seed_dir / "data" / "output")))

        # HTTP behaviour
        self.retry_attempts: int = self._get_int("RETRY_ATTEMPTS", 3)
        self.retry_delay: float = float(self._get("RETRY_DELAY", "2"))
        self.request_timeout: int = self._get_int("REQUEST_TIMEOUT", 30)
        self.verbose: bool = self._get_bool("VERBOSE", False)

        # Optional per-service URLs (bypasses gateway when set)
        self.identity_service_url: str = self._get("IDENTITY_SERVICE_URL", "").rstrip("/")
        self.finding_service_url: str = self._get("FINDING_SERVICE_URL", "").rstrip("/")
        self.scan_service_url: str = self._get("SCAN_SERVICE_URL", "").rstrip("/")
        self.asset_service_url: str = self._get("ASSET_SERVICE_URL", "").rstrip("/")
        self.sla_service_url: str = self._get("SLA_SERVICE_URL", "").rstrip("/")
        self.notification_service_url: str = self._get("NOTIFICATION_SERVICE_URL", "").rstrip("/")
        self.jira_service_url: str = self._get("JIRA_SERVICE_URL", "").rstrip("/")
        self.ranking_service_url: str = self._get("RANKING_SERVICE_URL", "").rstrip("/")
        self.ai_service_url: str = self._get("AI_SERVICE_URL", "").rstrip("/")
        self.data_service_url: str = self._get("DATA_SERVICE_URL", "").rstrip("/")

    # ------------------------------------------------------------------
    # Convenience methods
    # ------------------------------------------------------------------
    def resolve_service_url(self, service: str) -> str:
        """Return the URL for *service* — per-service override or gateway fallback.

        Parameters
        ----------
        service: one of identity | finding | scan | asset | sla | notification | jira | ranking | ai | data
        """
        mapping = {
            "identity": self.identity_service_url,
            "finding": self.finding_service_url,
            "scan": self.scan_service_url,
            "asset": self.asset_service_url,
            "sla": self.sla_service_url,
            "notification": self.notification_service_url,
            "jira": self.jira_service_url,
            "ranking": self.ranking_service_url,
            "ai": self.ai_service_url,
            "data": self.data_service_url,
        }
        override = mapping.get(service, "")
        return override if override else self.gateway_url


    def validate(self) -> None:
        """Raise ValueError if required config is missing."""
        if not self.gateway_url:
            raise ValueError("GATEWAY_URL is required")
        if not self.admin_api_key and not (self.admin_email and self.admin_password):
            raise ValueError(
                "Either ADMIN_API_KEY or both ADMIN_EMAIL + ADMIN_PASSWORD are required"
            )

    # ------------------------------------------------------------------
    # Logging setup
    # ------------------------------------------------------------------
    def setup_logging(self) -> logging.Logger:
        level = logging.DEBUG if self.verbose else logging.INFO
        logging.basicConfig(
            level=level,
            format="%(asctime)s [%(levelname)s] %(name)s — %(message)s",
            datefmt="%Y-%m-%dT%H:%M:%S",
        )
        return logging.getLogger("seed")

    # ------------------------------------------------------------------
    # Display
    # ------------------------------------------------------------------
    def __repr__(self) -> str:
        return (
            f"SeedConfig("
            f"gateway={self.gateway_url!r}, "
            f"email={self.admin_email!r}, "
            f"data_dir={self.seed_data_dir}, "
            f"output_dir={self.seed_output_dir})"
        )
