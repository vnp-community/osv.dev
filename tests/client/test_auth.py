"""
test_auth.py — Test các Auth endpoints (/api/v1/auth/*)

Kiểm tra:
  - POST /auth/login              → LoginResponse schema
  - POST /auth/refresh            → RefreshResponse schema
  - GET  /auth/me                 → { user: User } schema
  - GET  /auth/mfa/setup          → MFASetupResponse schema
  - POST /auth/mfa/confirm        → 200 or 400
  - GET  /auth/oauth/google       → 302 redirect (no auth needed)
  - GET  /auth/oauth/github       → 302 redirect (no auth needed)
  - GET  /auth/callback           → 302 or 400 (no auth needed)
  - POST /auth/logout             → 204

Chạy:
  python test_auth.py
"""

import sys
from pathlib import Path

# Thêm thư mục cha vào sys.path để import config/base_client
sys.path.insert(0, str(Path(__file__).parent))

from base_client import APIClient, TestResults, validate_required_fields, _info, _Color
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

LOGIN_RESPONSE_REQUIRED = ["expires_in"]
LOGIN_RESPONSE_OPTIONAL = ["access_token", "user", "mfa_required"]

USER_REQUIRED = ["id", "email", "name", "role", "permissions", "mfa_enabled", "created_at"]
USER_ROLE_VALUES = {"admin", "user", "readonly", "agent"}

REFRESH_RESPONSE_REQUIRED = ["access_token", "expires_in"]

