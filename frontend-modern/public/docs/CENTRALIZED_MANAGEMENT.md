# Centralized Agent Management (Pro/legacy Pro+/Cloud)

Agent profiles let an administrator save supported `pulse-agent` settings and
assign them to installed agents. They are configuration, not an installer,
credential rotation, command-authority grant or proof of fleet-wide deployment.
The `agent_profiles` licence capability is required.

Open **Settings → Infrastructure → Install on a host → Manage agent profiles**.
Keep the selected organisation and agent identity consistent throughout; an
agent's displayed hostname is not its unique Agent ID.

## Concepts

- **Profile**: a named configuration, optionally inheriting from a parent profile.
- **Assignment**: the link from an existing Agent ID to a profile's actual `id`.
  Assigning a different profile replaces that agent's previous assignment.
- **Precedence**: on a successful startup fetch, supported profile keys override
  local flags/environment, except explicit local privacy opt-outs. A local
  `--enable-docker=false` or `PULSE_ENABLE_DOCKER=false` remains a hard opt-out;
  a profile cannot turn Docker/Podman collection back on.

## Supported Configuration Keys

Read **schema** on your running server for its supported keys and validation.
The following keys are applied at agent startup; the three host settings marked
below also support live refresh when the host module is running:

| Key | Type | Description |
| --- | --- | --- |
| `interval` | duration | Reporting interval, for example `30s` or `1m`; also refreshed live for the host module |
| `enable_host` | boolean | Host metrics collection only; does not grant command execution |
| `enable_docker` | boolean | Docker/Podman monitoring, subject to the local opt-out |
| `enable_kubernetes` | boolean | Kubernetes monitoring |
| `enable_proxmox` | boolean | Proxmox monitoring |
| `proxmox_type` | enum | `pve`, `pbs` or `auto` |
| `docker_runtime` | enum | `auto`, `docker` or `podman` |
| `disable_auto_update` | boolean | Disable automatic agent updates |
| `disable_docker_update_checks` | boolean | Disable Docker image update detection |
| `kube_include_all_pods` | boolean | Include all non-succeeded Kubernetes pods |
| `kube_include_all_deployments` | boolean | Include all Kubernetes deployments |
| `log_level` | enum | `debug`, `info`, `warn` or `error` |
| `report_ip` | string | Reported host IP override; also refreshed live for the host module |
| `disable_ceph` | boolean | Disable local Ceph polling; also refreshed live for the host module |

