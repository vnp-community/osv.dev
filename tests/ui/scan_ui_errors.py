#!/usr/bin/env python3
"""
OSV Platform — UI Console Error Scanner v3
Architecture:
  - Login 1 lần via page.goto('/login')
  - Sau login: dùng window.history.pushState() + popstate event để trigger
    React Router navigate MÀ KHÔNG reload trang => accessToken in-memory được giữ
  - Thu thập console errors cho mỗi trang
  - Phân loại: REAL_ERROR (JS/app crash) vs AUTH_WARN (401 background API) vs OK
"""

import asyncio
import json
import re
from datetime import datetime
from pathlib import Path

from playwright.async_api import async_playwright, Page, ConsoleMessage

CHROME_EXEC = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

BASE_URL    = "https://c12.openledger.vn"
ADMIN_EMAIL = "admin@openvulnscan.io"
ADMIN_PASS  = "Admin@123!ChangeMe"
OUT_DIR     = Path("/Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/ui-crash")

SETTLE_MS   = 3500   # ms chờ sau client-side nav để components mount + API calls settle
LOGIN_MS    = 20000  # timeout cho login flow

PAGES = [
    ("/dashboard",                "Dashboard — Main"),
    ("/dashboard/executive",      "Dashboard — Executive Overview"),
    ("/dashboard/risk",           "Dashboard — Risk Overview"),
    ("/dashboard/sla",            "Dashboard — SLA Dashboard"),
    ("/cve/search",               "CVE Intel — CVE Search"),
    ("/cve/kev",                  "CVE Intel — KEV Catalog"),
    ("/cve/semantic",             "CVE Intel — Semantic Search"),
    ("/cve/epss",                 "CVE Intel — EPSS Analytics"),
    ("/cve/vendors",              "CVE Intel — Vendor Catalog"),
    ("/cve/cwe",                  "CVE Intel — CWE Library"),
    ("/cve/capec",                "CVE Intel — CAPEC Library"),
    ("/scans",                    "Scanning — Scan Dashboard"),
    ("/scans/new",                "Scanning — New Scan Wizard"),
    ("/scans/running",            "Scanning — Running Scans"),
    ("/scans/history",            "Scanning — Scan History"),
    ("/findings",                 "Findings — All Findings"),
    ("/findings/risk-acceptance", "Findings — Risk Acceptance Center"),
    ("/assets",                   "Assets — Asset Inventory"),
    ("/products",                 "Product Security"),
    ("/ai/triage",                "AI Center — AI Triage Queue"),
    ("/ai/enrichment",            "AI Center — AI Enrichment"),
    ("/reports",                  "Reports — Report Center"),
    ("/notifications",            "Notifications — Notification Center"),
    ("/integrations/api-keys",    "Integrations — API Key Management"),
    ("/integrations/webhooks",    "Integrations — Webhook Events"),
    ("/admin/users",              "Admin — User Management"),
    ("/admin/roles",              "Admin — RBAC Management"),
    ("/admin/audit",              "Admin — Audit Logs"),
    ("/admin/health",             "Admin — System Health"),
    ("/admin/settings",           "Admin — System Settings"),
    ("/profile",                  "User — Profile"),
    ("/onboarding",               "User — Onboarding"),
]

# Patterns noise (bỏ qua)
NOISE_RE = re.compile(
    r"Download the React DevTools|"
    r"\[HMR\]|"
    r"favicon\.ico|"
    r"React Router Future Flag|"
    r"source map",
    re.IGNORECASE
)

# Background API auth failures — không phải lỗi app
AUTH_RE = re.compile(
    r"401\s*\(\)|403\s*\(\)|"
    r"/auth/(me|refresh)|"
    r"/public/stats",
    re.IGNORECASE
)


def slug(path: str) -> str:
    return path.strip("/").replace("/", "-") or "root"


async def login(page: Page) -> bool:
    """Full page load login, trả về True nếu thành công."""
    print(f"  → goto {BASE_URL}/login")
    await page.goto(f"{BASE_URL}/login", timeout=LOGIN_MS, wait_until="domcontentloaded")
    await page.wait_for_load_state("networkidle", timeout=LOGIN_MS)

    # Fill email
    for sel in ["input[type='email']", "input[name='email']", "input[placeholder*='email' i]"]:
        el = page.locator(sel)
        if await el.count() > 0:
            await el.first.fill(ADMIN_EMAIL)
            break

    # Fill password
    for sel in ["input[type='password']", "input[name='password']"]:
        el = page.locator(sel)
        if await el.count() > 0:
            await el.first.fill(ADMIN_PASS)
            break

    # Submit
    for sel in ["button[type='submit']", "button:text-is('Sign in')", "button:text-is('Login')"]:
        el = page.locator(sel)
        if await el.count() > 0:
            await el.first.click()
            break

    try:
        await page.wait_for_url("**/dashboard**", timeout=12000)
        # Chờ app fully hydrate sau login
        await page.wait_for_load_state("networkidle", timeout=10000)
        await page.wait_for_timeout(1500)
        print(f"  ✓ Login OK → {page.url}")
        return True
    except Exception as e:
        print(f"  ✗ Login FAIL: {e} | url={page.url}")
        return False