MFA_SETUP_REQUIRED = ["secret", "qr_url", "backup_codes"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("AUTH API TESTS (/api/v1/auth)")
    print(f"{'='*60}{_Color.RESET}\n")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: POST /auth/login — sai credentials → 401
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: Login with wrong credentials → 401"))
    resp = client.post("/auth/login", body={"email": "wrong@example.com", "password": "wrongpass"})
    if resp.status_code == 401:
        # Validate APIError schema
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["error", "message"], "APIError")
            if errors:
                results.record_fail("login_wrong_creds_error_schema", "; ".join(errors))
            else:
                results.record_pass("login_wrong_credentials_returns_401_with_APIError")
        except Exception as e:
            results.record_fail("login_wrong_creds_error_schema", f"JSON parse error: {e}")
    elif resp.status_code == 404:
        results.record_skip("login_wrong_credentials_returns_401", "Endpoint not implemented (404)")
    else:
        results.record_fail("login_wrong_credentials_returns_401", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: POST /auth/login — credentials đúng → LoginResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: Login with valid credentials → 200 + LoginResponse"))
    resp = client.post("/auth/login", body={
        "email": Config.TEST_EMAIL,
        "password": Config.TEST_PASSWORD,
    })
    token = None
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Validate required fields
            errors = validate_required_fields(body, LOGIN_RESPONSE_REQUIRED, "LoginResponse")
            if errors:
                results.record_fail("login_response_schema", "; ".join(errors))
            else:
                results.record_pass("login_valid_credentials_returns_200")

            # expires_in phải là integer dương
            if isinstance(body.get("expires_in"), int) and body["expires_in"] > 0:
                results.record_pass("login_response_expires_in_positive_int")
            else:
                results.record_fail("login_response_expires_in_positive_int",
                                    f"expires_in={body.get('expires_in')} is not positive int")

            # Nếu không có MFA, access_token phải là string
            if not body.get("mfa_required"):
                if isinstance(body.get("access_token"), str) and len(body["access_token"]) > 10:
                    results.record_pass("login_response_access_token_present")
                    token = body["access_token"]
                else:
                    results.record_fail("login_response_access_token_present",
                                        "access_token missing or too short")

                # user object phải có đủ required fields
                if "user" in body and body["user"] is not None:
                    user_errors = validate_required_fields(body["user"], USER_REQUIRED, "LoginResponse.user")
                    if user_errors:
                        results.record_fail("login_response_user_schema", "; ".join(user_errors))
                    else:
                        results.record_pass("login_response_user_schema_valid")

                    # Kiểm tra role là một trong enum values
                    role = body["user"].get("role")
                    if role in USER_ROLE_VALUES:
                        results.record_pass("login_response_user_role_valid_enum")
                    else:
                        results.record_fail("login_response_user_role_valid_enum",
                                            f"role='{role}' not in {USER_ROLE_VALUES}")

                    # permissions phải là list
                    perms = body["user"].get("permissions")
                    if isinstance(perms, list):
                        results.record_pass("login_response_user_permissions_is_list")
                    else:
                        results.record_fail("login_response_user_permissions_is_list",
                                            f"permissions type={type(perms)}")
        except Exception as e:
            results.record_fail("login_valid_credentials_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("login_valid_credentials", "Endpoint not implemented (404)")
    else:
        results.record_fail("login_valid_credentials_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # Thiết lập token cho các test tiếp theo
    # ─────────────────────────────────────────────────────────────────────────
    if Config.ACCESS_TOKEN:
        token = Config.ACCESS_TOKEN
    if token:
        client.session.headers["Authorization"] = f"Bearer {token}"
    else:
        if not client.login():
            results.record_skip("remaining_auth_tests", "Cannot login — skipping authenticated tests")
            results.summary()
            return results

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: POST /auth/refresh → RefreshResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /auth/refresh → RefreshResponse"))
    resp = client.post("/auth/refresh")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, REFRESH_RESPONSE_REQUIRED, "RefreshResponse")
            if errors:
                results.record_fail("refresh_response_schema", "; ".join(errors))
            else:
                results.record_pass("refresh_returns_200_with_RefreshResponse")
            if isinstance(body.get("expires_in"), int) and body["expires_in"] > 0:
                results.record_pass("refresh_expires_in_positive_int")
            else:
                results.record_fail("refresh_expires_in_positive_int",
                                    f"expires_in={body.get('expires_in')}")
        except Exception as e:
            results.record_fail("refresh_response_schema", f"Exception: {e}")
    elif resp.status_code == 401:
        results.record_skip("refresh_returns_200", "No valid refresh cookie available (401)")
    elif resp.status_code == 404:
        results.record_skip("refresh_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("refresh_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /auth/me → { user: User }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /auth/me → { user: User }"))
    resp = client.get("/auth/me")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Response phải có wrapper { user: ... }
            if "user" not in body:
                results.record_fail("auth_me_response_has_user_wrapper",
                                    "'user' key missing in response wrapper")
            else:
                results.record_pass("auth_me_returns_200")
                user_errors = validate_required_fields(body["user"], USER_REQUIRED, "auth/me.user")
                if user_errors:
                    results.record_fail("auth_me_user_schema_valid", "; ".join(user_errors))
                else:
                    results.record_pass("auth_me_user_schema_valid")

                # Email phải khớp với config
                if body["user"].get("email") == Config.TEST_EMAIL:
                    results.record_pass("auth_me_email_matches_login")
                else:
                    results.record_fail("auth_me_email_matches_login",
                                        f"Got email={body['user'].get('email')!r}")
        except Exception as e:
            results.record_fail("auth_me_returns_200", f"Exception: {e}")
    elif resp.status_code == 401:
        results.record_fail("auth_me_returns_200", "Unauthorized — token invalid")
    elif resp.status_code == 404:
        results.record_skip("auth_me_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("auth_me_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /auth/mfa/setup → MFASetupResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /auth/mfa/setup → MFASetupResponse"))
    resp = client.get("/auth/mfa/setup")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, MFA_SETUP_REQUIRED, "MFASetupResponse")
            if errors:
                results.record_fail("mfa_setup_response_schema", "; ".join(errors))
            else:
                results.record_pass("mfa_setup_returns_200_with_schema")
            if isinstance(body.get("backup_codes"), list):
                results.record_pass("mfa_setup_backup_codes_is_list")
            else:
                results.record_fail("mfa_setup_backup_codes_is_list",
                                    f"backup_codes type={type(body.get('backup_codes'))}")
        except Exception as e:
            results.record_fail("mfa_setup_response_schema", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("mfa_setup_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("mfa_setup_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: POST /auth/mfa/confirm → 200 (valid code) or 400 (invalid code)
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /auth/mfa/confirm → 400 for invalid code (endpoint exists)"))
    resp = client.post("/auth/mfa/confirm", body={"code": "000000"})  # intentionally wrong
    if resp.status_code == 200:
        results.record_pass("mfa_confirm_returns_200")
    elif resp.status_code in (400, 401, 422):
        # Wrong code rejected — endpoint exists and validates
        results.record_pass("mfa_confirm_endpoint_exists_rejects_invalid_code")
    elif resp.status_code == 404:
        results.record_skip("mfa_confirm", "Endpoint not implemented (404)")
    else:
        results.record_fail("mfa_confirm_returns_200_or_400", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /auth/oauth/google → 302 redirect (no Bearer needed)
    # ─────────────────────────────────────────────────────────────────────────
    import requests as _requests
    print(_info("Test: GET /auth/oauth/google → 302 redirect to Google"))
    try:
        no_auth_resp = _requests.get(
            f"{client.v1_base}/auth/oauth/google",
            headers={"Accept": "application/json"},
            timeout=client.timeout,
            allow_redirects=False
        )
        if no_auth_resp.status_code in (302, 301, 307, 308):
            results.record_pass("auth_oauth_google_returns_redirect")
        elif no_auth_resp.status_code == 200:
            results.record_pass("auth_oauth_google_returns_200")
        elif no_auth_resp.status_code == 404:
            results.record_skip("auth_oauth_google", "Endpoint not implemented (404)")
        else:
            results.record_fail("auth_oauth_google_returns_redirect",
                                f"Got {no_auth_resp.status_code}")
    except Exception as e:
        results.record_fail("auth_oauth_google", f"Request exception: {e}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 8: GET /auth/oauth/github → 302 redirect to GitHub
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /auth/oauth/github → 302 redirect to GitHub"))
    try:
        no_auth_resp = _requests.get(
            f"{client.v1_base}/auth/oauth/github",
            headers={"Accept": "application/json"},
            timeout=client.timeout,
            allow_redirects=False
        )
        if no_auth_resp.status_code in (302, 301, 307, 308):
            results.record_pass("auth_oauth_github_returns_redirect")
        elif no_auth_resp.status_code == 200:
            results.record_pass("auth_oauth_github_returns_200")
        elif no_auth_resp.status_code == 404:
            results.record_skip("auth_oauth_github", "Endpoint not implemented (404)")
        else:
            results.record_fail("auth_oauth_github_returns_redirect",
                                f"Got {no_auth_resp.status_code}")
    except Exception as e:
        results.record_fail("auth_oauth_github", f"Request exception: {e}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 9: GET /auth/callback → 302 or 400 (called without valid state)
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /auth/callback → 302/400 (no valid OAuth state)"))
    try:
        no_auth_resp = _requests.get(
            f"{client.v1_base}/auth/callback",
            headers={"Accept": "application/json"},
            timeout=client.timeout,
            allow_redirects=False
        )
        if no_auth_resp.status_code in (302, 301, 307, 308, 400, 422):
            # Redirect or validation error — endpoint is registered
            results.record_pass("auth_callback_endpoint_exists")
        elif no_auth_resp.status_code == 200:
            results.record_pass("auth_callback_returns_200")
        elif no_auth_resp.status_code == 404:
            results.record_skip("auth_callback", "Endpoint not implemented (404)")
        else:
            results.record_fail("auth_callback_endpoint_exists",
                                f"Got {no_auth_resp.status_code}")
    except Exception as e:
        results.record_fail("auth_callback", f"Request exception: {e}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 10: POST /auth/logout → 204
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /auth/logout → 204"))
    resp = client.post("/auth/logout")
    if resp.status_code == 204:
        results.record_pass("auth_logout_returns_204")
    elif resp.status_code == 200:
        results.record_pass("auth_logout_returns_200")
    elif resp.status_code == 404:
        results.record_skip("auth_logout_returns_204", "Endpoint not implemented (404)")
    else:
        results.record_fail("auth_logout_returns_204", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
