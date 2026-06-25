# F01 — Auth: Business Logic

> **Services:** `identity-service` (:8081), `apps/osv` gateway (:8080)  
> **CR refs:** CR-007, CR-DD-011, CR-GCV-008, CR-OVS-003

---

## 1. Authentication Chain

### 1.1 Dual Auth (v2.x — Gateway Middleware)

Gateway kiểm tra theo thứ tự ưu tiên:

```
Incoming Request
    │
    ▼
1. Extract "Authorization: Bearer {token}"
   ├── Có JWT? → Validate JWT (HS256)
   │   ├── Valid + not expired → proceed with JWT claims
   │   └── Invalid/expired → fallback to step 2
   │
2. Extract "X-Api-Key: {key}"
   ├── Có API key? → ValidateAPIKey via identity-service
   │   ├── Valid + not revoked + not expired + scope OK → proceed
   │   └── Invalid → 401 Unauthorized
   │
3. Cả hai đều thất bại → 401 {"error": "unauthorized"}
    │
    ▼ (authenticated)
Inject headers → forward to upstream service:
    X-User-ID:    {user_id}
    X-User-Role:  {role}
    X-User-Perms: {scope1,scope2,...}
```

### 1.2 JWT Validation Logic

```
validateJWT(token string):
    1. Parse token header → verify algorithm == HS256 (v2.x) or RS256 (v3.0)
    2. Verify signature với HMAC secret (v2.x) hoặc RSA public key (v3.0)
    3. Check claims:
       - exp: token expired? → 401 TOKEN_EXPIRED
       - iss: issuer match?
       - jti: blacklisted? (Redis key: osv:jwt:revoked:{jti})
    4. Extract: sub=user_id, role, permissions[]
    5. Return (claims, nil)
```

### 1.3 API Key Validation Logic

```
validateAPIKey(apiKey string):
    1. prefix = apiKey[:12]
    2. hash = SHA-256(apiKey) — constant-time
    3. db.QueryRow("SELECT * FROM api_keys WHERE prefix=$1", prefix)
    4. timing-safe compare: stored_hash == hash
    5. Check: revoked == false AND (expires_at IS NULL OR expires_at > NOW())
    6. Check scope: required_scope IN key.scopes
    7. Return (apiKeyRecord, nil) or (nil, error)
```

---

## 2. Login Flow (Local Auth)

### 2.1 Happy Path

```
POST /api/v1/auth/login
{email, password}
    │
    ▼
identity-service:
    1. Find user by email → users table
    2. Check: user.active == true, user.locked_until < NOW()
    3. bcrypt.Compare(password, user.password_hash)
       └── Mismatch:
           a. Increment failed_attempts counter
           b. If failed_attempts >= 5:
              locked_until = NOW() + 15 min
              Return 423 ACCOUNT_LOCKED
           c. Else Return 401 INVALID_CREDENTIALS
    │
    ▼ (password match)
    4. Reset failed_attempts = 0
    5. Generate JWT access token:
       - Claims: {sub: user_id, role, permissions[], iss, exp: now+15min, jti: uuid}
       - Sign with HS256 (v2.x)
    6. Generate refresh token (crypto/rand 32 bytes → base64)
    7. Store: sessions.INSERT {user_id, SHA-256(refresh_token), expires: now+7d}
    8. Return:
       - Body: {access_token, user: {id, email, role, permissions}}
       - Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/refresh
```

### 2.2 LDAP Auth Chain

```
Auth Chain (configurable order in config):
    Local → LDAP (default)

LDAP flow:
    1. Try local auth first
    2. If user not found locally:
       a. LDAP bind: ldap.Bind(bind_dn, bind_password)
       b. LDAP search: (&(objectClass=person)(mail={email}))
       c. Verify password: ldap.Bind(user_dn, password)
       d. Fetch groups: memberOf attribute
       e. Map LDAP groups → OSV role (from ldap_configs.group_mapping)
       f. Upsert user locally (ldap_user=true, no local password)
    3. Issue JWT với mapped role
```

---

## 3. Token Lifecycle

### 3.1 Access Token Refresh

```
POST /api/v1/auth/refresh
Cookie: refresh_token={token}
    │
    ▼
identity-service:
    1. Extract refresh_token from httpOnly cookie
    2. Hash: SHA-256(refresh_token)
    3. Find session: sessions WHERE refresh_token_hash = hash
    4. Check: session.expires_at > NOW()
    5. Reuse detection: session.used == true?
       └── YES → REFRESH_TOKEN_REUSED
           - Revoke ALL sessions for user (security response)
           - Return 401
    6. Mark session.used = true (rotate)
    7. Generate NEW refresh token → INSERT new session
    8. Delete old session
    9. Generate new access token → Return
```

### 3.2 Logout

```
POST /api/v1/auth/logout
Authorization: Bearer {access_token}
    │
    ▼
identity-service:
    1. Add JWT jti to Redis blacklist:
       SET osv:jwt:revoked:{jti} 1 EX {remaining_ttl}
    2. Delete sessions for user (optional: all sessions or current session)
    3. Set-Cookie: refresh_token=""; Max-Age=0 (clear cookie)
    4. Return 204 No Content
```

---

## 4. API Key Management

### 4.1 Create API Key

```
POST /api/v1/api-keys
{name, scopes[], expires_at?}
    │
    ▼
identity-service:
    1. Validate scopes (must be from allowed scope list)
    2. Check limit: user has < 5 API keys
    3. Generate key: "osv_" + base58(crypto/rand 32 bytes)
    4. prefix = key[:12]
    5. hash = SHA-256(key)
    6. INSERT api_keys {user_id, prefix, hash, scopes, expires_at}
    7. Return {id, key: FULL_KEY, scopes, ...}
       └── key (plaintext) chỉ trả về 1 lần — không lưu!
```

