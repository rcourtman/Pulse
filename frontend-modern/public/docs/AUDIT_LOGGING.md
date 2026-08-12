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
| `agent_config_fetch` | Agent configuration retrieval attempts | Agent config fetched successfully |

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

```bash
# List recent events
curl http://localhost:7655/api/audit?limit=50 \
  -H "Authorization: Bearer $TOKEN"

# Filter by event type and date range
curl "http://localhost:7655/api/audit?event=login&startTime=2026-01-01T00:00:00Z&endTime=2026-01-31T23:59:59Z&success=false" \
  -H "Authorization: Bearer $TOKEN"

# Get audit summary
curl http://localhost:7655/api/audit/summary \
  -H "Authorization: Bearer $TOKEN"
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
curl http://localhost:7655/api/audit/export \
  -H "Authorization: Bearer $TOKEN" \
  -o audit-export.json
```

The export includes all events matching the current filter criteria.

---

## Tamper Detection

Current audit events are cryptographically signed at creation time. Verification reports both whether the retained bytes match and what integrity claim that signature format can support:

```bash
curl http://localhost:7655/api/audit/6b3c9c3c-9a2f-4b3c-9a3b-3d0e8c5c5d45/verify \
  -H "Authorization: Bearer $TOKEN"
```

Response:
```json
{
  "available": true,
  "verified": true,
  "status": "strong",
  "version": "v3",
  "assurance": "strong",
  "message": "Event signature strongly verified"
}
```

`status` and `assurance` are authoritative. `verified` is retained for older clients and is `true` for both `strong` and `compatibility`; it must not be used to present those states as equivalent.

| Status | Meaning |
|--------|---------|
| `strong` | A valid current `v3` signature with injective field encoding and a version-specific HMAC key domain. |
| `compatibility` | A valid historical `v2` or unprefixed legacy signature. The bytes match an accepted historical format, but that format does not support the current integrity claim. |
| `invalid` | A known signature format did not verify. Treat the evidence as failed integrity validation. |
| `unknown` | The envelope is malformed or unsupported, or verification key material is unavailable. Do not retry it under an older format. |
| `unsigned` | No signature was retained. |

The list API returns `signature_version`, `signature_status`, and `signature_assurance` on every row. JSON and CSV exports include the version and, with `verify=true`, the status and assurance. Summaries separately count strong, compatibility, invalid, unknown, and unsigned evidence. The UI reserves the green **Strongly verified** state for `strong`; compatibility history is amber and explicitly lower assurance.

### Signature versions and compatibility

- `v3:<64 hex digits>` is the current format. Its authenticated message contains the `v3` domain and envelope, an injective length-prefixed representation of every signed string, the exact persisted Unix-second timestamp and success byte, and the retained historical timestamp-evidence field. Its HMAC-SHA256 key is derived from the instance master audit key under the dedicated `pulse.audit.hmac-key\0v3\0` domain. Removing or changing the envelope cannot move the digest into the accepted `v2` or legacy key domain.
- `v2:<64 hex digits>` uses injective fields but shares the historical master-key domain, so it is compatibility evidence only.
- An unprefixed 64-hex signature may match one of three historical delimiter-separated encodings. Those encodings have ambiguous string boundaries, so a match is compatibility evidence only.
- Any other envelope or malformed MAC is `unknown`. Verification dispatch never falls back from a failed prefixed version into an older representation.

Signatures are opaque versioned strings, not fixed-length bare hex fields. The repository has no supported audit consumer that should assume 64 or 67 characters. Webhook/SIEM clients should retain the signature and the explicit version/assurance fields unchanged and use Pulse's verification endpoint for the authoritative result.

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

At startup, Pulse transactionally migrates older audit tables to a strict canonical schema. All signed strings are non-null SQLite `TEXT`, timestamps and success flags are canonical `INTEGER` values, and success is restricted to `0` or `1`. Reads validate raw SQLite storage classes and fail closed on alternate integers, nulls, blobs, reals, or noncanonical timestamp evidence instead of verifying a lossy Go projection.

Historical textual timestamps are normalized for ordering while their canonical RFC3339Nano form is retained separately as signature evidence. This preserves genuine fractional legacy timestamps through migration and restart. Pulse never replaces or re-signs a historical signature. A row that was migrated by an older release after its fractional timestamp material had already been discarded remains `invalid` or otherwise lower assurance; the new migration does not invent evidence or upgrade it. A malformed mixed dataset rolls back as one transaction, leaving the original table and rows intact.

---

## Related Documentation

- [Plans and Entitlements](PULSE_PRO.md) — Audit logging availability by plan
- [RBAC](RBAC.md) — Role-based access control (role changes are audit logged)
- [OIDC / SSO](OIDC.md) — SSO login events are audit logged
- [Security Policy](../SECURITY.md) — Core security model
- [Multi-Tenant Organizations](MULTI_TENANT.md) — Per-tenant audit isolation
- [API Reference](API.md#-audit-log-pro) — Audit log API endpoints
