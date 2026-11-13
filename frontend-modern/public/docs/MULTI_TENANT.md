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

1. Set `PULSE_MULTI_TENANT_ENABLED=true` through your [deployment configuration](#applying-the-feature-flag).
2. Activate your Enterprise license in **Settings → Plans & Billing**.
3. Create the organization with the session-authenticated `POST /api/orgs` endpoint (see [Creating an Organization](#creating-an-organization)). This build has no Create Organization button in **Settings → Organization**.
4. Assign infrastructure to the organization.
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

Manage members and shares in the signed-in Pulse UI. Organizations are created through the session-authenticated `POST /api/orgs` endpoint; the UI has no create control.
These changes require **session-based user authentication** and the relevant
organization role; API tokens are rejected with `403 session_required`, even
when they have `settings:write`. The UI handles the session and CSRF protection.
Do not extract a session cookie or CSRF token into a shell command.

The read-only examples use an org-bound token with `settings:read`. Define the
`pulse_api` helper and prepare its private header file from
[API authentication](API.md#-authentication) in the same Bash session. This is
local example code, not an installed Pulse command.
Keep the token out of command lines, URLs and reports. A token's organization
binding is an access boundary: changing a URL or `X-Pulse-Org-ID` header does
not grant it access to another organization.

The helper requires curl 7.76 or later, ignores local trace/verbose defaults,
and applies a five-second connection limit and twenty-second whole-request
limit. It does not follow redirects or retry. It prints only the HTTP status
and the path to a new owner-only file containing the response, including on
HTTP failure. Member identities, resource details and error bodies stay out of
terminal output; open the file privately and share only a relevant redacted
excerpt, not whole member, share or infrastructure responses. A partial file
after a transport failure is not a complete list or proof that no members or
shares exist. HTTP 401/402/403 and redirects are not successful reads; do not
widen a token or use a session cookie to bypass them.

The helper's loopback origin applies on the Pulse host; remotely, follow its
HTTPS instructions and keep certificate verification enabled. Replace the
example organization ID `production-datacenter` with your own, and run each
read separately. The helper's support for other HTTP methods does not make
organization mutations token-authenticated; keep member and share changes in the
signed-in UI with its session and CSRF protection.

## Managing Organizations

### Creating an Organization

**UI:** none in this build; Settings → Organization has no create control.

**API contract:** `POST /api/orgs`, session authentication only:
```json
{"id": "production-datacenter", "displayName": "Production Datacenter"}
```

The creator becomes the owner. `id` is a stable ID of 1–64 characters using
letters, digits, periods, underscores or hyphens, but not `.` or `..`; use a
simple lowercase/hyphen ID such as the example. `displayName` is the name
shown in Pulse. `name` and `description` are not the creation fields.

### Switching Organizations

Use the **Org Switcher** in the header to select an organization. Pulse clears
organization-scoped monitoring and AI state, reconnects the live data stream,
and requests the selected organization's data. This is not a new sign-in, a
transfer of saved infrastructure or a grant of membership.

Before switching, finish or deliberately discard any unsaved edit. If a change
is still being saved, or its result is uncertain, keep the selection unchanged
and follow [unconfirmed changes](#a-settings-change-could-not-be-confirmed).
Drafts must not be carried into a different organization.

Check the header selection and, once loaded, the organization ID in
**Settings → Organization → Overview** before editing. Display names can be
similar; use the stable ID to distinguish them. The **Organization switched**
message is not confirmation that the selected organization’s data loaded or
that the current user can edit it. Follow [switch recovery](#organization-data-not-loading-after-switch)
if the data or controls are unavailable.

### Managing Members

**UI:** Settings → Organization → Access

**Read-only API:**
```bash
pulse_api GET /api/orgs/production-datacenter/members
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

Use `accessRole`, not `role`, and the resource type and ID returned by Pulse,
not a display name. Supported types include `vm`, `system-container`, `agent`,
`node`, `docker-host`, `storage`, `pbs` and `pmg`; the generic types `host` and
`container` are not supported. Changing a share's access role makes it pending
again, so the target must accept the new grant.

**Read-only incoming shares:**
```bash
pulse_api GET /api/orgs/production-datacenter/shares/incoming
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

**Addresses an organization admin can use.** Organization owners and admins can add their organization's infrastructure connections, availability checks and SMTP server, but cannot point them at the Pulse server's own loopback, link-local or cloud metadata addresses. Private network addresses work. An instance administrator can add any address.

Route each estate's alerts into the right internal system with per-organization webhooks or the org-scoped alerts API. See the multi-tenant section of [WEBHOOKS.md](WEBHOOKS.md).

**Licensing:** self-hosted multi-tenant requires an Enterprise license with the `multi_tenant` capability (see [Requirements](#requirements)). MSP licensing is separate and is based on a signed provider MSP license that sets the client workspace cap for isolated client runtimes, not shared-process organizations.

## Settings Panels

When multi-tenant is enabled, **Settings → Organization** shows:

| Panel | Description |
|---|---|
| **Overview** | Organization display name (editable), org ID, creation date, member count |
| **Access** | Member list, invite/remove members, change roles |
| **Sharing** | Outgoing and incoming resource shares |
| **Billing** | Organization plan, usage policies and subscription status (the "Billing & Usage" panel) |

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

### "Multi-tenant functionality is not enabled. Set PULSE_MULTI_TENANT_ENABLED=true to enable." (501)

Apply the flag in your [deployment configuration](#applying-the-feature-flag).

### "Multi-tenant access requires an Enterprise license" (402)

Activate an Enterprise license with the `multi_tenant` capability in **Settings → Plans & Billing**.

### Organization data not loading after switch

1. **Keep the working administrator session.** Check the header selection
   against the intended organization, then the stable ID in **Overview** when
   it loads. If the intended organization is missing from the switcher, an
   existing owner or admin can check saved membership in their own session.
   Do not switch to another organization or grant a wider role just to make a
   failed request pass. Instance roles and organization membership are separate;
   see [RBAC setup and recovery](RBAC.md#quick-start).
2. **Distinguish unread from empty.** An unavailable panel is not proof that saved
   infrastructure, members or shares were removed. In the affected Organization
   panel, use its read-only recovery control:
   - **Reload organization** reads saved organization details.
   - **Reload access** reads saved members and invitations.
   - **Reload sharing** reads saved incoming and outgoing shares.

   Check that the panel identifies the intended organization and finishes
   loading successfully before making changes. These controls do not repeat
   a rename, invitation, role change or sharing change.
3. **Do not rebuild missing-looking state.** If Pulse reports **Failed to load
   organizations. Using default.**, the fallback selection is not evidence that
   another organization's data moved to Default. Do not recreate infrastructure
   there, delete saved configuration or clear browser storage to diagnose it.
   A refused read (401/403) is not an empty list; keep the intended account and
   scope rather than substituting a broader token or changing access rules.

A hard refresh cannot restore membership or prove that a save failed, and it
can discard an unsaved draft. Do not sign out of the working administrator
session, restart Pulse or reinstall agents as a first response to this error.

#### A settings change could not be confirmed

A failed response does not prove that a write failed. Keep the same
organization; do not start a competing change while one is still in progress.
Once it has settled, use the relevant **Reload** control above to inspect saved
state before deciding whether another change is needed. Do not resend an
invitation, repeat a rename or role/share change, or replay it after switching
accounts or organizations. If the saved result still cannot be read, leave
further changes alone rather than guessing or overwriting it.

For an unresolved failure, record the Pulse version, affected panel, time,
organization ID or consistent private alias, and the visible error. Review any
existing logs privately and share only a relevant redacted excerpt. Keep member
identities, resource inventories, tokens, cookies, HAR files and **Copy as cURL**
output private; do not enable Debug logging or weaken HTTPS or scoped access
just to collect a report.

### Shared resources not appearing

1. Verify the share exists and has been accepted: **Settings → Organization → Sharing → Incoming**.
2. Confirm the share role grants sufficient access.
3. Check that the source org's resources are online.

## See Also

- [Plans & Entitlements](PULSE_PRO.md), multi-tenant availability by plan
- [Pulse Cloud](CLOUD.md), hosted Pulse environment
- [Security](../SECURITY.md), authentication and authorization model

## Applying the feature flag

Set `PULSE_MULTI_TENANT_ENABLED=true` in your existing deployment's environment
configuration, not only in an interactive shell. Check
[environment precedence](CONFIGURATION.md#common-overrides-environment-variables)
if a managed value overrides the file you edited.

Apply this deliberate setup change in a maintenance window using your
[deployment-specific controls](DEPLOYMENT_MODELS.md#updates-by-model):

- **Docker or Compose:** recreate or redeploy the existing Pulse server with
  the changed environment. Keep the same pinned image, persistent data mount,
  credentials and other settings. A restart alone does not apply a changed
  container environment.
- **systemd:** update the server's existing environment source. Reload systemd
  if its unit or drop-in changed, then restart the actual Pulse server unit,
  not its agents. Do not start a server that was deliberately stopped.
- **Kubernetes:** update the existing workload controller's configuration and
  let its normal rollout apply the environment. Restarting an old pod without
  changing its template does not set the flag.

Pulse monitoring and alerts are unavailable while its server is stopped or
being replaced. Preserve a working administrator recovery path and check the
running instance afterwards. Enabling the flag is not a licence or membership
grant, and a successful restart or rollout alone does not prove organization
access. Keep the Enterprise capability, session authentication and existing
organization permissions intact. Do not reset data, recreate organizations or
re-enrol agents to clear a 501 response.
