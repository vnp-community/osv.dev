# F01 — Auth: Data Flow

> **Services:** `identity-service` (:8081), `apps/osv` gateway (:8080)

---

## 1. Login Flow — Sequence Diagram

```
Client          Gateway(:8080)      identity-service(:8081)   PostgreSQL    Redis
  │                  │                       │                     │           │
  │ POST /login      │                       │                     │           │
  │─────────────────►│                       │                     │           │
  │                  │  No auth required     │                     │           │
  │                  │  (login is public)    │                     │           │
  │                  │──────────────────────►│                     │           │
  │                  │                       │ SELECT * FROM users  │           │
  │                  │                       │  WHERE email=$1     │           │
  │                  │                       │────────────────────►│           │
  │                  │                       │◄────────────────────│           │
  │                  │                       │ bcrypt.Compare()    │           │
  │                  │                       │                     │           │
  │                  │                       │ [FAIL] increment    │           │
  │                  │                       │  login:fail:{email} │           │ INCR login:fail:email
  │                  │                       │────────────────────────────────►│
  │                  │                       │                     │           │
  │                  │                       │ [SUCCESS]           │           │
  │                  │                       │ Generate JWT        │           │
  │                  │                       │ Generate refresh_token          │
  │                  │                       │ INSERT sessions     │           │
  │                  │                       │────────────────────►│           │
  │                  │                       │◄────────────────────│           │
  │◄─────────────────────────────────────────│                     │           │
  │ 200 {access_token, user}                 │                     │           │
  │ Set-Cookie: refresh_token (HttpOnly)     │                     │           │
```

---

## 2. API Request Auth Flow — Sequence Diagram

```
Client          Gateway(:8080)           identity-service        Upstream Service
  │                  │                         │                      │
  │ GET /api/v2/...  │                         │                      │
  │ Authorization:   │                         │                      │
  │   Bearer {jwt}   │                         │                      │
  │─────────────────►│                         │                      │
  │                  │ authMiddleware           │                      │
  │                  │ ┌─────────────────────┐ │                      │
  │                  │ │ 1. extractBearer()  │ │                      │
  │                  │ │ 2. validateJWT()    │ │                      │
  │                  │ │    - verify sig     │ │                      │
  │                  │ │    - check exp      │ │                      │
  │                  │ │    - check Redis    │ │                      │
  │                  │ │      blacklist      │ │                      │
  │                  │ └─────────────────────┘ │                      │
  │                  │ [JWT valid]              │                      │
  │                  │ inject headers:          │                      │
  │                  │   X-User-ID: {id}        │                      │
  │                  │   X-User-Role: {role}    │                      │
  │                  │   X-User-Perms: {scopes} │                      │
  │                  │──────────────────────────────────────────────►│
  │                  │                                                 │ handler checks
  │                  │                                                 │ X-User-Perms
  │◄────────────────────────────────────────────────────────────────────
  │ 200 Response     │                         │                      │
```

---

## 3. API Key Validation Flow

```
Client                   Gateway(:8080)              identity-service    PostgreSQL
  │                           │                            │                 │
  │ GET /api/v2/cves/...      │                            │                 │
  │ X-Api-Key: osv_abc...     │                            │                 │
  │──────────────────────────►│                            │                 │
  │                           │ authMiddleware              │                 │
  │                           │ JWT not found/invalid      │                 │
  │                           │ Try API Key path:          │                 │
  │                           │                            │                 │
  │                           │ identityClient.            │                 │
  │                           │   ValidateAPIKey()         │                 │
  │                           │───────────────────────────►│                 │
  │                           │                            │ SELECT WHERE    │
  │                           │                            │  prefix=$1      │
  │                           │                            │────────────────►│
  │                           │                            │◄────────────────│
  │                           │                            │ SHA256 compare  │
  │                           │                            │ check revoked   │
  │                           │                            │ check expiry    │
  │                           │                            │ check scope     │
  │                           │◄───────────────────────────│                 │
  │                           │ [Valid] inject headers     │                 │
  │                           │──────────────────────────────────────────────►
  │◄──────────────────────────│                            │                 │
  │ 200 Response              │                            │                 │
```

---

## 4. Token Refresh Flow

