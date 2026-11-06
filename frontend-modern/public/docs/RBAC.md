# Role-Based Access Control (RBAC)

RBAC lets you define custom roles with granular permissions and assign them to users. This restricts what each user can see and do in Pulse.

**Requires:** Pro, legacy Pro+, Cloud, MSP, or Enterprise/custom license with the `rbac` capability.

For plan details, see [PULSE_PRO.md](PULSE_PRO.md). For API endpoints, see [API Reference](API.md#-rbac--role-management-pro).

---

## Concepts

### Roles

A role is a named set of permissions. Each permission is an `(action, resource)` pair:

- **action**: `read`, `write`, `delete`, or `admin`
- **resource**: A Pulse resource type (e.g., `alerts`, `settings`, `nodes`, `ai`)

Pulse ships with built-in roles: `admin` (full access), `operator` (manage alerts and resources), `viewer` (read-only), and `auditor` (audit log access). You can create additional custom roles for more granular control.

### Role Assignment

Users can hold multiple roles. Their effective permissions are combined across all assigned roles. Explicit `deny` rules take precedence over `allow` grants.

### OIDC Group Mapping

When using OIDC/SSO, built-in roles can be automatically assigned based on group membership on every plan. See [OIDC Group-to-Role Mapping](OIDC.md#group-to-role-mapping) for configuration. Creating custom roles and manually managing user assignments require Pro RBAC.

SSO authentication is not an administrator grant. An SSO-only deployment must
map at least one trusted IdP group to the built-in `admin` role (or retain a
configured local administrator) before operators can use instance-administration
routes. Mapping a user to `operator`, `viewer`, or no role never elevates that
user merely because no local administrator is configured.

---

## Quick Start

1. Confirm a Pro, grandfathered Pro+, Cloud, MSP, or Enterprise/custom license is active in **Settings → Plans & Billing**.
2. Go to **Settings → Security → Roles**.
3. Create roles with the permissions you need.
4. Assign roles to users.

An active licence is not an administrator grant. For self-hosted Pulse, the
public Community runtime also does not gain Pro-only hooks when a licence is
activated. Check **Settings → Plans & Billing** for both the licence and runtime
status before changing access. If it shows **Pro runtime missing**, follow its
**Open Pulse Pro downloads** action and the [plans guide](PULSE_PRO.md); do not
buy a second licence or reset the data directory to clear this gate.

Keep an independent administrator recovery path while changing roles. If OIDC
group mappings are configured, the next sign-in replaces manually assigned
roles with the mapped set. Review [automatic role assignment](#automatic-role-assignment-via-oidc)
before relying on a manual assignment for continued access.

If a save or removal response is lost, use
[unconfirmed-change recovery](#a-role-or-user-change-could-not-be-confirmed)
before making another change.

### If Roles or Users is missing or unavailable

- **Controls missing:** Settings navigation checks the runtime's `rbac`
  capability and this session's permission to administer users. A missing menu
  entry alone does not tell you which check failed. Check the licence/runtime
  status above and use an account already authorised for user administration.
- **Roles are unavailable for this session** or **User access is unavailable
  for this session:** the panel cannot use this session's access. Have an
  authorised administrator review the intended assignment; do not remove proxy
  role gating or widen an agent's token to bypass a denial.
- **Failed to load roles** or **Failed to load user assignments:** a failed
  read is not proof that no roles or users exist. Use the panel's **Retry** for
  that read when access is available. Do not create or delete access records to
  test it, and keep tokens, cookies and full API responses out of reports.

### A role or user change could not be confirmed

A timeout, server error or unreadable response can happen **after** Pulse has
saved a role or removed access. It does not establish that the change failed
or was undone. Do not repeat the Save or Delete, recreate a user or role, or
restore an old permission list just to clear the warning.

Keep your working administrator session open. In **Settings → Security**, use
the read-only check for the affected panel, when offered:

- **Roles → Check saved roles** reloads saved role definitions. Review the
  affected role's permissions, not just its name.
- **Users → Check saved access** reloads saved user assignments and roles.
  Review the intended user's identity and complete role list. A saved empty
  role list means no assigned roles, not that the previous roles still apply.

These checks do not repeat the change, send a connection test or change an
account's permissions. Pulse hides the unconfirmed list and closes its editor
so an old draft cannot be mistaken for saved access. If your version does not
offer these buttons, reopen the affected panel to read its saved list before
considering another write. Do not replay the API request as a substitute.

If the read fails, access remains unknown, not empty or unchanged. Use the
panel's read retry only when the intended account has access. If the session
is refused or the account or organisation changed, return to the intended
account and organisation first. Keep existing restrictions and an independent
administrator recovery path. Do not grant broader roles, widen a token or
disable authentication to recover an unread list.

A successful read establishes the saved assignments, not that the intended
user's access works. Check [OIDC role mapping](#automatic-role-assignment-via-oidc)
before relying on a manual assignment: the next sign-in can replace it. User
removal also does not disable the upstream identity-provider account.

For [help](TROUBLESHOOTING.md#-getting-help), give the original action, time,
Pulse version, redacted error and whether the saved-state read succeeded, where
already known. Do not repeat the change to obtain evidence. Keep identities,
permission exports, tokens, cookies and full responses private.

---

## Managing Roles

### Before Using the API

Prefer **Settings → Security → Roles** and **Settings → Security → Users** for
one-off changes. API calls need licensed `rbac`, permission to administer users and a
full-access (`*`) token bound to an authorised administrator. A monitoring token
is not enough. Do not widen an agent's token.

Use [API authentication](API.md#-authentication) to prepare a private header
file and define `pulse_api` **in the same Bash session**. It is example code,
not an installed Pulse command. Never paste tokens or session cookies into
commands, URLs or reports, or use **Copy as cURL**. Use loopback on the Pulse
host. Remotely, use that guide's HTTPS origin with certificate verification enabled.

The helper sends one request with five-second connection and twenty-second
total limits. It prints only HTTP status and saves each response in a new
owner-only file. Inspect errors and results privately. Share only a relevant
redacted excerpt, not full responses, header files or tokens.
HTTP 401/402/403, redirects and transport errors are not an empty role or user list.

**Choose each operation separately, not as a top-to-bottom script.** Keep an
administrator recovery path. Deleting a role or user, or sending an empty role
list, removes access. The helper does not follow redirects or retry. A partial
response or timeout can follow an applied change. Check saved roles and intended
access in the UI or a separate read before writing again. HTTP success does not
prove access or disable an upstream IdP account.

**Role PUT replaces the complete policy.** Read the existing role from
`GET /api/admin/roles`. Preserve its exact ID, `parentId`, `priority`, and each
permission's `effect` and `conditions`, except for intended changes. Missing
`effect` means `allow`, not “keep deny”. Use lowercase `allow` or `deny`. Stop
on failed reads or missing/null `permissions`. Coordinate writers.
Re-read the list after saving, not just the write response.

### Creating a Role

**UI:** Settings → Security → Roles → New Role

Use a new role ID. Built-in roles (`admin`, `operator`, `viewer`,
`auditor`) cannot be modified or deleted. These examples use the new `alert-manager`.

**API:**
```bash
pulse_api POST /api/admin/roles <<'JSON'
  {
    "id": "alert-manager",
    "name": "Alert Manager",
    "description": "Can view and manage alerts",
    "permissions": [
      {"action": "read", "resource": "alerts"},
      {"action": "write", "resource": "alerts"},
      {"action": "read", "resource": "nodes"}
    ]
  }
JSON
```

### Listing Roles

```bash
pulse_api GET /api/admin/roles
```

### Updating a Role

**Full replacement.** [Preserve saved fields](#before-using-the-api).

```bash
pulse_api PUT /api/admin/roles/alert-manager <<'JSON'
{ "permissions": [
    {"action": "read", "resource": "alerts"},
    {"action": "write", "resource": "alerts"},
    {"action": "read", "resource": "nodes"},
    {"action": "read", "resource": "ai"}
  ], "name": "Alert Manager",
  "description": "Updated description" }
JSON
```

### Deleting a Role

```bash
pulse_api DELETE /api/admin/roles/alert-manager
```

---

## Managing User Assignments

### Listing Users and Their Roles

```bash
pulse_api GET /api/admin/users
```

SSO users are displayed using the latest configured username claim and email
when available. The `username` field remains the provider-scoped stable
principal used for authorization.

### Setting Roles for a User

Role assignments are set as a complete list — the user's roles are replaced with the provided set:

Use the stable `username` returned by the user list, not the display name or
email. URL-encode it as one path segment (for example, `:` becomes `%3A` for an
SSO principal). `jane` below is an example local username. Create the custom
role before assigning it; do not run the deletion example first.

```bash
pulse_api PUT /api/admin/users/jane/roles <<'JSON'
{"roleIds": ["alert-manager", "viewer"]}
JSON
```

To clear a user's entire role assignment, including built-in roles, send an empty list:

```bash
pulse_api PUT /api/admin/users/jane/roles <<'JSON'
{"roleIds": []}
JSON
```

Note: Users cannot modify their own role assignments (self-escalation prevention).

### Removing User Access

```bash
pulse_api DELETE /api/admin/users/jane
```

This removes the Pulse identity and all role assignments and revokes its active
sessions. You cannot remove your own current identity. Removal does not disable
the upstream IdP account; a later authorized SSO login recreates the record.

---

## Automatic Role Assignment via OIDC

If you use an OIDC identity provider, Pulse can automatically assign roles based on group membership on each login.

Mapping groups to the built-in `admin`, `operator`, and `viewer` roles is part of Community SSO. Creating custom roles and manually managing user assignments require Pro RBAC.

**UI:** Settings → Security → Single Sign-On → Group Role Mappings

**Environment variable** (legacy env-configured OIDC provider only — it does not apply to providers created through the UI or the SSO provider API):
```bash
# Format: group1=role1,group2=role2
OIDC_GROUP_ROLE_MAPPINGS="oidc-admins=admin,oidc-operators=operator,oidc-viewers=viewer"
```

How it works:
- On each login, Pulse reads the user's groups from the OIDC groups claim.
- Matching groups are mapped to Pulse roles.
- A user can receive multiple roles from multiple group mappings.
- Once a provider has any group role mappings configured, the mapping is authoritative: on every login the user's role assignments are replaced with whatever the mapping resolves to. A login that matches no mapped group resolves to an empty set, which clears the user's existing role assignments. Watch for identity providers that silently drop the groups claim (see the Entra ID group overage warning in [OIDC.md](OIDC.md#microsoft-entra-id-formerly-azure-ad)) — to Pulse that looks the same as losing every group.
- Logins that assign at least one role are logged to the [audit log](AUDIT_LOGGING.md) as `sso_oidc_role_assignment` events. A login that clears a user's roles does not write this event.

See [OIDC documentation](OIDC.md#group-to-role-mapping) for full configuration details.

---

## Organization Roles (Enterprise/Internal Multi-Org)

In Enterprise/internal multi-organization deployments, each organization has its own role hierarchy:

| Role | Permissions |
|------|------------|
| **Owner** | Full control. Can transfer ownership and delete the org. |
| **Admin** | Manage members, shares, and org settings. Cannot transfer ownership. |
| **Editor** | Read/write access to org resources. Cannot manage members. |
| **Viewer** | Read-only access to all org data. |

These organization roles are separate from the RBAC custom roles described above. Organization roles control access within a specific internal organization, while RBAC roles control access to Pulse features globally.

Provider-hosted MSP uses a different boundary: each client workspace is its own isolated Pulse runtime. RBAC inside that runtime controls access for that client, and the provider control plane handles account-level staff access and handoff.

See [Multi-Tenant Organizations](MULTI_TENANT.md) for details.

---

## Example: Team Setup

A typical team configuration:

| User | Role | Access |
|------|------|--------|
| alice | `admin` | Full access to everything |
| bob | `operator` | Can view nodes/VMs and manage alerts |
| carol | `viewer` | Read-only access to monitoring views and metrics |
| monitoring-bot | API token with `monitoring:read` scope | Automated alert polling |

---

## Related Documentation

- [Plans and Entitlements](PULSE_PRO.md) — RBAC availability by plan
- [OIDC / SSO](OIDC.md) — Automatic role assignment from identity providers
- [Audit Logging](AUDIT_LOGGING.md) — Track role changes and access events
- [Multi-Tenant Organizations](MULTI_TENANT.md) — Organization-level roles
- [API Reference](API.md#-rbac--role-management-pro) — RBAC API endpoints
- [Security Policy](../SECURITY.md) — Core security model
