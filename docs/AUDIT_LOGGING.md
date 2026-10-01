# Audit Logging

Pulse's audit log records security-relevant events with tamper-evident signatures. Use it for compliance, incident investigation, and tracking who did what.

**Requires:** Pro, legacy Pro+, or Cloud license with the `audit_logging` capability to query, export, and verify events via the API. Events are recorded on all plans, but the API endpoints are license-gated.

For plan details, see [PULSE_PRO.md](PULSE_PRO.md). For API endpoints, see [API Reference](API.md#-audit-log-pro).

---

## What Gets Logged

Pulse automatically captures the following events:

| Event Type | Description | Example |
|------------|-------------|---------|
| `login` | Successful and failed login attempts | User `admin` logged in from 198.51.100.5 |
| `logout` | User logouts | User `admin` logged out |
| `password_change` | Password modifications | Password changed (Docker/systemd) |
| `csrf_failure` | Blocked cross-site request forgery attempts | Invalid CSRF token |
| `lockout_reset` | Account lockout resets | Admin reset lockout for user `bob` |
| `oidc_login` | OIDC SSO login attempts (success/failure at each stage) | OIDC login success |
| `oidc_token_refresh` | OIDC token refresh success/failure (global, not tenant-scoped) | Token refreshed successfully |
| `oidc_role_assignment` | Automatic role assignment from OIDC groups | Auto-assigned roles: operator, viewer |
| `saml_login` | SAML SSO login attempts | SAML login success via provider-id |
| `saml_role_assignment` | Automatic role assignment from SAML groups | Auto-assigned roles: admin |
| `sso_provider_created` | SSO provider configuration created | Created provider: Authentik |
| `sso_provider_updated` | SSO provider configuration modified | Updated provider: Authentik |
| `sso_provider_deleted` | SSO provider configuration removed | Deleted provider: Authentik |
| `ai_settings_updated` | AI configuration changes | AI settings updated |
| `agent_profile_assigned` | Agent profile assignments | Profile `production` assigned to agent |
| `agent_profile_unassigned` | Agent profile removals | Profile removed from agent |
| `user_roles_updated` | RBAC role assignments changed | Updated roles for user jane: [operator] |
| `agent_config_fetch` | Every failed agent configuration fetch. A successful fetch is recorded on the agent's first delivery after Pulse starts, when its token or delivered configuration changes, and otherwise once a day; agents poll every minute, so repeat polls are not recorded | `agent_id=… token_id=… config=sha256:… reason=config_changed` |

Each event includes:
- **Timestamp** (UTC)
- **Event type**
- **User** who triggered the event
- **Client IP** address
- **Request path**
- **Success/failure** flag
- **Details** (human-readable description)
- **Cryptographic signature** (tamper detection)

---

## Viewing Audit Events

### UI

**Settings → Security → Audit Log**

The audit log panel shows events in reverse chronological order with filtering by event type, user, date range, and success/failure.

### API

Use a token with `audit:read`, bound to a user permitted to read audit logs,
and the licensed `audit_logging` capability. Prepare the private header file
in [API authentication](API.md#-authentication); never paste the token or a
session cookie into a command. For a one-off read, you can instead open the
API path in your signed-in Pulse browser.

The loopback URLs below apply on the Pulse host. For remote access, use your
Pulse HTTPS URL with certificate verification enabled. Use curl 7.76 or later,
keeping `--disable` first to ignore local trace/verbose defaults.
`--fail-with-body` returns a non-zero exit on HTTP failures, including 401,
402 and 403. Share only the relevant redacted error, not the header file or
whole audit response: events can contain usernames, client addresses and paths.

```bash
# List recent events
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  'http://127.0.0.1:7655/api/audit?limit=50'

# Filter by event type and date range
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  'http://127.0.0.1:7655/api/audit?event=login&startTime=2026-01-01T00:00:00Z&endTime=2026-01-31T23:59:59Z&success=false'

# Get audit summary
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  http://127.0.0.1:7655/api/audit/summary
```

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Maximum events to return (default: 100) |
| `event` | string | Filter by event type (e.g., `login`, `password_change`) |
| `user` | string | Filter by username |
| `success` | boolean | Filter by success (`true`) or failure (`false`) |
| `startTime` | ISO 8601 | Start of date range |
| `endTime` | ISO 8601 | End of date range |

---

## Exporting Audit Data

Export the audit log for external analysis or compliance archival:

```bash
umask 077
export_dir="$(mktemp -d)" &&
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  --output "$export_dir/audit-export.json" \
  http://127.0.0.1:7655/api/audit/export &&
printf 'Saved private export to %s\n' "$export_dir/audit-export.json"
```

This creates a new private directory and prints the file's location only on
success. It does not reuse filters selected in the UI. Treat the export as
sensitive data; keep it outside shared repositories and issue attachments.

---

## Tamper Detection

Every audit event is cryptographically signed at creation time. You can verify that an event has not been modified:

```bash
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  http://127.0.0.1:7655/api/audit/6b3c9c3c-9a2f-4b3c-9a3b-3d0e8c5c5d45/verify
```

Response:
```json
{
  "available": true,
  "verified": true,
  "message": "Event signature verified"
}
```

If `verified` is `false`, the event data has been tampered with since it was recorded.

---

## Multi-Tenant Audit Isolation

In multi-tenant deployments, most events are scoped to the active organization:

- Tenant-aware events (logins, role changes, config updates) are stored per-organization.
- Some auth lifecycle events (e.g., `oidc_token_refresh`) are global and not tenant-scoped.
- Switching organizations shows only that organization's tenant-scoped events.
- The tenant context is determined by `X-Pulse-Org-ID` header or session cookie.

See [Multi-Tenant Organizations](MULTI_TENANT.md) for details.

---

## Community vs Pro Behavior

| Capability | Community | Pro / legacy Pro+ / Cloud |
|------------|-----------|-------------|
| Events captured | Yes | Yes |
| Persistent storage (SQLite) | Yes | Yes |
| Query/filter API | License-gated (402) | Full access |
| Signature verification | License-gated (402) | Available |
| Export | License-gated (402) | Available |
| `persistentLogging` API flag | `false` | `true` |

On all plans, audit events are written to the SQLite database. However, the query, verify, and export API endpoints require the `audit_logging` license feature and return `402 Payment Required` without it. The `persistentLogging` flag in API responses indicates whether the licensed query capabilities are available.

---

## Storage

Audit events are stored in a SQLite database in the Pulse data directory:
- **Single-tenant:** `{data-dir}/audit/audit.db`
- **Multi-tenant:** `{data-dir}/orgs/{org-id}/audit/audit.db`

Data directory locations:
- systemd: `/etc/pulse/`
- Docker/Kubernetes: `/data/`
- Development: `tmp/dev-config/`

---

## Related Documentation

- [Plans and Entitlements](PULSE_PRO.md) — Audit logging availability by plan
- [RBAC](RBAC.md) — Role-based access control (role changes are audit logged)
- [OIDC / SSO](OIDC.md) — SSO login events are audit logged
- [Security Policy](../SECURITY.md) — Core security model
- [Multi-Tenant Organizations](MULTI_TENANT.md) — Per-tenant audit isolation
- [API Reference](API.md#-audit-log-pro) — Audit log API endpoints
