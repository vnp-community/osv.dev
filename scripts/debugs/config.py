#!/usr/bin/env python3
"""
config.py — Shared configuration for all OSV debug scripts.
Load từ deploy/dev/.env — gọi API qua domain public (không SSH).

External routing (c12.openledger.vn → nginx → 172.20.2.48):
  /health         → gateway :8080/health
  /api/v2/*       → gateway :8080/api/v2/*
  /api/v1/*       → gateway :8080/api/v1/*  (requires auth)
  /v1/*           → gateway :8080/v1/*
  /cve/*          → gateway :8080/cve/*   (data-service embedded)
  /internal/*     → NOT exposed (admin-only, SSH-only)
"""

import os
import pathlib
import requests as _requests

# ── Auto-locate .env ──────────────────────────────────────────────────────────
_SCRIPT_DIR  = pathlib.Path(__file__).resolve().parent   # scripts/debugs/
_REPO_ROOT   = _SCRIPT_DIR.parent.parent                 # osv.dev/
_DEFAULT_ENV = _SCRIPT_DIR / ".env"

DOTENV_PATH = pathlib.Path(os.getenv("DOTENV_PATH", str(_DEFAULT_ENV)))


def _load_env(path: pathlib.Path) -> dict:
    """Parse .env file → dict. Ignores comments and blank lines."""
    env = {}
    if not path.exists():
        return env
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, val = line.partition("=")
        env[key.strip()] = val.strip().strip('"').strip("'")
    return env


_CFG = _load_env(DOTENV_PATH)


def _get(key: str, default: str = "") -> str:
    """Priority: OS env → .env file → default."""
    return os.getenv(key, _CFG.get(key, default))


# ── Public domain (primary entry point) ───────────────────────────────────────
PUBLIC_DOMAIN = os.getenv("OSV_DOMAIN", "https://c12.openledger.vn")

# Fallback direct IP nếu domain chưa forward đúng
SERVER_IP     = os.getenv("OSV_SERVER_IP", "172.20.2.48")
GATEWAY_PORT  = int(_get("HTTP_PORT", "8080"))

# Direct URL (dùng khi domain chưa route đủ endpoints)
DIRECT_URL    = os.getenv("OSV_DIRECT_URL", f"http://{SERVER_IP}:{GATEWAY_PORT}")

# Base URL mặc định — ưu tiên domain public
BASE_URL = os.getenv("OSV_BASE_URL", PUBLIC_DOMAIN)

# ── Database info (chỉ dùng trong 05_verify_stores.py qua SSH exec) ───────────
POSTGRES_USER = _get("POSTGRES_USER", "osv")
POSTGRES_PASS = _get("POSTGRES_PASSWORD", "osv_dev")
POSTGRES_DB   = _get("POSTGRES_DB", "osv")
MONGO_DB      = _get("MONGO_DB", "cvedb")
REDIS_PASS    = _get("REDIS_PASSWORD", "")

# ── SSH (chỉ dùng trong 05_verify_stores.py) ──────────────────────────────────
SSH_USER      = os.getenv("OSV_SSH_USER", "ubuntu")
COMPOSE_DIR   = "/opt/osv-backend"
COMPOSE_F     = "docker compose -f docker-compose.server.yml"

# ── Admin credentials ─────────────────────────────────────────────────────────
ADMIN_EMAIL    = _get("INIT_ADMIN_EMAIL",    "admin@openvulnscan.io")
ADMIN_PASSWORD = _get("INIT_ADMIN_PASSWORD", "Admin@123!ChangeMe")
JWT_SECRET     = _get("JWT_SECRET", "")

# ── HTTP settings ─────────────────────────────────────────────────────────────
REQUEST_TIMEOUT = int(os.getenv("REQUEST_TIMEOUT", "15"))

# ── Session với auth token ────────────────────────────────────────────────────
_session = None
_auth_token = None


def get_session() -> _requests.Session:
    """Trả về requests.Session (không auth)."""
    global _session
    if _session is None:
        _session = _requests.Session()
        _session.headers.update({
            "User-Agent": "osv-debug-scripts/1.0",
            "Accept":     "application/json",
        })
    return _session


from typing import Optional

