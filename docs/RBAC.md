# Role-Based Access Control (RBAC)

RBAC lets you define custom roles with granular permissions and assign them to users. This restricts what each user can see and do in Pulse.

**Requires:** Pro or Cloud license with the `rbac` capability.

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

When using OIDC/SSO, roles can be automatically assigned based on group membership. See [OIDC Group-to-Role Mapping](OIDC.md#group-to-role-mapping-pro) for configuration.

---

## Quick Start

1. Activate a Pro or Cloud license in **Settings → License**.
2. Go to **Settings → Security → Access Control**.
3. Create roles with the permissions you need.
4. Assign roles to users.

---

## Managing Roles

### Creating a Role

**UI:** Settings → Security → Access Control → Create Role

**API:**
```bash
curl -X POST http://localhost:7655/api/admin/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "operator",
    "name": "Operator",
    "description": "Can view and manage alerts",
    "permissions": [
      {"action": "read", "resource": "alerts"},
      {"action": "write", "resource": "alerts"},
      {"action": "read", "resource": "nodes"}
    ]
  }'
```

### Listing Roles

```bash
curl http://localhost:7655/api/admin/roles \
  -H "Authorization: Bearer $TOKEN"
```

### Updating a Role

```bash
curl -X PUT http://localhost:7655/api/admin/roles/operator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Operator",
    "description": "Updated description",
    "permissions": [
      {"action": "read", "resource": "alerts"},
      {"action": "write", "resource": "alerts"},
      {"action": "read", "resource": "nodes"},
      {"action": "read", "resource": "ai"}
    ]
  }'
```

### Deleting a Role

```bash
curl -X DELETE http://localhost:7655/api/admin/roles/operator \
  -H "Authorization: Bearer $TOKEN"
```

---

## Managing User Assignments

### Listing Users and Their Roles

```bash
curl http://localhost:7655/api/admin/users \
  -H "Authorization: Bearer $TOKEN"
```

### Setting Roles for a User

Role assignments are set as a complete list — the user's roles are replaced with the provided set:

```bash
curl -X PUT http://localhost:7655/api/admin/users/jane/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"roleIds": ["operator", "viewer"]}'
```

To remove all custom roles from a user, send an empty list:

```bash
curl -X PUT http://localhost:7655/api/admin/users/jane/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"roleIds": []}'
```

Note: Users cannot modify their own role assignments (self-escalation prevention).

---

## Automatic Role Assignment via OIDC

If you use an OIDC identity provider, Pulse can automatically assign roles based on group membership on each login.

**UI:** Settings → Security → Single Sign-On → Group Role Mappings

**Environment variable:**
```bash
# Format: group1=role1,group2=role2
OIDC_GROUP_ROLE_MAPPINGS="oidc-admins=admin,oidc-operators=operator,oidc-viewers=viewer"
```

How it works:
- On each login, Pulse reads the user's groups from the OIDC groups claim.
- Matching groups are mapped to Pulse roles.
- A user can receive multiple roles from multiple group mappings.
- When at least one mapped role is found, role assignments are updated to match. Note: logins with zero matching groups do not clear existing role assignments.
- Role changes are logged to the [audit log](AUDIT_LOGGING.md) as `oidc_role_assignment` events.

See [OIDC documentation](OIDC.md#group-to-role-mapping-pro) for full configuration details.

---

## Organization Roles (Multi-Tenant)

In multi-tenant deployments (Cloud Enterprise), each organization has its own role hierarchy:

| Role | Permissions |
|------|------------|
| **Owner** | Full control. Can transfer ownership and delete the org. |
| **Admin** | Manage members, shares, and org settings. Cannot transfer ownership. |
| **Editor** | Read/write access to org resources. Cannot manage members. |
| **Viewer** | Read-only access to all org data. |

These organization roles are separate from the RBAC custom roles described above. Organization roles control access within a specific tenant, while RBAC roles control access to Pulse features globally.

See [Multi-Tenant Organizations](MULTI_TENANT.md) for details.

---

## Example: Team Setup

A typical team configuration:

| User | Role | Access |
|------|------|--------|
| alice | `admin` | Full access to everything |
| bob | `operator` | Can view nodes/VMs and manage alerts |
| carol | `viewer` | Read-only access to dashboards and metrics |
| monitoring-bot | API token with `alerts:read` scope | Automated alert polling |

---

## Related Documentation

- [Plans and Entitlements](PULSE_PRO.md) — RBAC availability by plan
- [OIDC / SSO](OIDC.md) — Automatic role assignment from identity providers
- [Audit Logging](AUDIT_LOGGING.md) — Track role changes and access events
- [Multi-Tenant Organizations](MULTI_TENANT.md) — Organization-level roles
- [API Reference](API.md#-rbac--role-management-pro) — RBAC API endpoints
- [Security Policy](../SECURITY.md) — Core security model
