# Multi-Tenant Organizations (Enterprise/Internal)

Pulse supports shared-process organizations for Enterprise and internal multi-organization deployments. Each organization gets its own infrastructure, resources, alerts, and audit log namespace on the same Pulse process.

This is not the canonical Pulse MSP model for separate customer businesses. MSP crosses legal and security ownership boundaries, so the canonical MSP route is provider-hosted: a Stripe-free provider control plane runs one isolated Pulse runtime per client workspace. Use shared-process organizations when one owner is deliberately separating internal sites, teams, departments, or environments.

## Requirements

| Requirement | Detail |
|---|---|
| **Feature flag** | `PULSE_MULTI_TENANT_ENABLED=true` |
| **License** | Enterprise license with `multi_tenant` capability |

Without these, all API calls return `501 Not Implemented` (flag off) or `402 Payment Required` (no license). The **default** organization always works regardless.

The Community, Relay, and Pro tiers do not include the `multi_tenant` capability, and Enterprise licensing is not sold self-serve on the pricing page. Email [support@pulserelay.pro](mailto:support@pulserelay.pro) to arrange an Enterprise license.

## Quick Start

1. Set `PULSE_MULTI_TENANT_ENABLED=true` in your environment and restart Pulse.
2. Activate your Enterprise license in **Settings → Plans & Billing**.
3. Go to **Settings → Organization** and click **Create Organization**.
4. Name your organization and assign infrastructure to it.
5. Use the **Org Switcher** in the header bar to switch between organizations.

## Concepts

### Organizations

An organization is a separate monitoring namespace inside the same Pulse runtime:

- Its own set of monitored nodes and resources.
- Its own alerts, thresholds, and notifications.
- Its own audit log.
- Its own configuration directory on disk.

The **default** organization always exists and is used when multi-tenant is disabled. It cannot be deleted or renamed.

### Roles

Each member has a role within an organization:

| Role | Permissions |
|---|---|
| **Owner** | Full control. Can transfer ownership, delete the org. |
| **Admin** | Manage members, shares, and org settings. Cannot transfer ownership. |
| **Editor** | Read/write access to org resources. Cannot manage members or shares. |
| **Viewer** | Read-only access to all org data. |

### Resource Sharing

Organizations can share specific resources with other organizations:

- Share a VM, container, machine agent, storage, PBS, or PMG resource with another org.
- Assign an access role (`viewer`, `editor`, or `admin`) to the share.
- The receiving org must accept the share before it sees the resource alongside its own.

## Before Using the API

Create organizations and manage members and shares in the signed-in Pulse UI.
These changes require **session-based user authentication** and the relevant
organization role; API tokens are rejected with `403 session_required`, even
when they have `settings:write`. The UI handles the session and CSRF protection.
Do not extract a session cookie or CSRF token into a shell command.