def login(email: str = None, password: str = None) -> Optional[str]:
    """
    Login và lưu JWT token vào session. Trả về token hoặc None nếu fail.
    Tự động inject vào header Authorization cho các request tiếp theo.
    """
    global _auth_token
    email    = email    or ADMIN_EMAIL
    password = password or ADMIN_PASSWORD
    try:
        resp = api_post("/api/v1/auth/login", {
            "email": email, "password": password
        }, auth=False)
        token = (resp.get("data", {}) or {}).get("access_token") or resp.get("access_token")
        if token:
            _auth_token = token
            get_session().headers["Authorization"] = f"Bearer {token}"
            return token
    except Exception as e:
        warn(f"Login failed: {e}")
    return None


def api_get(path: str, params: dict = None, auth: bool = False) -> dict:
    """
    GET request tới BASE_URL. Returns dict (JSON body) hoặc {}.
    auth=True → tự động login nếu chưa có token.
    """
    if auth and not _auth_token:
        login()
    try:
        resp = get_session().get(
            f"{BASE_URL}{path}",
            params=params,
            timeout=REQUEST_TIMEOUT
        )
        resp.raise_for_status()
        return resp.json()
    except _requests.exceptions.HTTPError as e:
        try:
            return {"_error": str(e), "_status": e.response.status_code,
                    **e.response.json()}
        except Exception:
            return {"_error": str(e), "_status": getattr(e.response, "status_code", 0)}
    except Exception as e:
        return {"_error": str(e), "_status": 0}


def api_get_status(path: str, params: dict = None, auth: bool = False) -> tuple:
    """GET request → (status_code: int, data: dict)."""
    if auth and not _auth_token:
        login()
    try:
        resp = get_session().get(
            f"{BASE_URL}{path}",
            params=params,
            timeout=REQUEST_TIMEOUT
        )
        try:
            data = resp.json()
        except Exception:
            data = {"_raw": resp.text[:200]}
        return resp.status_code, data
    except _requests.exceptions.ConnectionError:
        return 0, {"_error": f"Connection refused: {BASE_URL}"}
    except _requests.exceptions.Timeout:
        return 0, {"_error": f"Timeout after {REQUEST_TIMEOUT}s"}
    except Exception as e:
        return 0, {"_error": str(e)}


def api_post(path: str, body: dict = None, auth: bool = False) -> dict:
    """POST request → dict (JSON body)."""
    if auth and not _auth_token:
        login()
    try:
        resp = get_session().post(
            f"{BASE_URL}{path}",
            json=body or {},
            timeout=REQUEST_TIMEOUT
        )
        try:
            return resp.json()
        except Exception:
            return {"_status": resp.status_code, "_raw": resp.text[:200]}
    except Exception as e:
        return {"_error": str(e), "_status": 0}


def ssh_exec_server(cmd: str, timeout: int = 30) -> tuple:
    """
    Execute shell command trên server (chỉ dùng trong 05_verify_stores.py).
    Returns (rc, stdout, stderr).
    """
    import subprocess
    full = f"ssh {SSH_USER}@{SERVER_IP} '{cmd}'"
    result = subprocess.run(full, shell=True, capture_output=True, text=True, timeout=timeout)
    return result.returncode, result.stdout.strip(), result.stderr.strip()


def print_env_summary():
    """In cấu hình đang dùng."""
    info(f"Config:   {DOTENV_PATH}")
    info(f"Base URL: {BASE_URL}")
    info(f"Admin:    {ADMIN_EMAIL}")
    info(f"Postgres: {POSTGRES_USER}@<server>/{POSTGRES_DB}")
    info(f"MongoDB:  {MONGO_DB}")


# ── Terminal colors ────────────────────────────────────────────────────────────
class C:
    GREEN  = "\033[92m"
    RED    = "\033[91m"
    YELLOW = "\033[93m"
    CYAN   = "\033[96m"
    BOLD   = "\033[1m"
    DIM    = "\033[2m"
    RESET  = "\033[0m"


def ok(msg):   print(f"  {C.GREEN}✓{C.RESET} {msg}")
def fail(msg): print(f"  {C.RED}✗{C.RESET} {msg}")
def warn(msg): print(f"  {C.YELLOW}⚠{C.RESET} {msg}")
def info(msg): print(f"  {C.CYAN}→{C.RESET} {msg}")
def head(msg): print(f"\n{C.BOLD}{C.CYAN}{'='*60}{C.RESET}\n{C.BOLD}  {msg}{C.RESET}\n{C.BOLD}{C.CYAN}{'='*60}{C.RESET}")
def sub(msg):  print(f"\n{C.BOLD}── {msg} ──{C.RESET}")
