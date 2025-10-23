# API Token Scope Design Brief

## Objective
Introduce scoped API tokens so administrators can grant the minimum necessary permissions to each integration (Docker agent, host agent, future platform agents, automation scripts, etc.). This replaces today’s “all-or-nothing” tokens and provides safer rotation/revocation paths.

## Security Rationale
- Agent and automation tokens are often deployed on hosts or third-party services we do not fully trust. If one leaks today, the attacker inherits full administrator powers (issuing new tokens, mutating settings, triggering installs/updates, etc.).
- Constraining tokens to the minimal scope limits the blast radius: a compromised reporting agent can only submit telemetry, not reconfigure Pulse or other integrations.
- Customers operating in regulated or multi-team environments increasingly ask for auditable least-privilege controls. Scopes give us the primitives to surface warnings on over-privileged tokens and eventually add rotation workflows.
- The feature still has to earn adoption, so pair the technical work with UI nudges and reporting that highlight “Full access” tokens and encourage admins to narrow permissions.

## Requirements Overview

1. **Token Model Changes**
   - Each API token record stores a list of scopes (strings) or a bitmask. Recommended canonical strings:
     - `monitoring:read`
     - `monitoring:write`
     - `docker:report`
     - `docker:manage`
     - `host-agent:report`
     - `settings:read`
     - `settings:write`
     - `*` (legacy full-access sentinel; backend accepts for migration/edit flows but the UI should not expose it)
   - Existing tokens must remain valid. Treat missing scopes as `["*"]` (full access) until the admin edits them.

2. **Persistence & Migration**
   - Extend the token persistence layer (currently BoltDB JSON) to include `scopes: []string`.
   - On startup, detect tokens without the new field and default to `["*"]`.
   - Expose the complete scope list when returning token metadata (internal API used by Settings UI).

3. **Middleware Enforcement**
   - Add a helper `RequireScope(scope string)` that checks the request’s token record for `scope` or `*`.
   - Apply the helper according to the table below:

     | Endpoint (or group)                             | HTTP verbs                          | Required scope(s)           | Notes                                                |
     |-------------------------------------------------|-------------------------------------|-----------------------------|------------------------------------------------------|
     | `/api/agents/docker/report`                     | `POST`                              | `docker:report`             | Docker agent heartbeat payloads                      |
     | `/api/agents/docker/commands/*`                 | `POST`                              | `docker:manage` (optional)  | If we expose command ack/management over tokens      |
     | `/api/agents/docker/hosts/*`                    | `DELETE`, `PUT`, `POST`             | `docker:manage`             | Admin actions for Docker hosts                       |
     | `/api/agents/host/report`                       | `POST`                              | `host-agent:report`         | Host agent reporting                                 |
     | `/api/state`                                    | `GET`                               | `monitoring:read`           | General state polling (if token-authenticated)       |
     | `/api/alerts/*`                                 | `GET`                               | `monitoring:read`           | Alerts reading APIs                                  |
     | `/api/alerts/*` (mutations)                     | `POST`, `PUT`, `DELETE`             | `monitoring:write`          | Acknowledge, silence, etc.                           |
     | `/api/settings/*`                               | `GET`                               | `settings:read`             | Settings reads via API                               |
     | `/api/settings/*`                               | `POST`, `PUT`, `DELETE`, `PATCH`    | `settings:write`            | Any settings mutation                                |
     | `/api/security/tokens*`                         | all verbs                           | _n/a (session only)_        | Leave browser-session only; do not allow API tokens yet |
     | `/api/install/*`, `/api/updates/*` (mutations)  | `POST`, `PUT`                       | `settings:write`            | Sensitive operational endpoints                      |

     (More endpoints can be added as required; start with the rows above and expand during implementation.)
   - Maintain compatibility for admin sessions (browser login) which continue to bypass token checks.

4. **Token Generation API**
   - Update `POST /api/security/tokens` to accept a `scopes` array.
   - Validation rules:
     - Reject unknown scope identifiers (except the `*` sentinel described above).
     - If the array is omitted (legacy callers), default to `['*']` (full access) to preserve backward compatibility.
     - If the array is provided but empty, reject with a 400 (“select at least one scope or delete the token”).
     - Reject mixed arrays that contain both `'*'` and explicit scopes; if the UI submits such a payload, return a 400 with guidance (“either all scopes or full access”).
   - Include the scope list in the response payload so the UI can display it.

5. **UI/UX Adjustments**
   - **Settings → Security** panel:
     - When generating or editing a token, show a multi-select with friendly labels (“Docker agent reporting”, “Host agent reporting”, etc.).
     - Display the scope summary in the token list (e.g. badges).
     - For legacy tokens (implicit `*`), show “Full access” and allow editing to reduce scope.
   - **Docker Agents / Host Agents screens:**
     - When requesting a token:
       - Docker: pre-select both `docker:report` (always) and `docker:manage` if the user needs lifecycle commands (hide manage behind a toggle if desired).
       - Host agent: pre-select `host-agent:report`.
     - Warn if the stored token lacks the required scope (fallback to showing `<api-token>` placeholder).

6. **Testing**
   - Unit tests covering:
     - Scope parsing/migration
     - Middleware checks (token with/without required scope)
   - Integration tests for agent endpoints verifying 403 on missing scope.

7. **Documentation**
   - Update `README.md` and relevant docs (e.g. `docs/CONFIGURATION.md`, `docs/HOST_AGENT.md`, Docker docs) to explain scoped tokens.
   - Provide an upgrade note for existing installations (“legacy tokens default to full access; edit to restrict scope”).

## Implementation Notes

- Use constants for scope strings to avoid typos throughout the codebase.
- Token middleware already retrieves `APITokenRecord`; that struct should grow a `Scopes []string` field with helper methods (`HasScope`).
- For future extensibility, keep the scope checks granular but simple (string equality) rather than regex matching.
- Ensure the Settings UI gracefully handles lack of admin privileges (disable scope selection, show hint).
- Update agent commands (Docker/Host) to mention required scope in their description.
- Guardrails: the backend should never auto-insert `*` once a scoped token exists, and any admin edit that clears all scopes should surface a clear “delete token or assign scopes” decision.

## Acceptance Criteria

- Scoped tokens persisted and surfaced via API.
- Middleware rejects tokens missing required scope.
- UI can create, edit, and display scoped tokens; agent panels auto-fill only when valid.
- Documentation updated; existing tokens remain functional without manual migration.

Once implemented delete this doc.
