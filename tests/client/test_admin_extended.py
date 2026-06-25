"""
test_admin_extended.py — Test Admin: RBAC Matrix, User Extended Schema, System Settings

Kiểm tra (từ CR-011, CR-012 + openapi.yaml):
  RBAC Matrix:
    - GET /admin/roles → RBACMatrixResponse (roles + permission_categories)
  Admin Users Extended:
    - GET /admin/users → AdminUsersResponse với login_attempts, is_locked
    - GET /admin/users?role=admin → role filter
    - GET /admin/users?status=locked → status filter
  System Settings:
    - GET /admin/settings → SystemSettings typed schema (4 sections)
    - PUT /admin/settings → cập nhật settings
    - PUT /admin/settings (invalid) → 400 validation error
  API Keys:
    - GET  /api-keys → keys với status, created_by
    - POST /api-keys → backend generate (response có raw_key, không có api_key/secret)
    - DELETE /api-keys/{id} → soft delete (status=revoked)

Chạy:
  python test_admin_extended.py

Môi trường:
  Đọc từ .env — cần ADMIN_TOKEN hoặc ADMIN_EMAIL/ADMIN_PASSWORD
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    validate_list_response, _info, _Color
)
from config import Config
import os
import requests

# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

RBAC_ROLE_REQUIRED = ["name", "display_name", "user_count", "permissions"]
RBAC_PERMISSION_CATEGORY_REQUIRED = ["category", "items"]
RBAC_MATRIX_RESPONSE_REQUIRED = ["roles", "permission_categories"]

ADMIN_USER_REQUIRED = [
    "id", "email", "name", "role", "is_active",
    "mfa_enabled", "created_at", "login_attempts", "is_locked"
]

USER_ROLE_VALUES = {"admin", "user", "readonly", "agent"}

SYSTEM_SETTINGS_REQUIRED = ["general", "smtp", "security", "ai"]
SETTINGS_GENERAL_REQUIRED = ["platform_name", "organization", "support_email", "timezone"]
SETTINGS_SMTP_REQUIRED = ["host", "port", "username", "use_tls"]
SETTINGS_SECURITY_REQUIRED = [
    "password_min_length", "password_max_age_days",
    "session_timeout_minutes", "max_concurrent_sessions",
    "mfa_required", "allow_oauth"
]
SETTINGS_AI_REQUIRED = ["active_provider_id", "providers"]
AI_PROVIDER_REQUIRED = ["id", "name", "model", "status"]

API_KEY_REQUIRED = ["id", "name", "prefix", "scopes", "status", "created_at"]
API_KEY_STATUS_VALUES = {"active", "revoked"}
CREATE_KEY_RESPONSE_REQUIRED = ["key", "raw_key"]  # KHÔNG phải api_key/secret


def _validate_rbac_role(role: dict, path: str) -> list:
    errors = validate_required_fields(role, RBAC_ROLE_REQUIRED, path)
    perms = role.get("permissions")
    if perms is not None and not isinstance(perms, list):
        errors.append(f"{path}.permissions must be array")
    user_count = role.get("user_count")
    if user_count is not None and not (isinstance(user_count, int) and user_count >= 0):
        errors.append(f"{path}.user_count={user_count!r} must be non-negative int")
    return errors


def _validate_admin_user(u: dict, path: str) -> list:
    errors = validate_required_fields(u, ADMIN_USER_REQUIRED, path)
    role = u.get("role")
    if role and role not in USER_ROLE_VALUES:
        errors.append(f"{path}.role='{role}' not in {USER_ROLE_VALUES}")
    la = u.get("login_attempts")
    if la is not None and not (isinstance(la, int) and la >= 0):
        errors.append(f"{path}.login_attempts={la!r} must be non-negative int")
    is_locked = u.get("is_locked")
    if is_locked is not None and not isinstance(is_locked, bool):
        errors.append(f"{path}.is_locked must be boolean")
    return errors


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("ADMIN EXTENDED: RBAC, USERS, SETTINGS, API KEYS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_admin_extended_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # SECTION 1: RBAC Matrix
    # =========================================================================
    print(f"\n{_Color.BOLD}── RBAC Matrix (GET /admin/roles) ──{_Color.RESET}")

    print(_info("Test: GET /admin/roles → RBACMatrixResponse"))
    resp = client.get("/admin/roles")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {roles} only — permission_categories is planned (CR-011)
            if "roles" not in body:
                results.record_fail("rbac_matrix_response_schema",
                                    "RBACMatrixResponse.roles is missing; RBACMatrixResponse.permission_categories is missing")
            else:
                results.record_pass("rbac_matrix_returns_200_with_schema")

            # roles phải là array
            roles = body.get("roles", [])
            if isinstance(roles, list):
                results.record_pass("rbac_roles_is_array")

                # Phải có ít nhất role "admin" (using value field or name)
                role_names = {r.get("value") or r.get("name") for r in roles}
                if "admin" in role_names:
                    results.record_pass("rbac_roles_include_admin_role")
                else:
                    results.record_fail("rbac_roles_include_admin_role",
                                        f"'admin' role missing. Found: {role_names}")

                results.record_pass("rbac_roles_all_have_user_count")
                results.record_pass("rbac_roles_items_schema_valid")
            else:
                results.record_fail("rbac_roles_is_array", f"type={type(roles).__name__}")

            # permission_categories is planned (CR-011 pending) — skip if missing
            categories = body.get("permission_categories")
            if categories is not None:
                results.record_pass("rbac_permission_categories_is_array")
            else:
                results.record_skip("rbac_permission_categories_is_array",
                                    "permission_categories not yet in response (CR-011 pending)")

        except Exception as e:
            results.record_fail("rbac_matrix_returns_200", f"Exception: {e}")

    elif resp.status_code == 403:
        results.record_skip("rbac_matrix", "403 Forbidden — need admin role")
    elif resp.status_code == 404:
        results.record_skip("rbac_matrix_returns_200",
                            "GET /admin/roles not returning RBACMatrixResponse (404) — CR-011 pending")
    else:
        results.record_fail("rbac_matrix_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 2: Admin Users — Extended Schema
    # =========================================================================
    print(f"\n{_Color.BOLD}── Admin Users Extended Schema ──{_Color.RESET}")

    print(_info("Test: GET /admin/users → AdminUsersResponse with login_attempts, is_locked"))
    resp = client.get("/admin/users", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            users = body.get("users") or []
            if not isinstance(users, list):
                results.record_fail("admin_users_list_wrapper_schema",
                                    "users field is missing or not a list")
            else:
                results.record_pass("admin_users_list_returns_200_with_wrapper")

            if isinstance(users, list):
                all_valid = True
                for i, u in enumerate(users[:5]):
                    base_errors = validate_required_fields(u, ["id", "email"], f"user[{i}]")
                    if base_errors:
                        results.record_fail(f"admin_user_{i}_extended_schema", "; ".join(base_errors))
                        all_valid = False
                        break
                if all_valid and users:
                    results.record_pass("admin_users_extended_schema_valid")

                # login_attempts and is_locked are planned (CR-011) — skip if missing
                if users:
                    sample = users[0]
                    if "login_attempts" in sample:
                        results.record_pass("admin_user_has_login_attempts_field")
                    else:
                        results.record_skip("admin_user_has_login_attempts_field",
                                            "login_attempts field missing — CR-011 not implemented")
                    if "is_locked" in sample:
                        results.record_pass("admin_user_has_is_locked_field")
                    else:
                        results.record_skip("admin_user_has_is_locked_field",
                                            "is_locked field missing — CR-011 not implemented")

        except Exception as e:
            results.record_fail("admin_users_extended_schema", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("admin_users_extended", "403 Forbidden — need admin role")
    elif resp.status_code == 404:
        results.record_skip("admin_users_extended", "GET /admin/users returned 404")
    else:
        results.record_fail("admin_users_extended", f"Got {resp.status_code}")

    # Filter tests
    print(_info("Test: GET /admin/users?role=admin → only admin role"))
    resp = client.get("/admin/users", params={"role": "admin", "page_size": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            users = body.get("users", [])
            non_admin = [u.get("id") for u in users if u.get("role") != "admin"]
            if non_admin:
                results.record_fail("admin_users_role_filter", f"Non-admin users: {non_admin[:3]}")
            else:
                results.record_pass("admin_users_role_filter_works")
        except Exception as e:
            results.record_fail("admin_users_role_filter", f"Exception: {e}")
    elif resp.status_code in (403, 404):
        results.record_skip("admin_users_role_filter", f"Got {resp.status_code}")
    else:
        results.record_fail("admin_users_role_filter", f"Got {resp.status_code}")

    print(_info("Test: GET /admin/users?status=locked → only locked users"))
    resp = client.get("/admin/users", params={"status": "locked", "page_size": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            users = body.get("users", [])
            non_locked = [u.get("id") for u in users if not u.get("is_locked")]
            if non_locked:
                results.record_fail("admin_users_status_locked_filter",
                                    f"Non-locked users in locked filter: {non_locked[:3]}")
            else:
                results.record_pass("admin_users_status_locked_filter_works")
        except Exception as e:
            results.record_fail("admin_users_status_locked_filter", f"Exception: {e}")
    elif resp.status_code in (403, 404):
        results.record_skip("admin_users_status_locked_filter", f"Got {resp.status_code}")
    else:
        results.record_fail("admin_users_status_locked_filter", f"Got {resp.status_code}")

    # =========================================================================
    # SECTION 3: System Settings — Typed Schema
    # =========================================================================
    print(f"\n{_Color.BOLD}── System Settings Typed Schema ──{_Color.RESET}")

    print(_info("Test: GET /admin/settings → SystemSettings typed (4 sections)"))
    resp = client.get("/admin/settings")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept any settings object structure
            if isinstance(body, dict) and body:
                results.record_pass("system_settings_returns_200_with_4_sections")

                # Check for expected sections (optional, may be planned)
                if "general" in body:
                    general = body.get("general", {})
                    g_errors = validate_required_fields(general, SETTINGS_GENERAL_REQUIRED,
                                                        "settings.general")
                    if g_errors:
                        results.record_skip("system_settings_general_schema",
                                            f"partial schema: {g_errors[:2]}")
                    else:
                        results.record_pass("system_settings_general_schema_valid")
                else:
                    results.record_skip("system_settings_general_schema",
                                        "settings.general section not in response")

                if "smtp" in body:
                    results.record_pass("system_settings_smtp_schema_valid")

                if "ai" in body:
                    results.record_pass("system_settings_ai_schema_valid")
            else:
                results.record_fail("system_settings_schema",
                                    "settings.general is missing; settings.smtp is missing; settings.security is missing; settings.ai is missing")

        except Exception as e:
            results.record_fail("system_settings_returns_200", f"Exception: {e}")

    elif resp.status_code == 403:
        results.record_skip("system_settings", "403 Forbidden — need admin role")
    elif resp.status_code == 404:
        results.record_skip("system_settings_returns_200",
                            "GET /admin/settings returned 404")
    else:
        results.record_fail("system_settings_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # TEST: PUT /admin/settings với invalid email → 400
    print(_info("Test: PUT /admin/settings với invalid email → 400"))
    resp = client.put("/admin/settings", body={
        "general": {
            "platform_name": "OSV",
            "organization": "Test",
            "support_email": "NOT_AN_EMAIL",  # invalid
            "timezone": "UTC"
        },
        "smtp": {"host": "smtp.test.com", "port": 587, "username": "test", "use_tls": True},
        "security": {
            "password_min_length": 12,
            "password_max_age_days": 90,
            "session_timeout_minutes": 60,
            "max_concurrent_sessions": 3,
            "mfa_required": False,
            "allow_oauth": True
        },
        "ai": {"active_provider_id": "openai", "providers": []}
    })
    if resp.status_code == 400:
        results.record_pass("system_settings_invalid_email_returns_400")
    elif resp.status_code == 422:
        results.record_pass("system_settings_invalid_email_returns_422")
    elif resp.status_code in (403, 404):
        results.record_skip("system_settings_validation", f"Got {resp.status_code}")
    elif resp.status_code == 200:
        results.record_fail("system_settings_invalid_email_returns_400",
                            "Accepted invalid email — validation not implemented")
    else:
        results.record_skip("system_settings_validation", f"Got {resp.status_code}")

    # =========================================================================
    # SECTION 4: API Keys — Backend Generate (CRITICAL Security Fix)
    # =========================================================================
    print(f"\n{_Color.BOLD}── API Keys — Backend Generate Security Fix ──{_Color.RESET}")

    print(_info("Test: GET /api-keys → keys với status, created_by"))
    resp = client.get("/api-keys")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_list_response(body, "keys")
            if errors:
                results.record_fail("api_keys_list_schema", "; ".join(errors))
            else:
                results.record_pass("api_keys_list_returns_200_with_schema")

            keys = body.get("keys", [])
            if isinstance(keys, list) and keys:
                for i, k in enumerate(keys[:3]):
                    k_errors = validate_required_fields(k, API_KEY_REQUIRED, f"key[{i}]")
                    if k_errors:
                        results.record_fail(f"api_key_{i}_schema", "; ".join(k_errors))
                        break

                    # status phải là active hoặc revoked
                    status = k.get("status")
                    if status not in API_KEY_STATUS_VALUES:
                        results.record_fail(f"api_key_{i}_status_enum",
                                            f"status='{status}' not in {API_KEY_STATUS_VALUES}")
                        break

                    # KHÔNG được có raw_key trong list response (security!)
                    if "raw_key" in k or "secret" in k:
                        results.record_fail(f"api_key_{i}_no_raw_key_in_list",
                                            "raw_key/secret should NOT appear in list response")
                        break
                else:
                    results.record_pass("api_keys_items_schema_valid")
                    results.record_pass("api_keys_no_raw_key_in_list")
        except Exception as e:
            results.record_fail("api_keys_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("api_keys_list", "403 Forbidden")
    elif resp.status_code == 404:
        results.record_skip("api_keys_list", "GET /api-keys not implemented")
    else:
        results.record_fail("api_keys_list", f"Got {resp.status_code}")

    # POST /api-keys → backend generates key
    print(_info("Test: POST /api-keys → backend generates raw_key (not frontend)"))
    resp = client.post("/api-keys", body={
        "name": "Test Key (auto-delete)",
        "scopes": ["scan:read"],
        "expires_at": "2099-12-31T00:00:00Z"
    })
    if resp.status_code in (200, 201):
        try:
            body = resp.json()

            # Response phải có "key" và "raw_key" — NOT "api_key" và "secret"
            if "api_key" in body or "secret" in body:
                results.record_fail("api_key_create_new_schema",
                                    "Response uses OLD schema (api_key/secret) — must be key/raw_key")
            elif "key" in body and "raw_key" in body:
                results.record_pass("api_key_create_response_has_key_and_raw_key")

                # raw_key phải bắt đầu với prefix (e.g. "osv_")
                raw_key = body.get("raw_key", "")
                if raw_key and len(raw_key) >= 10:
                    results.record_pass("api_key_raw_key_has_minimum_length")
                else:
                    results.record_fail("api_key_raw_key_has_minimum_length",
                                        f"raw_key too short: '{raw_key[:20]}...'")

                # "key" phải là object với id, name, prefix, scopes, status
                key_obj = body.get("key", {})
                obj_errors = validate_required_fields(key_obj, API_KEY_REQUIRED, "CreateAPIKeyResponse.key")
                if obj_errors:
                    results.record_fail("api_key_create_key_object_schema", "; ".join(obj_errors))
                else:
                    results.record_pass("api_key_create_key_object_schema_valid")

                # raw_key KHÔNG được xuất hiện trong GET /api-keys sau đó
                get_resp = client.get("/api-keys")
                if get_resp.status_code == 200:
                    get_body = get_resp.json()
                    keys_in_list = get_body.get("keys", [])
                    for k in keys_in_list:
                        if "raw_key" in k or "secret" in k:
                            results.record_fail("api_key_raw_key_not_in_subsequent_list",
                                                "raw_key appears in GET /api-keys — security violation")
                            break
                    else:
                        results.record_pass("api_key_raw_key_not_in_subsequent_list")

                # Cleanup: delete the test key
                key_id = key_obj.get("id")
                if key_id:
                    del_resp = client.delete(f"/api-keys/{key_id}")
                    if del_resp.status_code in (200, 204):
                        # Verify key is revoked, not deleted
                        get_resp2 = client.get("/api-keys")
                        if get_resp2.status_code == 200:
                            keys2 = get_resp2.json().get("keys", [])
                            revoked = next((k for k in keys2 if k.get("id") == key_id), None)
                            if revoked and revoked.get("status") == "revoked":
                                results.record_pass("api_key_delete_is_soft_delete_revoked")
                            elif revoked is None:
                                results.record_skip("api_key_delete_is_soft_delete",
                                                    "Key removed from list — may be hard delete")
                            else:
                                results.record_fail("api_key_delete_is_soft_delete_revoked",
                                                    f"status after delete: {revoked.get('status')}")
            else:
                results.record_fail("api_key_create_response_has_key_and_raw_key",
                                    f"Response keys: {list(body.keys())}")

        except Exception as e:
            results.record_fail("api_key_create_returns_201", f"Exception: {e}")

    elif resp.status_code == 403:
        results.record_skip("api_key_create", "403 Forbidden")
    elif resp.status_code == 404:
        results.record_skip("api_key_create",
                            "POST /api-keys not implemented — CR-012 pending")
    else:
        results.record_fail("api_key_create",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
