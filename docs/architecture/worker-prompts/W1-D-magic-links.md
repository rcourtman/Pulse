# Worker Prompt: W1-D — Fix Hosted Signup Auth Flow (Magic Links)

## Task ID: W1-D (P0-4)

## Goal

Replace the broken password-based hosted signup with passwordless magic link authentication. Currently, `HandlePublicSignup()` collects email + password, validates the password (min 8 chars), but never stores it. The user completes signup but cannot log in.

## Current State

`internal/api/hosted_signup_handlers.go` lines 57-162:
- `hostedSignupRequest` struct has `Email`, `Password`, `OrgName` fields
- Password is validated for length >= 8 (line 88) but never hashed or persisted
- User is created in RBAC with email as userID (line 149) but no credential
- After signup, the user has no way to authenticate

## Design

**Magic link flow:**
1. **Signup**: User enters email + org name → org is created → magic link sent to email → user clicks link → session created → redirect to dashboard
2. **Login**: User enters email → magic link sent → click → session → dashboard
3. **Token**: Short-lived (15 min), single-use, signed with HMAC
4. **Rate limit**: Max 3 magic link requests per email per hour

## Scope (Exact Files)

### 1. `internal/api/hosted_signup_handlers.go`

**Modify `hostedSignupRequest`:**
- Remove `Password` field
- Keep `Email` and `OrgName`

**Modify `HandlePublicSignup()`:**
- Remove password validation (line 88)
- After org creation, generate a magic link token instead of trying to create a session
- Return a response indicating "Check your email" (don't return a session token)

### 2. `internal/api/magic_link.go` (NEW FILE)

Create the magic link subsystem:

```go
type MagicLinkService struct {
    hmacKey    []byte
    store      MagicLinkStore    // persists tokens (can be in-memory with expiry)
    emailer    MagicLinkEmailer  // sends emails (interface for testability)
    rateLimiter *rate.Limiter    // per-email rate limiting
}

type MagicLinkToken struct {
    Email     string
    OrgID     string
    ExpiresAt time.Time
    Used      bool
    Token     string  // HMAC-signed opaque token
}
```

**Key methods:**
- `GenerateToken(email, orgID string) (string, error)` — creates signed token, stores it, returns URL-safe string
- `ValidateToken(token string) (*MagicLinkToken, error)` — validates signature, checks expiry, marks as used
- `SendMagicLink(email, orgID, token string) error` — sends email with link (use interface so tests can mock)

**Token format:**
- Payload: `email|orgID|expiresAt|nonce`
- Signed: HMAC-SHA256 with server key
- Encoded: base64url for URL safety
- Single-use: marked `used=true` after validation

### 3. `internal/api/magic_link_handlers.go` (NEW FILE)

**Endpoints:**

`POST /api/public/magic-link/request` — Request a magic link (for login)
- Input: `{ "email": "user@example.com" }`
- Validates email format
- Checks rate limit (3 per email per hour)
- Generates token and "sends" email
- Always returns 200 (don't leak whether email exists — security)

`GET /api/public/magic-link/verify?token=...` — Verify magic link
- Validates token (signature, expiry, not-used)
- Creates session for the user
- Redirects to dashboard (or returns session token for API use)

### 4. `internal/api/router.go` or `internal/api/router_routes_hosted.go`

Wire the new endpoints:
- `POST /api/public/magic-link/request` — no auth required
- `GET /api/public/magic-link/verify` — no auth required
- Both gated behind `hostedMode` check

### 5. Magic Link Store

For now, use an in-memory store with TTL eviction (tokens expire in 15 min anyway). Structure:

```go
type InMemoryMagicLinkStore struct {
    mu     sync.RWMutex
    tokens map[string]*MagicLinkToken
}
```

Add a cleanup goroutine that evicts expired tokens every 5 minutes.

### 6. Email Interface

Create an interface for email sending — the actual implementation can be a stub/log-only for now (we'll wire real email in P2):

```go
type MagicLinkEmailer interface {
    SendMagicLink(to, magicLinkURL string) error
}

// LogEmailer logs the magic link URL instead of sending email (dev/staging)
type LogEmailer struct{}
```

### 7. Tests

**`internal/api/magic_link_test.go`** (NEW):
- Test token generation + validation roundtrip
- Test expired token is rejected
- Test used token is rejected (single-use)
- Test invalid signature is rejected
- Test rate limiting (4th request in an hour is rejected)

**`internal/api/hosted_signup_handlers_test.go`** (modify existing):
- Update test to not send password
- Verify signup returns "check email" response, not a session

## Constraints

- Do NOT implement real email sending — use `LogEmailer` that logs the magic link URL. Real email is a P2 concern.
- Do NOT change the RBAC system — users are still identified by email
- Do NOT change session management — once the magic link is verified, create a session using the existing session infrastructure
- The HMAC key should come from the existing encryption key infrastructure (check `tmp/dev-config/.encryption.key` pattern) or generate a dedicated one
- Magic link URLs should be: `https://{host}/api/public/magic-link/verify?token={token}`
- All magic link endpoints must be behind `hostedMode` guard

## Acceptance Checks

```bash
# Must pass
go build ./internal/api/...
go test ./internal/api/... -count=1 -v -run "TestMagicLink"
go test ./internal/api/... -count=1 -v -run "TestHostedSignup|TestPublicSignup"

# Verify password field is gone from signup
grep -n "Password" internal/api/hosted_signup_handlers.go
# Expected: no matches in the request struct
```

## Expected Return

```
status: done | blocked
files_changed: [list with brief why for each]
commands_run: [command + exit code for each]
summary: [what was done]
blockers: [if any]
```
