# Centralized Agent Management (Pulse Pro)

Pulse Pro supports centralized management of agent configurations, allowing administrators to define "Configuration Profiles" and assign them to specific agents. This enables bulk updates and consistent configuration across your fleet without manually editing configuration files on each host.

Profiles are managed in the UI: **Settings → Agents → Agent Profiles**.

## Concepts

-   **Agent Profile**: A named collection of configuration settings (e.g., "Production Servers", "Debug Mode").
-   **Assignment**: A link between a specific Agent ID and an Agent Profile.
-   **Precedence**: Server-side profile settings override local agent flags/environment for supported keys.

## Supported Configuration Keys

The following settings can be controlled remotely via profiles:

| Key | Type | Description |
| :--- | :--- | :--- |
| `interval` | string | Set reporting interval (e.g., "30s", "1m") |
| `enable_host` | boolean | Enable/Disable host monitoring (metrics + command execution) |
| `enable_docker` | boolean | Enable/Disable Docker monitoring |
| `enable_kubernetes` | boolean | Enable/Disable Kubernetes monitoring |
| `enable_proxmox` | boolean | Enable/Disable Proxmox monitoring |
| `proxmox_type` | string | Set Proxmox type (`pve`, `pbs`, or `auto`) |
| `docker_runtime` | string | Container runtime preference (`auto`, `docker`, `podman`) |
| `disable_auto_update` | boolean | Disable automatic agent updates |
| `disable_docker_update_checks` | boolean | Disable Docker image update detection |
| `kube_include_all_pods` | boolean | Include all non-succeeded pods in Kubernetes reports |
| `kube_include_all_deployments` | boolean | Include all deployments in Kubernetes reports |
| `log_level` | string | Set agent log level (`debug`, `info`, `warn`, `error`) |
| `report_ip` | string | Override the reported IP address for the agent |
| `disable_ceph` | boolean | Disable local Ceph status polling |

Notes:
- `interval` accepts a duration string. If you send a JSON number, it is interpreted as seconds.
- Docker auto-detection can still enable Docker monitoring if the agent is not explicitly configured. To force-disable Docker, set `PULSE_ENABLE_DOCKER=false` or install with `--disable-docker`.
- `commandsEnabled` (AI command execution) is controlled separately per agent in **Settings → Agents → Unified Agents** and is applied live on report. It is not part of profile settings.

## API Usage

All endpoints require **Admin** authentication and a **Pulse Pro** license.

### 1. Create a Profile

```http
POST /api/admin/profiles/
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "name": "Production Servers",
  "config": {
    "enable_docker": true,
    "log_level": "info",
    "interval": "60s"
  }
}
```

### 2. Assign Profile to Agent

You need the Agent ID (typically the machine ID, visible in the Pulse UI or agent logs).

```http
POST /api/admin/profiles/assignments
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "agent_id": "01234567-89ab-cdef-0123-456789abcdef",
  "profile_id": "prod-servers"
}
```

### 3. List Profiles

```http
GET /api/admin/profiles/
Authorization: Bearer <admin-token>
```

### 4. List Assignments

```http
GET /api/admin/profiles/assignments
Authorization: Bearer <admin-token>
```

### 5. Unassign Profile

```http
DELETE /api/admin/profiles/assignments/{agent_id}
Authorization: Bearer <admin-token>
```

### 6. Get Agent Config (Debugging)

To see what configuration an agent receives:

```http
GET /api/agents/host/{agent_id}/config
Authorization: Bearer <agent-or-admin-token>
```

Requires `host-agent:config:read` (or admin tokens with management scopes).

### 7. Schema, Validation, and Suggestions

Use the schema endpoint to see supported keys and types, and validate configs before saving:

```http
GET /api/admin/profiles/schema
Authorization: Bearer <admin-token>
```

```http
POST /api/admin/profiles/validate
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "config": {
    "interval": "60s",
    "enable_docker": true
  }
}
```

Optional AI suggestions:

```http
POST /api/admin/profiles/suggestions
Authorization: Bearer <admin-token>
Content-Type: application/json
```

### 8. Version History and Rollback

Each profile update increments its version and is stored in `profile-versions.json`.

```http
GET /api/admin/profiles/{id}/versions
Authorization: Bearer <admin-token>
```

Rollback to a specific version:

```http
POST /api/admin/profiles/{id}/rollback/{version}
Authorization: Bearer <admin-token>
```

### 9. Change Log and Deployment Status

Change log entries are stored in `profile-changelog.json`:

```http
GET /api/admin/profiles/changelog
Authorization: Bearer <admin-token>
```

Deployment status is stored in `profile-deployments.json`:

```http
GET /api/admin/profiles/deployments
Authorization: Bearer <admin-token>
```

Update deployment status via:

```http
POST /api/admin/profiles/deployments
Authorization: Bearer <admin-token>
Content-Type: application/json
```

## Agent Behavior

1.  On startup, the agent computes its Agent ID.
2.  It contacts the Pulse server to fetch its configuration profile.
3.  If successful, it applies the remote settings, overriding local flags/env for supported keys.
4.  If the server is unreachable or returns an error, the agent proceeds with its local configuration.
5.  Profile changes take effect on the next agent restart. Command execution toggles are applied dynamically.

## Storage

Profiles and assignments are stored in the Pulse config directory:

- `agent_profiles.json`
- `agent_profile_assignments.json`
- `profile-versions.json`
- `profile-changelog.json`
- `profile-deployments.json`

Deleting a profile automatically removes its assignments.
