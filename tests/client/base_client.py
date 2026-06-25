"""
base_client.py — HTTP client cơ sở với auto-login, logging, và schema validation.

Tất cả test scripts đều import và dùng APIClient từ file này.
"""

from __future__ import annotations

import json
import sys
import time
from typing import Any, Dict, List, Optional, Tuple

import requests
from requests import Response, Session

from config import Config


# ── Color helpers ─────────────────────────────────────────────────────────────

class _Color:
    RESET   = "\033[0m"
    GREEN   = "\033[92m"
    RED     = "\033[91m"
    YELLOW  = "\033[93m"
    CYAN    = "\033[96m"
    BOLD    = "\033[1m"
    MAGENTA = "\033[95m"


def _ok(msg: str) -> str:
    return f"{_Color.GREEN}✓ {msg}{_Color.RESET}"


def _fail(msg: str) -> str:
    return f"{_Color.RED}✗ {msg}{_Color.RESET}"


def _warn(msg: str) -> str:
    return f"{_Color.YELLOW}⚠ {msg}{_Color.RESET}"


def _info(msg: str) -> str:
    return f"{_Color.CYAN}→ {msg}{_Color.RESET}"


# ── Test result tracking ──────────────────────────────────────────────────────

class TestResults:
    """Theo dõi kết quả các test cases."""

    def __init__(self) -> None:
        self.passed: List[str] = []
        self.failed: List[Tuple[str, str]] = []  # (name, reason)
        self.skipped: List[str] = []
        self.start_time: float = time.time()

    def record_pass(self, name: str) -> None:
        print(_ok(name))
        self.passed.append(name)

    def record_fail(self, name: str, reason: str) -> None:
        print(_fail(f"{name}: {reason}"))
        self.failed.append((name, reason))

    def record_skip(self, name: str, reason: str) -> None:
        print(_warn(f"SKIP {name}: {reason}"))
        self.skipped.append(name)

    def summary(self) -> None:
        elapsed = time.time() - self.start_time
        total = len(self.passed) + len(self.failed) + len(self.skipped)
        print()
        print("=" * 60)
        print(f"{_Color.BOLD}TEST SUMMARY{_Color.RESET}")
        print("=" * 60)
        print(f"  Total   : {total}")
        print(f"  {_Color.GREEN}Passed  : {len(self.passed)}{_Color.RESET}")
        print(f"  {_Color.RED}Failed  : {len(self.failed)}{_Color.RESET}")
        print(f"  {_Color.YELLOW}Skipped : {len(self.skipped)}{_Color.RESET}")
        print(f"  Time    : {elapsed:.2f}s")
        print("=" * 60)
        if self.failed:
            print(f"\n{_Color.RED}FAILURES:{_Color.RESET}")
            for name, reason in self.failed:
                print(f"  - {name}: {reason}")
        print()

    def exit_code(self) -> int:
        return 1 if self.failed else 0


# ── Schema validator (lightweight, không cần jsonschema lib) ──────────────────

def validate_required_fields(data: Any, required: List[str], path: str = "root") -> List[str]:
    """Trả về list các field bị thiếu."""
    errors: List[str] = []
    if not isinstance(data, dict):
        return [f"{path}: expected object, got {type(data).__name__}"]
    for field in required:
        if field not in data:
            errors.append(f"{path}.{field} is missing")
        elif data[field] is None:
            # None cho phép nếu schema không có nullable=true,
            # nhưng ta chỉ check sự tồn tại của key ở đây
            pass
    return errors


def validate_list_response(data: Any, list_key: str, total_key: str = "total") -> List[str]:
    """Validate response dạng { list_key: [...], total: int }."""
    errors: List[str] = []
    if not isinstance(data, dict):
        return [f"expected object, got {type(data).__name__}"]
    if list_key not in data:
        errors.append(f"missing field '{list_key}'")
    elif not isinstance(data[list_key], list):
        errors.append(f"'{list_key}' must be an array")
    if total_key not in data:
        errors.append(f"missing field '{total_key}'")
    elif not isinstance(data[total_key], int):
        errors.append(f"'{total_key}' must be integer")
    return errors


def validate_pagination(data: Dict) -> List[str]:
    """Validate trường phân trang: page, page_size."""
    errors: List[str] = []
    for field in ("page", "page_size"):
        if field not in data:
            errors.append(f"missing pagination field '{field}'")
        elif not isinstance(data[field], int):
            errors.append(f"'{field}' must be integer")
    return errors


# ── APIClient ─────────────────────────────────────────────────────────────────

