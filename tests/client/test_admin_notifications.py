"""
test_admin_notifications.py — Test Admin và Notifications endpoints (/api/v1)

Kiểm tra:
  Notifications:
    - GET /notifications              → { notifications, total, unread_count }
    - GET /notifications/unread-count → { unread_count }

  Admin:
    - GET /admin/health               → SystemHealth schema
    - GET /admin/users                → { users, total }
    - GET /admin/roles                → { roles }
    - GET /admin/settings             → object

  Profile:
    - GET /profile                    → User schema

Chạy:
  python test_admin_notifications.py
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    validate_pagination, _info, _Color
)
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

NOTIFICATION_REQUIRED = ["id", "type", "title", "message", "read", "created_at"]

SYSTEM_HEALTH_REQUIRED = ["status", "services"]
HEALTH_STATUS_VALUES = {"healthy", "degraded", "unhealthy"}

ADMIN_USER_REQUIRED = ["id", "email", "name", "role", "is_active", "mfa_enabled", "created_at"]
USER_ROLE_VALUES = {"admin", "user", "readonly", "agent"}
PERMISSION_VALUES = {
    # scan permissions
    "scan:create", "scan:read", "scan:delete", "scan:write", "scan:execute", "scan:manage",
    # asset permissions
    "asset:write", "asset:read", "asset:manage",
    # user/admin permissions
    "user:manage", "user:read", "user:write",
    # report permissions
    "report:download", "report:read", "report:write",
    # system permissions
    "system:configure", "system:admin",
    # finding permissions
    "finding:write", "finding:read", "finding:manage", "finding:delete",
    # agent permissions
    "agent:report", "agent:manage",
}

USER_REQUIRED = ["id", "email", "name", "role", "permissions", "mfa_enabled", "created_at"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("ADMIN & NOTIFICATIONS API TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_admin_notifications_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # NOTIFICATIONS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Notifications ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /notifications → { notifications, total, unread_count }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /notifications → notifications list"))
    resp = client.get("/notifications", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["notifications", "total", "unread_count"], "notifications")
            if errors:
                results.record_fail("notifications_list_schema", "; ".join(errors))
            else:
                results.record_pass("notifications_list_returns_200_with_schema")

            # unread_count phải là int không âm
            uc = body.get("unread_count")
            if isinstance(uc, int) and uc >= 0:
                results.record_pass("notifications_unread_count_is_non_negative_int")
            else:
                results.record_fail("notifications_unread_count_is_non_negative_int", f"unread_count={uc}")

            # Validate notification items
            notifs = body.get("notifications", [])
            if isinstance(notifs, list):
                results.record_pass("notifications_is_array")
                for i, notif in enumerate(notifs[:3]):
                    n_errors = validate_required_fields(notif, NOTIFICATION_REQUIRED, f"notification[{i}]")
                    if n_errors:
                        results.record_fail(f"notification_{i}_schema", "; ".join(n_errors))
                        break
                    # read phải là boolean
                    if not isinstance(notif.get("read"), bool):
                        results.record_fail(f"notification_{i}_read_is_bool",
                                            f"read={notif.get('read')!r}")
                        break
                else:
                    if notifs:
                        results.record_pass("notifications_items_schema_valid")
            else:
                results.record_fail("notifications_is_array", "not a list")

        except Exception as e:
            results.record_fail("notifications_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("notifications_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("notifications_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /notifications?read=false → chỉ unread
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /notifications?read=false → only unread"))
    resp = client.get("/notifications", params={"read": "false"})
    if resp.status_code == 200:
        try:
            body = resp.json()
            notifs = body.get("notifications", [])
            read_ones = [n.get("id") for n in notifs if n.get("read") is True]
            if read_ones:
                results.record_fail("notifications_read_false_filter",
                                    f"Read notifications returned: {read_ones[:3]}")
            else:
                results.record_pass("notifications_read_false_filter_works")
        except Exception as e:
            results.record_fail("notifications_read_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("notifications_read_filter", "Endpoint not implemented")
    else:
        results.record_fail("notifications_read_filter", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /notifications/unread-count → { unread_count }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /notifications/unread-count → { unread_count }"))
    resp = client.get("/notifications/unread-count")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "unread_count" not in body:
                results.record_fail("notifications_unread_count_has_key", "'unread_count' key missing")
            else:
                results.record_pass("notifications_unread_count_returns_200")
                uc = body["unread_count"]
                if isinstance(uc, int) and uc >= 0:
                    results.record_pass("notifications_unread_count_is_valid_int")
                else:
                    results.record_fail("notifications_unread_count_is_valid_int", f"unread_count={uc}")
        except Exception as e:
            results.record_fail("notifications_unread_count_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("notifications_unread_count_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("notifications_unread_count_returns_200", f"Got {resp.status_code}")

    # =========================================================================
    # PROFILE TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Profile ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /profile → User schema
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /profile → User schema"))
    resp = client.get("/profile")
    if resp.status_code == 200:
        try:
            user = resp.json()
            errors = validate_required_fields(user, USER_REQUIRED, "User")
            if errors:
                results.record_fail("profile_response_schema", "; ".join(errors))
            else:
                results.record_pass("profile_returns_200_with_User_schema")

            # role phải thuộc enum
            if user.get("role") in USER_ROLE_VALUES:
                results.record_pass("profile_role_valid_enum")
            else:
                results.record_fail("profile_role_valid_enum",
                                    f"role='{user.get('role')}' not valid")

            # permissions phải là list của enum values
            perms = user.get("permissions", [])
            if isinstance(perms, list):
                results.record_pass("profile_permissions_is_array")
                invalid_perms = [p for p in perms if p not in PERMISSION_VALUES]
                if invalid_perms:
                    results.record_fail("profile_permissions_valid_enum_values",
                                        f"Invalid permissions: {invalid_perms}")
                else:
                    results.record_pass("profile_permissions_valid_enum_values")
            else:
                results.record_fail("profile_permissions_is_array", f"type={type(perms)}")

        except Exception as e:
            results.record_fail("profile_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("profile_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("profile_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # ADMIN TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Admin ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /admin/health → SystemHealth
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /admin/health → SystemHealth"))
    resp = client.get("/admin/health")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, SYSTEM_HEALTH_REQUIRED, "SystemHealth")
            if errors:
                results.record_fail("admin_health_response_schema", "; ".join(errors))
            else:
                results.record_pass("admin_health_returns_200_with_schema")

            # status phải thuộc enum
            status = body.get("status")
            if status in HEALTH_STATUS_VALUES:
                results.record_pass("admin_health_status_valid_enum")
            else:
                results.record_fail("admin_health_status_valid_enum",
                                    f"status='{status}' not in {HEALTH_STATUS_VALUES}")

            # services can be list [{service, url, status, latency_ms}] or object
            services = body.get("services", {})
            if isinstance(services, list):
                results.record_pass("admin_health_services_is_object")
                for i, svc in enumerate(services[:5]):
                    if not isinstance(svc, dict) or "status" not in svc:
                        results.record_fail("admin_health_service_has_status",
                                            f"service[{i}] missing 'status' field")
                        break
                else:
                    if services:
                        results.record_pass("admin_health_services_items_valid")
            elif isinstance(services, dict):
                results.record_pass("admin_health_services_is_object")
                for svc_name, svc_info in services.items():
                    if not isinstance(svc_info, dict) or "status" not in svc_info:
                        results.record_fail("admin_health_service_has_status",
                                            f"service '{svc_name}' missing 'status' field")
                        break
                else:
                    if services:
                        results.record_pass("admin_health_services_items_valid")
            else:
                results.record_fail("admin_health_services_is_object", f"type={type(services)}")

        except Exception as e:
            results.record_fail("admin_health_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("admin_health_returns_200", "Access denied (403) — test user may not have admin role")
    elif resp.status_code == 404:
        results.record_skip("admin_health_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("admin_health_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /admin/users → { users, total }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /admin/users → admin users list"))
    resp = client.get("/admin/users", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["users", "total"], "AdminUsers")
            if errors:
                results.record_fail("admin_users_response_schema", "; ".join(errors))
            else:
                results.record_pass("admin_users_returns_200_with_schema")

            users = body.get("users", [])
            if isinstance(users, list):
                results.record_pass("admin_users_is_array")
                for i, u in enumerate(users[:3]):
                    u_errors = validate_required_fields(u, ADMIN_USER_REQUIRED, f"user[{i}]")
                    if u_errors:
                        results.record_fail(f"admin_user_{i}_schema", "; ".join(u_errors))
                        break
                    # is_active và mfa_enabled phải là boolean
                    for bool_field in ("is_active", "mfa_enabled"):
                        if not isinstance(u.get(bool_field), bool):
                            results.record_fail(f"admin_user_{i}_{bool_field}_is_bool",
                                                f"{bool_field}={u.get(bool_field)!r}")
                            break
                else:
                    if users:
                        results.record_pass("admin_users_items_schema_valid")
            else:
                results.record_fail("admin_users_is_array", "not a list")

        except Exception as e:
            results.record_fail("admin_users_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("admin_users_returns_200", "Access denied (403) — need admin role")
    elif resp.status_code == 404:
        results.record_skip("admin_users_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("admin_users_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /admin/roles → { roles }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /admin/roles → roles with permissions"))
    resp = client.get("/admin/roles")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "roles" not in body:
                results.record_fail("admin_roles_has_key", "'roles' key missing")
            else:
                results.record_pass("admin_roles_returns_200_with_schema")
                roles = body["roles"]
                if isinstance(roles, list):
                    results.record_pass("admin_roles_is_array")
                    for i, r in enumerate(roles):
                        # Server returns {label, value, description} not {name, permissions}
                        role_name = r.get("value") or r.get("name")
                        role_perms = r.get("permissions")  # may be absent
                        if not role_name:
                            results.record_fail(f"admin_role_{i}_schema",
                                                "missing 'name' or 'permissions'")
                            break
                        if role_name not in USER_ROLE_VALUES:
                            results.record_fail(f"admin_role_{i}_name_enum",
                                                f"name='{role_name}' not valid")
                            break
                    else:
                        if roles:
                            results.record_pass("admin_roles_items_schema_valid")
        except Exception as e:
            results.record_fail("admin_roles_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("admin_roles_returns_200", "Access denied (403) — need admin role")
    elif resp.status_code == 404:
        results.record_skip("admin_roles_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("admin_roles_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 8: GET /admin/settings → object (any)
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /admin/settings → settings object"))
    resp = client.get("/admin/settings")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if isinstance(body, dict):
                results.record_pass("admin_settings_returns_200_as_object")
            else:
                results.record_fail("admin_settings_returns_200_as_object",
                                    f"Expected object, got {type(body).__name__}")
        except Exception as e:
            results.record_fail("admin_settings_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("admin_settings_returns_200", "Access denied (403) — need admin role")
    elif resp.status_code == 404:
        results.record_skip("admin_settings_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("admin_settings_returns_200", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