The read-only curl examples use an org-bound token with `settings:read` and
the private header file from [API authentication](API.md#-authentication).
Keep the token out of command lines, URLs and reports. A token's organization
binding is an access boundary: changing a URL or `X-Pulse-Org-ID` header does
not grant it access to another organization.

Use curl 7.76 or later. Keep `--disable` first to ignore local trace/verbose
defaults; `--fail-with-body` makes HTTP failures return a non-zero exit. The
loopback URLs apply on the Pulse host; remotely, use your Pulse HTTPS URL and
keep certificate verification enabled. Replace the example organization ID
`production-datacenter` with your own. Run requests separately, and share only
the relevant redacted error, not whole member or infrastructure responses.

## Managing Organizations

### Creating an Organization

**UI:** Settings → Organization → Create Organization

**API contract:** `POST /api/orgs`, session authentication only:
```json
{"id": "production-datacenter", "displayName": "Production Datacenter"}
```

The creator becomes the owner. `id` is a lowercase alphanumeric/hyphen ID
(3–64 characters); `displayName` is the name shown in Pulse. `name` and
`description` are not the creation fields.

### Switching Organizations

Use the **Org Switcher** dropdown in the header. When you switch:

- All pages reload with the new organization's data.
- AI chat history is reset (each org has its own context).
- Caches are invalidated and re-fetched.

### Managing Members

**UI:** Settings → Organization → Access

**Read-only API:**
```bash
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  http://127.0.0.1:7655/api/orgs/production-datacenter/members
```

**Invite or update a member:** use the Access panel as an owner or admin.
Its API contract is `POST /api/orgs/{id}/members`, session authentication only:
```json
{"userId": "user-id", "role": "editor"}
```

For a new member, the response is `202` with a pending invitation; the user
must accept it in Pulse before gaining access. An existing member's role is
updated by posting their `userId` and new role to the same endpoint, not by
PATCHing a member URL:
```json
{"userId": "user-id", "role": "admin"}
```

Only the current owner can transfer ownership, and only to an existing member
after fresh sign-in. The owner cannot be demoted or removed as an ordinary
member update. Default-organization members cannot be managed here.

### Sharing Resources

**UI:** Settings → Organization → Sharing

Create the share as an owner or admin of the source organization. An owner or
admin of the target organization then accepts it in **Sharing → Incoming**.
Until acceptance, the share is pending and does not grant access.

**API contract:** `POST /api/orgs/{id}/shares`, session authentication only:
```json
{
  "targetOrgId": "other-org-id",
  "resourceType": "vm",
  "resourceId": "vm:101",
  "accessRole": "viewer"
}
```

Use `accessRole`, not `role`. Supported resource types are `vm`, `container`,
`agent`, `storage`, `pbs` and `pmg`; `host` is not supported. Use the resource ID
returned by Pulse, not a display name. Changing a share's access role makes it
pending again, so the target must accept the new grant.

**Read-only incoming shares:**
```bash
curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header" \
  http://127.0.0.1:7655/api/orgs/production-datacenter/shares/incoming
```

## Monitoring Multiple Internal Estates

An Enterprise deployment can run one central Pulse server and keep each internal estate in its own organization, so dashboards, alerts, notifications, and audit logs are scoped by organization. The same default node names (`pve`, `pve1`) in different organizations do not collide, because each organization is a separate namespace.

Use this for one company operating many internal sites, teams, departments, or environments. Do not use this as the default MSP model for unrelated customer businesses; MSP client isolation belongs to the provider-hosted client-workspace model with one isolated Pulse runtime per client.

To onboard an internal estate:

1. **Create an organization for the estate** (see [Creating an Organization](#creating-an-organization)).
2. **Create an org-bound API token** with the `agent:report` scope, bound to that estate's organization (`orgId`). A token bound to a single organization automatically routes every agent that uses it into that organization, with no extra header required. Binding also scopes the token: an org-bound token cannot access other organizations, including the default org (bind `default` explicitly if a token genuinely needs it). Legacy unbound tokens keep their default-org access.
3. **Install the estate's agents** (Proxmox host, Docker, Kubernetes) using that token. Their telemetry lands in the selected organization.
4. **(Optional) Alias node names per estate.** If two estates both use the default `pve` hostname and you want them visually distinct, set `--hostname` (or the `PULSE_HOSTNAME` environment variable) on the agent, for example `--hostname "acme-pve1"`. See [UNIFIED_AGENT.md](UNIFIED_AGENT.md).
5. **(Optional) Isolate the agent control plane on its own port.** When remote nodes reach the central server across the internet, enable [Split-Port Agent Ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation) so reports and command WebSockets share a dedicated, firewalled agent port that never exposes the web UI or management API.

Route each estate's alerts into the right internal system with per-organization webhooks or the org-scoped alerts API. See the multi-tenant section of [WEBHOOKS.md](WEBHOOKS.md).

**Licensing:** self-hosted multi-tenant requires an Enterprise license with the `multi_tenant` capability (see [Requirements](#requirements)). MSP licensing is separate and is based on a signed provider MSP license that sets the client workspace cap for isolated client runtimes, not shared-process organizations.

## Settings Panels

When multi-tenant is enabled, **Settings → Organization** shows:

| Panel | Description |
|---|---|
| **Overview** | Organization name, description, creation date |
| **Access** | Member list, invite/remove members, change roles |
| **Sharing** | Outgoing and incoming resource shares |
| **Billing & Plan** | Organization-level plan and license info |

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/orgs` | List organizations the current user can access |
| `POST` | `/api/orgs` | Create a new organization |
| `GET` | `/api/orgs/{id}` | Get organization details |
| `PUT` | `/api/orgs/{id}` | Update organization (session only) |
| `DELETE` | `/api/orgs/{id}` | Delete organization |
| `GET` | `/api/orgs/{id}/members` | List members |
| `POST` | `/api/orgs/{id}/members` | Invite a member or update an existing member's role (session only) |
| `DELETE` | `/api/orgs/{id}/members/{userId}` | Remove a member |
| `POST` | `/api/org-invitations/{id}/accept` | Accept your pending invitation (session only) |
| `GET` | `/api/orgs/{id}/shares` | List outgoing shares |
| `GET` | `/api/orgs/{id}/shares/incoming` | List incoming shares |
| `POST` | `/api/orgs/{id}/shares` | Create or update a share (session only) |
| `POST` | `/api/orgs/{id}/shares/incoming/{shareId}/accept` | Accept an incoming share (session only) |
| `DELETE` | `/api/orgs/{id}/shares/{shareId}` | Remove a share |

### Tenant Context

All data-fetching endpoints respect the active organization context. The active org is determined by:

1. `X-Pulse-Org-ID` header (API clients)
2. Session cookie (browser)
3. Falls back to the `default` organization

Neither the active context nor a token scope overrides organization membership
or a token's organization binding.

## Storage

- The **default** org uses the root data directory (backward compatible).
- Non-default orgs store data in `{data-dir}/orgs/{org-id}/`.
- Organization metadata is stored in `org.json` inside each org directory.
- When multi-tenant is first enabled, legacy single-tenant data is migrated into `orgs/default/` with symlinks for compatibility.

## Troubleshooting

### "Multi-tenant is not enabled on this server" (501)

Set `PULSE_MULTI_TENANT_ENABLED=true` in your environment and restart Pulse.

### "Multi-tenant requires an Enterprise license" (402)

Activate an Enterprise license with the `multi_tenant` capability in **Settings → Plans & Billing**.

### Organization data not loading after switch

1. Hard-refresh the browser (`Ctrl+Shift+R`).
2. Check the Org Switcher dropdown — ensure the correct org is selected.
3. Check Pulse logs for tenant middleware errors.

### Shared resources not appearing

1. Verify the share exists and has been accepted: **Settings → Organization → Sharing → Incoming**.
2. Confirm the share role grants sufficient access.
3. Check that the source org's resources are online.

## See Also

- [Plans & Entitlements](PULSE_PRO.md), multi-tenant availability by plan
- [Pulse Cloud](CLOUD.md), hosted Pulse environment
- [Security](../SECURITY.md), authentication and authorization model