`interval` accepts a positive duration string; JSON numbers are interpreted as
seconds. Do not enable Debug, shorten intervals or change monitoring scope just
to collect a report. Use the [bounded agent log readers](UNIFIED_AGENT.md#collect-agent-logs-safely)
and evidence already available.

### Monitoring is not command authority

`enable_host` controls metrics, not AI or container actions. `commandsEnabled`
is a separate per-agent control, not a profile key. Command use also depends on
the installed local authority, the token's `agent:exec` scope and its permitted
agent binding. A monitoring-only runtime rejects remote command enablement;
changing a profile or adding a token scope cannot promote that runtime.

Leave commands off for monitoring. Do not enable them, reinstall with broader
privileges or replace the token to fix missing metrics. A switch saved on the
server is not proof of an admitted command channel. See
[Agent Security](AGENT_SECURITY.md) and
[blocked remote control](UNIFIED_AGENT.md#commands-enabled-but-remote-control-blocked)
for the separate authority checks.

## Safe profile rollout

1. Read the current profile, assignment and local opt-outs before changing them.
   Retain the previous configuration privately. A shared profile change affects
   every agent assigned to it, including agents inheriting it through a parent.
2. Validate the intended configuration without saving. Validation checks keys
   and values, not host permissions, installed modules, connectivity or workload
   safety. Use one suitable non-production agent before widening a rollout.
3. Save the intended profile and assign it using the returned profile `id` and
   the existing agent's canonical Agent ID, not a guessed name or machine ID.
   Read back both records after saving.
4. Observe normal config fetches and fresh reports from each affected agent.
   A saved profile, deployment-status row or successful config response is not
   proof that every setting was applied or that readings recovered.
5. For startup-only keys, arrange an agent restart through its existing
   deployment during a suitable maintenance window, preserving its image,
   credentials and state. Do not restart, re-enrol or change a host identity
   just to diagnose a missing reading. Monitoring is interrupted during restart;
   arrange independent coverage where needed.

### When settings take effect

The agent fetches configuration on startup. With the host module running,
current agents also refresh about once a minute. The host module can apply
`interval`, `report_ip` and `disable_ceph` without restarting. Other profile
keys, including module enablement, runtime choice, log level and auto-update
settings, require a successful startup fetch; a refresh is not an all-module
live reload. `commandsEnabled` can change live, within the separate authority
boundary above.

An existing process keeps its current settings if a refresh fails. A failed
startup fetch can leave the agent using local settings. A timeout, rejected
signature or authentication failure is not proof of rollback, revocation or
successful application. Preserve the error and use the normal fetch/report
observations; do not disable signature or TLS verification to make it pass.

Unassigning or deleting a profile does not undo settings already applied to a
running process, revoke its token, stop monitoring or revoke command authority.
Rollback restores the selected historical profile content as a **new version**;
it still needs the same fetch/application checks on every affected agent.
Unassignment is not an emergency stop. If collection must stop, use the
established local service procedure rather than relying on a saved assignment;
keep independent monitoring and incident recovery separate.

## API Usage

Prefer the signed-in UI for changes. Profile endpoints require administrator
access, `settings:write` token scope and the `agent_profiles` licence capability,
even for profile reads. An agent reporting token is not a profile-management
credential. Keep the current organisation selected; a denied request is not a
reason to grant unrelated scopes or bypass authentication.

For a read-only API check, prepare the private header file and define `pulse_api`
**in the same Bash session** using [API authentication](API.md#-authentication).
That helper is local example code, not an installed Pulse command. On the Pulse
host it uses loopback with bounded curl requests; use the guide's verified HTTPS
origin for remote access. It saves responses privately, with no redirects or
automatic retries. Never paste a token or session cookie into a command, URL or
thread, or use browser **Copy as cURL**.

```bash
# Read only the records relevant to this change
pulse_api GET /api/admin/profiles/schema
pulse_api GET /api/admin/profiles/
pulse_api GET /api/admin/profiles/assignments
```

Open saved responses locally: profiles, assignments, IP overrides and errors
can reveal infrastructure identities. Share only a relevant manually redacted
excerpt, not full responses, headers, configuration files or tokens. HTTP
401/402/403, redirects and transport errors are not an empty profile list.

### Endpoint reference

These paths describe existing operations; do not run every operation as a
diagnostic checklist. Replace `{id}`, `{agent_id}` and `{version}` with values
from the current instance. Use the UI or an authenticated client that reads
credentials and request bodies from private files, preserves CSRF protection
for session writes and verifies HTTPS. The example `pulse_api` helper supports
GET and POST only, not PUT or DELETE.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/admin/profiles/` | List profiles |
| POST | `/api/admin/profiles/` | Create a profile; returns its generated UUID `id` and version 1 |
| GET / PUT / DELETE | `/api/admin/profiles/{id}` | Read, replace or delete a profile |
| GET / POST | `/api/admin/profiles/assignments` | Read or replace assignments |
| DELETE | `/api/admin/profiles/assignments/{agent_id}` | Unassign a profile; not token or command revocation |
| GET | `/api/admin/profiles/schema` | Supported configuration keys |
| POST | `/api/admin/profiles/validate` | Validate without saving |
| POST | `/api/admin/profiles/suggestions` | Optional AI suggestion; may call the configured AI provider, not a read-only diagnostic |
| GET | `/api/admin/profiles/{id}/versions` | Profile version history |
| POST | `/api/admin/profiles/{id}/rollback/{version}` | Restore historical content as a new version |
| GET | `/api/admin/profiles/changelog` | Recorded profile changes |
| GET / POST | `/api/admin/profiles/deployments` | Read or record deployment status; not independent host acceptance |

Example **request bodies**, not credential-bearing HTTP commands:

```json
{
  "name": "Production Servers",
  "config": {"enable_docker": true, "log_level": "info", "interval": "60s"}
}
```

For validation, send only `{"config": {...}}`. For assignment, use the actual
`id` returned by creation or the profile list, not `prod-servers`:

```json
{
  "agent_id": "<existing-canonical-agent-id>",
  "profile_id": "<returned-profile-uuid>"
}
```

After an uncertain create, update, assignment, rollback or deletion, read the
current profile, assignment and version before considering another write. The
server may already have saved it despite a lost response. Do not blindly repeat
POST, delete configuration files, recreate profiles or clear history to make a
failed banner disappear. Creation/readback alone does not establish delivery.

### Reading an agent's desired configuration

`GET /api/agents/agent/{agent_id}/config` uses a separate credential path:
`agent:config:read` or a compatible existing agent token is restricted by token
binding; `agent:manage` or `settings:write` allows management reads. Use the
appropriate existing credential, never give an agent an administrator token to
work around a denial. The response names the resolved `agentId`; compare it
with the intended canonical agent before interpreting its settings.

This response is the server's **desired configuration**, not an observation of
what the process applied. It can contain private settings and signed metadata;
keep it private. A 404 can mean the agent has not registered, not that a profile
was deleted. Config-signature validation and live readings remain separate
checks; do not re-enrol or reset an agent just to obtain this response.

## Storage

Profiles and assignments live in the selected organisation's Pulse config
storage, not on each monitored host:

- `agent_profiles.json`
- `agent_profile_assignments.json`
- `profile-versions.json`
- `profile-changelog.json`
- `profile-deployments.json`

Deleting a profile removes its direct assignments in server storage; it does
not remove local credentials or provide a verified rollback on remote hosts.
Keep configuration, history and consistent private backups intact during
investigation. See [configuration transfer](MIGRATION.md#configuration-transfer)
for what a backup actually includes.
