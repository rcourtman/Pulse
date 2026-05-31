# Multi-Tenant Organizations (Cloud Enterprise)

Pulse supports isolated, multi-tenant organizations for MSPs, homelabs with multiple environments, and multi-datacenter deployments. Each organization gets its own infrastructure, resources, alerts, and audit log — fully isolated from other organizations on the same Pulse instance.

## Requirements

| Requirement | Detail |
|---|---|
| **Feature flag** | `PULSE_MULTI_TENANT_ENABLED=true` |
| **License** | Enterprise license with `multi_tenant` capability |

Without these, all API calls return `501 Not Implemented` (flag off) or `402 Payment Required` (no license). The **default** organization always works regardless.

## Quick Start

1. Set `PULSE_MULTI_TENANT_ENABLED=true` in your environment and restart Pulse.
2. Activate your Enterprise license in **Settings → Plans**.
3. Go to **Settings → Organization** and click **Create Organization**.
4. Name your organization and assign infrastructure to it.
5. Use the **Org Switcher** in the header bar to switch between organizations.

## Concepts

### Organizations

An organization is a fully isolated monitoring environment:

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

- Share a VM, container, host, or storage resource with another org.
- Assign an access role (`viewer`, `editor`, or `admin`) to the share.
- The receiving org sees shared resources alongside their own, with a share badge.

## Managing Organizations

### Creating an Organization

**UI:** Settings → Organization → Create Organization

**API:**
```bash
curl -X POST http://localhost:7655/api/orgs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Production Datacenter", "description": "EU production infrastructure"}'
```

### Switching Organizations

Use the **Org Switcher** dropdown in the header. When you switch:

- All pages reload with the new organization's data.
- AI chat history is reset (each org has its own context).
- Caches are invalidated and re-fetched.

### Managing Members

**UI:** Settings → Organization → Access

**API:**
```bash
# List members
curl http://localhost:7655/api/orgs/{orgId}/members \
  -H "Authorization: Bearer $TOKEN"

# Add a member
curl -X POST http://localhost:7655/api/orgs/{orgId}/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"userId": "user-id", "role": "editor"}'

# Update role
curl -X PATCH http://localhost:7655/api/orgs/{orgId}/members/{userId} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role": "admin"}'
```

### Sharing Resources

**UI:** Settings → Organization → Sharing

**API:**
```bash
# Create a share
curl -X POST http://localhost:7655/api/orgs/{orgId}/shares \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "targetOrgId": "other-org-id",
    "resourceType": "host",
    "resourceId": "resource-id",
    "role": "viewer"
  }'

# View incoming shares
curl http://localhost:7655/api/orgs/{orgId}/shares/incoming \
  -H "Authorization: Bearer $TOKEN"
```

## Monitoring Multiple Clients (MSP Setup)

A common managed service provider pattern is to run one central Pulse server and keep each client in its own organization, so dashboards, alerts, notifications, and audit logs never mix between clients. The same default node names (`pve`, `pve1`) in different client organizations do not collide, because each organization is a separate namespace.

To onboard a client:

1. **Create an organization for the client** (see [Creating an Organization](#creating-an-organization)).
2. **Create an org-bound API token** with the `agent:report` scope, bound to that client's organization (`orgId`). A token bound to a single organization automatically routes every agent that uses it into that organization, with no extra header required.
3. **Install the client's agents** (Proxmox host, Docker, Kubernetes) using that token. Their telemetry lands in the client's organization, isolated from every other client.
4. **(Optional) Alias node names per client.** If two clients both use the default `pve` hostname and you want them visually distinct, set `--hostname` (or the `PULSE_HOSTNAME` environment variable) on the agent, for example `--hostname "acme-pve1"`. See [UNIFIED_AGENT.md](UNIFIED_AGENT.md).
5. **(Optional) Isolate agent check-in on its own port.** When client nodes reach the central server across the internet, enable [Split-Port Agent Ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation) so agents connect on a dedicated, firewalled port that exposes only `/api/agents/*` and never the web UI or management API.

Route each client's alerts into your ticketing system (ConnectWise and others) with per-organization webhooks or the org-scoped alerts API. See the Multi-tenant / MSP section of [WEBHOOKS.md](WEBHOOKS.md).

**Licensing:** a single multi-tenant instance covers every client organization on that server. You do not need a separate Pro license per client. Self-hosted multi-tenant requires an Enterprise license with the `multi_tenant` capability (see [Requirements](#requirements)).

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
| `PATCH` | `/api/orgs/{id}` | Update organization |
| `DELETE` | `/api/orgs/{id}` | Delete organization |
| `GET` | `/api/orgs/{id}/members` | List members |
| `POST` | `/api/orgs/{id}/members` | Add a member |
| `PATCH` | `/api/orgs/{id}/members/{userId}` | Update member role |
| `DELETE` | `/api/orgs/{id}/members/{userId}` | Remove a member |
| `GET` | `/api/orgs/{id}/shares` | List outgoing shares |
| `GET` | `/api/orgs/{id}/shares/incoming` | List incoming shares |
| `POST` | `/api/orgs/{id}/shares` | Create a share |
| `DELETE` | `/api/orgs/{id}/shares/{shareId}` | Remove a share |

### Tenant Context

All data-fetching endpoints respect the active organization context. The active org is determined by:

1. `X-Pulse-Org-ID` header (API clients)
2. Session cookie (browser)
3. Falls back to the `default` organization

## Storage

- The **default** org uses the root data directory (backward compatible).
- Non-default orgs store data in `{data-dir}/orgs/{org-id}/`.
- Organization metadata is stored in `org.json` inside each org directory.
- When multi-tenant is first enabled, legacy single-tenant data is migrated into `orgs/default/` with symlinks for compatibility.

## Troubleshooting

### "Multi-tenant is not enabled on this server" (501)

Set `PULSE_MULTI_TENANT_ENABLED=true` in your environment and restart Pulse.

### "Multi-tenant requires an Enterprise license" (402)

Activate an Enterprise license with the `multi_tenant` capability in **Settings → Plans**.

### Organization data not loading after switch

1. Hard-refresh the browser (`Ctrl+Shift+R`).
2. Check the Org Switcher dropdown — ensure the correct org is selected.
3. Check Pulse logs for tenant middleware errors.

### Shared resources not appearing

1. Verify the share exists: **Settings → Organization → Sharing → Incoming**.
2. Confirm the share role grants sufficient access.
3. Check that the source org's resources are online.

## See Also

- [Plans & Entitlements](PULSE_PRO.md) — multi-tenant availability by plan
- [Pulse Cloud](CLOUD.md) — hosted multi-tenant environment
- [Security](../SECURITY.md) — authentication and authorization model