```
Client          Gateway(:8080)      identity-service    PostgreSQL    Redis
  │                  │                    │                  │           │
  │ POST /auth/refresh│                   │                  │           │
  │ Cookie: refresh_token=...             │                  │           │
  │─────────────────►│                   │                  │           │
  │                  │ (public route)    │                  │           │
  │                  │──────────────────►│                  │           │
  │                  │                   │ extract cookie   │           │
  │                  │                   │ hash token       │           │
  │                  │                   │ SELECT session   │           │
  │                  │                   │  WHERE hash=$1   │           │
  │                  │                   │─────────────────►│           │
  │                  │                   │◄─────────────────│           │
  │                  │                   │                  │           │
  │                  │                   │ [Reuse detected] │           │
  │                  │                   │  DELETE ALL sessions for user│
  │                  │                   │─────────────────►│           │
  │                  │◄──────────────────│ 401 REFRESH_TOKEN_REUSED     │
  │◄─────────────────│                   │                  │           │
  │                  │                   │ [Valid]          │           │
  │                  │                   │ Mark old used=true           │
  │                  │                   │ INSERT new session│           │
  │                  │                   │ Issue new access_token       │
  │                  │◄──────────────────│ 200 {access_token}          │
  │◄─────────────────│ Set-Cookie: new refresh_token                    │
```

---

## 5. Logout Flow

```
Client          Gateway(:8080)        identity-service         Redis
  │                  │                      │                    │
  │ POST /auth/logout│                      │                    │
  │ Bearer: {jwt}    │                      │                    │
  │─────────────────►│                      │                    │
  │                  │ validate JWT         │                    │
  │                  │──────────────────────►                    │
  │                  │                      │ SETEX              │
  │                  │                      │  osv:jwt:revoked:{jti}     │
  │                  │                      │  TTL=remaining_exp │
  │                  │                      │───────────────────►│
  │                  │                      │◄───────────────────│
  │                  │                      │ DELETE session     │
  │                  │◄─────────────────────│                    │
  │◄─────────────────│ 204 No Content       │                    │
  │ Set-Cookie: refresh_token=""; Max-Age=0 │                    │
```

---

## 6. [Planned v3.0] OAuth2 Flow

```
Client          Gateway(:8080)      identity-service      Google OAuth
  │                  │                    │                    │
  │ GET /auth/google │                    │                    │
  │─────────────────►│                    │                    │
  │                  │──────────────────►│                    │
  │                  │                   │ generate state + PKCE verifier
  │                  │                   │ store in session   │
  │◄─────────────────│ 302 Redirect to Google OAuth URL      │
  │──────────────────────────────────────────────────────────►│
  │ [User authenticates on Google]        │                    │
  │◄──────────────────────────────────────────────────────────│
  │ GET /auth/callback?code=xxx&state=yyy │                    │
  │─────────────────►│                    │                    │
  │                  │──────────────────►│                    │
  │                  │                   │ validate state     │
  │                  │                   │ exchange code for token ──►│
  │                  │                   │◄──────────────────────────│
  │                  │                   │ fetch Google userinfo      │
  │                  │                   │ upsert user in DB │        │
  │                  │                   │ issue OSV JWT     │        │
  │◄─────────────────────────────────────│ 302 → /dashboard  │        │
  │ Set-Cookie: refresh_token            │                    │        │
```

---

## 7. Gateway Rate Limit Flow

```
Client                    Gateway (Redis)
  │                            │
  │ POST /auth/login            │
  │────────────────────────────►│
  │                             │ key = "ratelimit:{ip}:login"
  │                             │ ZREMRANGEBYSCORE (remove old entries)
  │                             │ ZCARD (count current window)
  │                             │ count >= 5?
  │                             │  YES → 429 RATE_LIMIT_EXCEEDED
  │◄────────────────────────────│      Retry-After: 60
  │                             │  NO  → ZADD(now), forward request
```

---

## 8. NATS Events

Auth không publish NATS events trực tiếp. Tuy nhiên, các events liên quan:

| Event | Publisher | Trigger |
|-------|-----------|---------|
| `audit.user.login` | audit-service | Subscribe từ identity-service event |
| `audit.user.locked` | audit-service | Account lockout triggered |
| `audit.apikey.created` | audit-service | New API key created |
| `audit.apikey.revoked` | audit-service | API key revoked |