async def client_navigate(page: Page, path: str) -> None:
    """
    Trigger React Router navigation WITHOUT full page reload.
    Dùng pushState + popstate event => React Router picks up new path,
    accessToken in-memory được giữ nguyên.
    """
    js = f"""
    (function() {{
        window.history.pushState(null, '', {json.dumps(path)});
        window.dispatchEvent(new PopStateEvent('popstate', {{ state: null }}));
    }})();
    """
    await page.evaluate(js)


async def check_page(page: Page, path: str, label: str) -> dict:
    """Navigate client-side, thu thập errors, phân loại."""
    real_errors = []
    auth_errors = []
    pageerrors  = []
    all_logs    = []

    def on_console(msg: ConsoleMessage):
        text = msg.text
        if NOISE_RE.search(text):
            return
        loc = str(msg.location)
        entry = {"type": msg.type, "text": text, "location": loc}
        all_logs.append(entry)
        if msg.type == "error":
            if AUTH_RE.search(text + loc):
                auth_errors.append(entry)
            else:
                real_errors.append(entry)

    def on_pageerror(exc):
        entry = {"type": "pageerror", "text": str(exc), "location": ""}
        all_logs.append(entry)
        pageerrors.append(entry)

    page.on("console", on_console)
    page.on("pageerror", on_pageerror)

    await client_navigate(page, path)
    await page.wait_for_timeout(SETTLE_MS)

    # Kiểm tra có bị redirect về login không
    current_url = page.url
    session_lost = "/login" in current_url

    page.remove_listener("console", on_console)
    page.remove_listener("pageerror", on_pageerror)

    all_real = real_errors + pageerrors
    if session_lost:
        status = "SESSION_LOST"
    elif all_real:
        status = "ERROR"
    elif auth_errors:
        status = "AUTH_WARN"
    else:
        status = "OK"

    icons = {"OK": "✓", "AUTH_WARN": "~", "ERROR": "✗", "SESSION_LOST": "🔒"}
    icon = icons.get(status, "?")
    print(f"  {icon} [{status:12s}] {label} — {len(all_real)} real, {len(auth_errors)} auth-401")

    return {
        "path":             path,
        "label":            label,
        "url":              f"{BASE_URL}{path}",
        "final_url":        current_url,
        "session_lost":     session_lost,
        "status":           status,
        "real_error_count": len(all_real),
        "auth_error_count": len(auth_errors),
        "real_errors":      all_real,
        "auth_errors":      auth_errors,
        "all_logs":         all_logs,
    }


def write_bug_report(r: dict, out_dir: Path, run_ts: str) -> Path:
    fname = out_dir / f"BUG-{slug(r['path'])}.md"
    lines = [
        f"# Bug: {r['label']}",
        "",
        f"| Field | Value |",
        f"|-------|-------|",
        f"| **Route** | `{r['path']}` |",
        f"| **URL** | {r['url']} |",
        f"| **Final URL** | {r['final_url']} |",
        f"| **Status** | `{r['status']}` |",
        f"| **Real Errors** | {r['real_error_count']} |",
        f"| **Auth 401/403** | {r['auth_error_count']} |",
        f"| **Recorded** | {run_ts} |",
        "",
    ]

    if r["session_lost"]:
        lines += [
            "## ⚠️ Session Lost",
            "",
            "Trang bị redirect về `/login`. Khả năng httpOnly cookie bị expire hoặc server CORS issue.",
            "",
        ]

    if r["real_errors"]:
        lines += [f"## ❌ Real Errors ({r['real_error_count']})", ""]
        for i, e in enumerate(r["real_errors"], 1):
            lines += [
                f"### Error #{i} — `{e['type']}`",
                f"- **Location**: `{e['location']}`",
                "- **Message**:",
                "  ```",
                f"  {e['text']}",
                "  ```",
                "",
            ]

    if r["auth_errors"]:
        lines += [
            f"## ⚠️ Background Auth Errors 401/403 ({r['auth_error_count']})",
            "",
            "_Background API polling thất bại — thường không ảnh hưởng trang nếu render OK._",
            "",
        ]
        for e in r["auth_errors"]:
            lines.append(f"- `{e['text']}` — `{e['location']}`")
        lines.append("")

    fname.write_text("\n".join(lines), encoding="utf-8")
    return fname