### 4.2 Validation (Gateway)

```
X-Api-Key: osv_abc123xyz789...
    │
    ▼
1. prefix = key[:12]                           → "osv_abc123xy"
2. SELECT * FROM api_keys WHERE prefix=$1      → DB lookup
3. SHA-256(incoming_key) == stored.hash_sha256 → constant-time compare
4. stored.revoked == false
5. stored.expires_at IS NULL OR > NOW()
6. required_scope IN stored.scopes             → scope check
    │
    ▼
OK → inject X-User-ID, X-User-Role, X-User-Perms
```

---

## 5. RBAC Permission Enforcement

### 5.1 Middleware Check

```
Upstream service receives request:
    X-User-Role:  "user"
    X-User-Perms: "cve:read,finding:read,finding:write"
    │
    ▼
Handler checks required permission:
    if !hasPermission(r, "finding:write") {
        return 403 Forbidden
    }

func hasPermission(r *http.Request, required string) bool {
    perms := strings.Split(r.Header.Get("X-User-Perms"), ",")
    for _, p := range perms {
        if p == required { return true }
    }
    role := r.Header.Get("X-User-Role")
    return role == "admin"  // admin bypasses all checks
}
```

### 5.2 Scope Matrix

| Scope | Role: admin | Role: user | Role: readonly |
|-------|-------------|-----------|----------------|
| `cve:read` | ✅ | ✅ | ✅ |
| `finding:read` | ✅ | ✅ | ✅ |
| `finding:write` | ✅ | ✅ | ❌ |
| `scan:read` | ✅ | ✅ | ✅ |
| `scan:write` | ✅ | ✅ | ❌ |
| `report:download` | ✅ | ✅ | ✅ |
| `report:generate` | ✅ | ✅ | ❌ |
| `admin:*` | ✅ | ❌ | ❌ |
| `agent:report` | ✅ | ✅ | ❌ |

---

## 6. Account Lockout Logic

```
On each failed login attempt:
    1. failed_count = INCR Redis key: login:fail:{email}  (TTL: 15min)
    2. if failed_count >= 5:
       - UPDATE users SET locked_until = NOW() + INTERVAL '15 minutes'
       - Return 423 ACCOUNT_LOCKED

On next login attempt:
    1. Check: users.locked_until > NOW()
       - YES → Return 423 ACCOUNT_LOCKED (include retry-after timestamp)
       - NO  → Continue auth flow
       
On successful login:
    1. RESET Redis key: DEL login:fail:{email}
    2. UPDATE users SET failed_attempts = 0, locked_until = NULL
```

---

## 7. Error Codes

| HTTP Status | Error Code | Trigger |
|-------------|-----------|---------|
| 401 | `INVALID_CREDENTIALS` | Wrong email or password |
| 401 | `TOKEN_EXPIRED` | JWT exp claim passed |
| 401 | `TOKEN_INVALID` | Signature invalid or tampered |
| 401 | `REFRESH_TOKEN_REUSED` | Replay attack detected on refresh |
| 401 | `MFA_REQUIRED` | Account has MFA enabled (v3.0) |
| 401 | `API_KEY_INVALID` | Key not found or hash mismatch |
| 401 | `API_KEY_REVOKED` | Key explicitly revoked |
| 401 | `API_KEY_EXPIRED` | Key past expires_at |
| 401 | `INSUFFICIENT_SCOPE` | API key lacks required scope |
| 403 | `FORBIDDEN` | Role lacks permission for resource |
| 423 | `ACCOUNT_LOCKED` | > 5 consecutive failures |
| 429 | `RATE_LIMIT_EXCEEDED` | Login rate limit: 5 req/min/IP |

---

## 8. [Planned v3.0] MFA — TOTP Business Logic

```
Setup Flow:
    1. GET /api/v1/auth/mfa/setup
       → Generate TOTP secret (crypto/rand 20 bytes → base32)
       → Store secret in users.mfa_secret_pending (NOT active yet)
       → Return: {qr_code_uri, backup_codes[8]}

    2. POST /api/v1/auth/mfa/confirm {totp_code}
       → Validate TOTP: HMAC-SHA1(secret, floor(now/30))
       → Window: ±1 period tolerance (3 codes valid: t-1, t, t+1)
       → If valid: SET users.mfa_enabled = true, mfa_secret = pending_secret
       → Hash and store backup codes (bcrypt)

Login with MFA:
    1. POST /login {email, password}
       ← 200 {mfa_required: true, mfa_token: temp_jwt_15min}
    
    2. POST /login/mfa {mfa_token, totp_code}
       → Validate TOTP or backup code
       → Issue full access_token + refresh_token
```

---

## 9. [Planned v3.0] OAuth2 Business Logic

```
Google/GitHub OAuth2 PKCE Flow:
    1. GET /auth/google
       → Generate code_verifier (crypto/rand 32 bytes)
       → code_challenge = BASE64URL(SHA-256(code_verifier))
       → Store: session[state] = {code_verifier, redirect_uri}
       → Redirect to Google with: client_id, scope, state, code_challenge

    2. Google redirects back → GET /auth/callback?code=xxx&state=yyy
       → Validate state matches session
       → Exchange code for Google access_token (with code_verifier)
       → Fetch Google userinfo: {email, name, picture}
       → Upsert user: INSERT OR UPDATE users SET oauth_provider="google"
       → Issue OSV JWT + refresh token
       → Redirect to frontend with access_token

Argon2id Password (v3.0):
    Params: Memory=65536 (64MB), Iterations=3, Parallelism=4
    Output: 32 bytes → base64 stored
```