class APIClient:
    """HTTP client dùng chung cho mọi test script."""

    def __init__(self) -> None:
        self.v1_base = Config.API_BASE_URL_V1.rstrip("/")
        self.v2_base = Config.API_BASE_URL_V2.rstrip("/")
        self.timeout = Config.REQUEST_TIMEOUT
        self.verbose = Config.VERBOSE
        self.session: Session = requests.Session()
        self.session.headers.update({
            "Content-Type": "application/json",
            "Accept": "application/json",
        })
        self._access_token: Optional[str] = None

    # ── Auth ──────────────────────────────────────────────────────────────────

    def login(self) -> bool:
        """Đăng nhập và lưu access token vào session.
        Trả về True nếu thành công.
        """
        # Dùng token có sẵn từ .env nếu đã cấu hình
        if Config.ACCESS_TOKEN:
            self._access_token = Config.ACCESS_TOKEN
            self.session.headers["Authorization"] = f"Bearer {self._access_token}"
            if self.verbose:
                print(_info("Using pre-configured ACCESS_TOKEN from .env"))
            return True

        if self.verbose:
            print(_info(f"Logging in as {Config.TEST_EMAIL} ..."))

        try:
            resp = self.session.post(
                f"{self.v1_base}/auth/login",
                json={"email": Config.TEST_EMAIL, "password": Config.TEST_PASSWORD},
                timeout=self.timeout,
            )
        except requests.exceptions.ConnectionError as e:
            print(_fail(f"Cannot connect to {self.v1_base}: {e}"))
            return False

        if resp.status_code != 200:
            print(_fail(f"Login failed: HTTP {resp.status_code} — {resp.text[:200]}"))
            return False

        body = resp.json()

        # Xử lý MFA
        if body.get("mfa_required"):
            print(_warn("MFA is required. Set ACCESS_TOKEN in .env to bypass."))
            return False

        token = body.get("access_token")
        if not token:
            print(_fail("Login response missing access_token"))
            return False

        self._access_token = token
        self.session.headers["Authorization"] = f"Bearer {token}"
        if self.verbose:
            print(_ok(f"Logged in. Token: {token[:30]}..."))
        return True

    # ── Request helpers ───────────────────────────────────────────────────────

    def _log(self, method: str, url: str, status: int, elapsed_ms: float) -> None:
        if not self.verbose:
            return
        color = _Color.GREEN if status < 400 else _Color.RED
        print(f"  {method:6s} {url}  →  {color}{status}{_Color.RESET}  ({elapsed_ms:.0f}ms)")

    def get(self, path: str, *, v2: bool = False, params: Optional[Dict] = None) -> Response:
        base = self.v2_base if v2 else self.v1_base
        url = f"{base}{path}"
        t0 = time.time()
        resp = self.session.get(url, params=params, timeout=self.timeout)
        self._log("GET", url, resp.status_code, (time.time() - t0) * 1000)
        return resp

    def post(self, path: str, *, v2: bool = False, body: Optional[Dict] = None) -> Response:
        base = self.v2_base if v2 else self.v1_base
        url = f"{base}{path}"
        t0 = time.time()
        resp = self.session.post(url, json=body, timeout=self.timeout)
        self._log("POST", url, resp.status_code, (time.time() - t0) * 1000)
        return resp

    def patch(self, path: str, *, v2: bool = False, body: Optional[Dict] = None) -> Response:
        base = self.v2_base if v2 else self.v1_base
        url = f"{base}{path}"
        t0 = time.time()
        resp = self.session.patch(url, json=body, timeout=self.timeout)
        self._log("PATCH", url, resp.status_code, (time.time() - t0) * 1000)
        return resp

    def delete(self, path: str, *, v2: bool = False) -> Response:
        base = self.v2_base if v2 else self.v1_base
        url = f"{base}{path}"
        t0 = time.time()
        resp = self.session.delete(url, timeout=self.timeout)
        self._log("DELETE", url, resp.status_code, (time.time() - t0) * 1000)
        return resp

    def put(self, path: str, *, v2: bool = False, body: Optional[Dict] = None) -> Response:
        base = self.v2_base if v2 else self.v1_base
        url = f"{base}{path}"
        t0 = time.time()
        resp = self.session.put(url, json=body, timeout=self.timeout)
        self._log("PUT", url, resp.status_code, (time.time() - t0) * 1000)
        return resp

    # ── Assertion helpers ─────────────────────────────────────────────────────

    def assert_status(self, resp: Response, expected: int, test_name: str, results: TestResults) -> bool:
        if resp.status_code != expected:
            results.record_fail(
                test_name,
                f"Expected HTTP {expected}, got {resp.status_code}. Body: {resp.text[:200]}"
            )
            return False
        return True

    def assert_json_fields(
        self,
        data: Any,
        required_fields: List[str],
        test_name: str,
        results: TestResults,
        path: str = "response",
    ) -> bool:
        errors = validate_required_fields(data, required_fields, path)
        if errors:
            results.record_fail(test_name, "; ".join(errors))
            return False
        return True

    def pretty_json(self, data: Any, max_len: int = 500) -> str:
        """Trả về JSON đẹp, cắt nếu quá dài."""
        s = json.dumps(data, indent=2, ensure_ascii=False)
        if len(s) > max_len:
            return s[:max_len] + "\n  ... (truncated)"
        return s
