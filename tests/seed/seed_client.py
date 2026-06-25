"""
seed_client.py — Shared HTTP client for OSV seed scripts.

Features
--------
* Login via POST /api/v1/auth/login → caches JWT token
* Falls back to API-Key header authentication
* Automatic retry on transient 5xx errors with exponential back-off
* Structured logging via Python logging module
* All responses are returned as dict; caller decides what to do on errors
"""

from __future__ import annotations

import json
import logging
import time
from typing import Any

try:
    import requests
    from requests.adapters import HTTPAdapter
    from urllib3.util.retry import Retry
except ImportError as e:
    raise SystemExit(
        "Missing dependency: requests\n"
        "Install with: pip install requests"
    ) from e

from seed_config import SeedConfig

logger = logging.getLogger("seed.client")


# ---------------------------------------------------------------------------
# HTTP Client
# ---------------------------------------------------------------------------


class SeedClient:
    """Thin wrapper around requests.Session with auth and retry support."""

    def __init__(self, config: SeedConfig) -> None:
        self.config = config
        self._session: requests.Session = self._build_session()
        self._jwt_token: str = ""

    # ------------------------------------------------------------------
    # Session & Retry
    # ------------------------------------------------------------------

    def _build_session(self) -> requests.Session:
        session = requests.Session()
        retry = Retry(
            total=self.config.retry_attempts,
            backoff_factor=self.config.retry_delay,
            status_forcelist=[500, 502, 503, 504],
            allowed_methods=["GET", "POST", "PUT", "PATCH", "DELETE"],
            raise_on_status=False,
        )
        adapter = HTTPAdapter(max_retries=retry)
        session.mount("http://", adapter)
        session.mount("https://", adapter)
        return session

    # ------------------------------------------------------------------
    # Authentication
    # ------------------------------------------------------------------

    def authenticate(self) -> None:
        """Obtain a JWT token or set the API-Key header."""
        if self.config.admin_api_key:
            logger.info("Using API-Key authentication")
            self._session.headers.update({"X-API-Key": self.config.admin_api_key})
            return

        logger.info("Logging in as %s …", self.config.admin_email)
        resp = self._raw_post(
            f"{self.config.gateway_url}/api/v1/auth/login",
            json={
                "email": self.config.admin_email,
                "password": self.config.admin_password,
            },
            authenticated=False,
        )
        if resp.status_code != 200:
            raise RuntimeError(
                f"Login failed: HTTP {resp.status_code} — {resp.text[:500]}"
            )
        data = resp.json()
        # Supports: {"token": "..."} or {"access_token": "..."}
        token = data.get("token") or data.get("access_token") or data.get("data", {}).get("token")
        if not token:
            raise RuntimeError(f"No token in login response: {data}")
        self._jwt_token = token
        self._session.headers.update({"Authorization": f"Bearer {token}"})
        logger.info("Authenticated successfully (token cached)")

    # ------------------------------------------------------------------
    # Low-level HTTP helpers
    # ------------------------------------------------------------------

    def _raw_post(
        self,
        url: str,
        *,
        json: Any = None,
        data: Any = None,
        files: Any = None,
        authenticated: bool = True,
    ) -> requests.Response:
        headers: dict[str, str] = {}
        if not authenticated:
            # strip auth header for public endpoints
            headers["Authorization"] = ""
        return self._session.post(
            url,
            json=json,
            data=data,
            files=files,
            headers=headers or None,
            timeout=self.config.request_timeout,
        )

    def _request(
        self,
        method: str,
        url: str,
        *,
        json_body: Any = None,
        params: dict[str, Any] | None = None,
        data: Any = None,
        files: Any = None,
    ) -> requests.Response:
        """Execute an authenticated request with logging."""
        logger.debug("%s %s", method.upper(), url)
        resp = self._session.request(
            method=method.upper(),
            url=url,
            json=json_body,
            params=params,
            data=data,
            files=files,
            timeout=self.config.request_timeout,
        )
        log_fn = logger.debug if resp.ok else logger.warning
        log_fn(
            "%s %s → %d (%s)",
            method.upper(),
            url,
            resp.status_code,
            "OK" if resp.ok else resp.text[:200],
        )
        return resp

    # ------------------------------------------------------------------
    # Convenience wrappers
    # ------------------------------------------------------------------

    def get(self, path: str, *, base_url: str = "", params: dict | None = None) -> requests.Response:
        url = (base_url or self.config.gateway_url) + path
        return self._request("GET", url, params=params)

    def post(
        self,
        path: str,
        *,
        base_url: str = "",
        body: Any = None,
        data: Any = None,
        files: Any = None,
    ) -> requests.Response:
        url = (base_url or self.config.gateway_url) + path
        return self._request("POST", url, json_body=body, data=data, files=files)

    def put(self, path: str, *, base_url: str = "", body: Any = None) -> requests.Response:
        url = (base_url or self.config.gateway_url) + path
        return self._request("PUT", url, json_body=body)

    def patch(self, path: str, *, base_url: str = "", body: Any = None) -> requests.Response:
        url = (base_url or self.config.gateway_url) + path
        return self._request("PATCH", url, json_body=body)

    def delete(self, path: str, *, base_url: str = "") -> requests.Response:
        url = (base_url or self.config.gateway_url) + path
        return self._request("DELETE", url)

    # ------------------------------------------------------------------
    # Response helpers
    # ------------------------------------------------------------------

    @staticmethod
    def parse_json(resp: requests.Response, context: str = "") -> dict | list | None:
        """Try to parse response JSON; log and return None on failure."""
        try:
            return resp.json()
        except Exception:
            logger.error("[%s] Could not parse JSON from response: %s", context, resp.text[:300])
            return None

    @staticmethod
    def expect_status(
        resp: requests.Response,
        *expected: int,
        context: str = "",
        raise_on_fail: bool = False,
    ) -> bool:
        """Return True if response status is in *expected*."""
        ok = resp.status_code in expected
        if not ok:
            msg = (
                f"[{context}] Expected HTTP {expected}, got {resp.status_code}: "
                f"{resp.text[:300]}"
            )
            logger.error(msg)
            if raise_on_fail:
                raise AssertionError(msg)
        return ok

    # ------------------------------------------------------------------
    # Health check
    # ------------------------------------------------------------------

    def health_check(self) -> bool:
        """Return True if the gateway health endpoint is reachable."""
        try:
            resp = self._session.get(
                f"{self.config.gateway_url}/health",
                timeout=5,
            )
            return resp.ok
        except requests.exceptions.ConnectionError:
            return False
