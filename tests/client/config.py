"""
config.py — Đọc cấu hình test từ file .env

Tự động tìm .env trong thư mục hiện tại hoặc thư mục của script.
"""

import os
from pathlib import Path
from typing import Optional


def _load_dotenv(env_path: Path) -> None:
    """Parse và load .env file thủ công (không cần thư viện python-dotenv)."""
    if not env_path.exists():
        return
    with open(env_path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            # Bỏ qua comment và dòng trống
            if not line or line.startswith("#"):
                continue
            if "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip()
            # Bỏ quotes bao quanh value nếu có
            if len(value) >= 2 and value[0] == value[-1] and value[0] in ('"', "'"):
                value = value[1:-1]
            # Không override biến đã được set trong environment
            if key and key not in os.environ:
                os.environ[key] = value


# Tìm .env file
_here = Path(__file__).parent
_dotenv_candidates = [
    _here / ".env",
    _here.parent / ".env",
    Path.cwd() / ".env",
]
for _candidate in _dotenv_candidates:
    if _candidate.exists():
        _load_dotenv(_candidate)
        break


class Config:
    """Tập hợp tất cả cấu hình từ environment variables."""

    # ── API Endpoints ──────────────────────────────────────────────────────────
    API_BASE_URL_V1: str = os.environ.get("API_BASE_URL_V1", "http://localhost:8080/api/v1")
    API_BASE_URL_V2: str = os.environ.get("API_BASE_URL_V2", "http://localhost:8080/api/v2")

    # ── Auth ───────────────────────────────────────────────────────────────────
    TEST_EMAIL: str = os.environ.get("TEST_EMAIL", "admin@osv.local")
    TEST_PASSWORD: str = os.environ.get("TEST_PASSWORD", "changeme")
    ACCESS_TOKEN: Optional[str] = os.environ.get("ACCESS_TOKEN") or None

    # ── HTTP ───────────────────────────────────────────────────────────────────
    REQUEST_TIMEOUT: int = int(os.environ.get("REQUEST_TIMEOUT", "30"))
    VERBOSE: bool = os.environ.get("VERBOSE", "true").lower() in ("true", "1", "yes")

    # ── Sample IDs (tuỳ dữ liệu trong DB) ─────────────────────────────────────
    SAMPLE_CVE_ID: str = os.environ.get("SAMPLE_CVE_ID", "CVE-2021-44228")
    SAMPLE_SCAN_ID: Optional[str] = os.environ.get("SAMPLE_SCAN_ID") or None
    SAMPLE_FINDING_ID: Optional[str] = os.environ.get("SAMPLE_FINDING_ID") or None
    SAMPLE_PRODUCT_ID: Optional[str] = os.environ.get("SAMPLE_PRODUCT_ID") or None
    SAMPLE_ASSET_ID: Optional[str] = os.environ.get("SAMPLE_ASSET_ID") or None
    SAMPLE_CWE_ID: str = os.environ.get("SAMPLE_CWE_ID", "CWE-89")
    SAMPLE_CAPEC_ID: str = os.environ.get("SAMPLE_CAPEC_ID", "CAPEC-66")
    SAMPLE_VENDOR: str = os.environ.get("SAMPLE_VENDOR", "apache")
    SAMPLE_PRODUCT_NAME: str = os.environ.get("SAMPLE_PRODUCT_NAME", "log4j")

    @classmethod
    def dump(cls) -> None:
        """In ra toàn bộ cấu hình (che password và token)."""
        print("=" * 60)
        print("TEST CLIENT CONFIGURATION")
        print("=" * 60)
        print(f"  API_BASE_URL_V1  : {cls.API_BASE_URL_V1}")
        print(f"  API_BASE_URL_V2  : {cls.API_BASE_URL_V2}")
        print(f"  TEST_EMAIL       : {cls.TEST_EMAIL}")
        print(f"  TEST_PASSWORD    : {'*' * len(cls.TEST_PASSWORD)}")
        token_display = f"{cls.ACCESS_TOKEN[:20]}..." if cls.ACCESS_TOKEN else "(auto-login)"
        print(f"  ACCESS_TOKEN     : {token_display}")
        print(f"  REQUEST_TIMEOUT  : {cls.REQUEST_TIMEOUT}s")
        print(f"  VERBOSE          : {cls.VERBOSE}")
        print(f"  SAMPLE_CVE_ID    : {cls.SAMPLE_CVE_ID}")
        print(f"  SAMPLE_CWE_ID    : {cls.SAMPLE_CWE_ID}")
        print(f"  SAMPLE_VENDOR    : {cls.SAMPLE_VENDOR}")
        print("=" * 60)