def write_summary(results: list, out_dir: Path, run_ts: str) -> Path:
    counts = {s: sum(1 for r in results if r["status"] == s)
              for s in ("OK", "AUTH_WARN", "ERROR", "SESSION_LOST")}

    lines = [
        "# OSV Platform — UI Scan Report",
        "",
        f"**Scanned**: {run_ts}  ",
        f"**Base URL**: {BASE_URL}  ",
        f"**Total pages**: {len(results)}  ",
        f"**Method**: React Router client-side navigation (no reload)",
        "",
        "| Status | Count | Meaning |",
        "|--------|-------|---------|",
        f"| ✅ OK | {counts['OK']} | Trang render OK, không lỗi |",
        f"| ⚠️ AUTH_WARN | {counts['AUTH_WARN']} | Chỉ có 401 background API — trang render OK |",
        f"| ❌ ERROR | {counts['ERROR']} | Lỗi JavaScript/app thực sự |",
        f"| 🔒 SESSION_LOST | {counts['SESSION_LOST']} | Bị redirect /login |",
        "",
        "---",
        "",
        "## Kết quả chi tiết",
        "",
        "| # | Page | Route | Status | Real Err | Auth 401 |",
        "|---|------|-------|--------|----------|----------|",
    ]

    for i, r in enumerate(results, 1):
        icon = {"OK": "✅", "AUTH_WARN": "⚠️", "ERROR": "❌", "SESSION_LOST": "🔒"}.get(r["status"], "?")
        lines.append(
            f"| {i} | {r['label']} | `{r['path']}` | {icon} `{r['status']}` "
            f"| {r['real_error_count']} | {r['auth_error_count']} |"
        )

    # Bugs
    bugs = [r for r in results if r["status"] in ("ERROR", "SESSION_LOST")]
    lines += ["", "---", "", "## ❌ Bugs (Real Errors & Session Issues)", ""]
    if bugs:
        for r in bugs:
            lines.append(f"- **{r['label']}** `{r['path']}` → [BUG-{slug(r['path'])}.md](BUG-{slug(r['path'])}.md)")
            for e in r["real_errors"][:3]:
                lines.append(f"  - `{e['text'][:100]}`")
    else:
        lines.append("_Không có lỗi thực sự._")

    # Auth warns
    warns = [r for r in results if r["status"] == "AUTH_WARN"]
    lines += ["", "## ⚠️ Auth Warn (401 background only)", ""]
    if warns:
        for r in warns:
            lines.append(f"- **{r['label']}** `{r['path']}` — {r['auth_error_count']} lần 401")
    else:
        lines.append("_Không có._")

    summary_path = out_dir / "SUMMARY.md"
    summary_path.write_text("\n".join(lines), encoding="utf-8")
    (out_dir / "scan_results.json").write_text(
        json.dumps(results, indent=2, ensure_ascii=False), encoding="utf-8"
    )
    return summary_path


async def main():
    run_ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    print(f"OSV UI Scanner v3 — {run_ts}")
    print(f"Target: {BASE_URL}")
    print(f"Output: {OUT_DIR}")
    print("=" * 60)

    async with async_playwright() as pw:
        browser = await pw.chromium.launch(
            headless=True,
            executable_path=CHROME_EXEC,
        )
        context = await browser.new_context(
            ignore_https_errors=True,
            viewport={"width": 1440, "height": 900},
        )
        page = await context.new_page()

        print("\n[1] Login...")
        if not await login(page):
            print("ABORT: Login thất bại.")
            await browser.close()
            return

        print(f"\n[2] Scanning {len(PAGES)} pages (client-side nav)...\n")
        results = []
        for path, label in PAGES:
            r = await check_page(page, path, label)
            results.append(r)
            if r["status"] not in ("OK", "AUTH_WARN"):
                write_bug_report(r, OUT_DIR, run_ts)
            # Nếu session mất, re-login và tiếp tục
            if r["session_lost"]:
                print("    → Re-login sau khi session mất...")
                await login(page)

        await browser.close()

    print("\n[3] Writing results...")
    summary = write_summary(results, OUT_DIR, run_ts)

    ok   = sum(1 for r in results if r["status"] == "OK")
    aw   = sum(1 for r in results if r["status"] == "AUTH_WARN")
    err  = sum(1 for r in results if r["status"] == "ERROR")
    sl   = sum(1 for r in results if r["status"] == "SESSION_LOST")
    print(f"\n{'='*60}")
    print(f"Done: ✅ {ok} OK | ⚠️ {aw} AUTH_WARN | ❌ {err} ERROR | 🔒 {sl} SESSION_LOST")
    print(f"Summary: {summary}")
    print("=" * 60)


if __name__ == "__main__":
    asyncio.run(main())
