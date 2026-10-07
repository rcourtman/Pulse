# 🔌 Pulse API Reference

Pulse provides a comprehensive REST API for automation and integration.

**Base URL**: `http://<your-pulse-ip>:7655/api`

## 🔐 Authentication

Most API requests require authentication via one of the following methods:

### API Token (Recommended)

Create an API token in Pulse's **API Access** settings with only the scopes
needed for your task. The read-only example below requires `monitoring:read`.
Pass the token in the `X-API-Token` header, using a private header file so the
secret does not appear in shell history or process arguments.

On a trusted machine, as the account running curl, prepare the file and open
it in an editor. This preserves an existing file, restricts its directory and
stops before editing if preparation fails or a credential path is symlinked or
not a regular file:

```bash
(
  set -eu
  umask 077
  auth_dir="$HOME/.config/pulse"
  auth_file="$auth_dir/api-header"
  if [ -L "$HOME/.config" ] || [ -L "$auth_dir" ] || [ -L "$auth_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  if [ -e "$auth_file" ] && [ ! -f "$auth_file" ]; then
    printf 'Credential file must be a regular file.\n' >&2
    exit 1
  fi
  mkdir -p "$auth_dir"
  chmod 700 "$auth_dir"
  touch "$auth_file"
  chmod 600 "$auth_file"
  vi "$auth_file"
)
```

In the editor, save just this line, replacing `<token>` with the API token:

```text
X-API-Token: <token>
```

Define this helper in the same Bash session (curl 7.76 or later). The examples
on this page use it to make **one request**, print only the HTTP status and save
each response to a new owner-only file. This keeps resource names, audit details
and credential-bearing error bodies out of terminal output and recordings.

```bash
pulse_api() (
  set -eu
  umask 077
  if [ "$#" -ne 2 ]; then
    printf 'Usage: pulse_api GET|POST /api/path\n' >&2
    exit 2
  fi
  method=$1
  api_path=$2
  case "$api_path" in
    /api/*) ;;
    *) printf 'Use an API path on this Pulse instance, not a full URL.\n' >&2; exit 2 ;;
  esac
  case "$method" in
    GET) set -- ;;
    POST) set -- --header 'Content-Type: application/json' --request POST --data-binary @- ;;
    *) printf 'This example helper accepts only GET and POST.\n' >&2; exit 2 ;;
  esac
  auth_dir="$HOME/.config/pulse"
  auth_file="$auth_dir/api-header"
  if [ -L "$HOME/.config" ] || [ -L "$auth_dir" ] || [ -L "$auth_file" ] || [ ! -f "$auth_file" ]; then
    printf 'Prepare a regular, private header file first.\n' >&2
    exit 1
  fi
  chmod 700 "$auth_dir" || exit "$?"
  chmod 600 "$auth_file" || exit "$?"
  result_file=$(mktemp "$auth_dir/api-response.XXXXXX") || exit "$?"
  curl_exit=0
  status=$(curl --disable --fail-with-body --silent --show-error \
    --proto '=http' --noproxy 127.0.0.1 --connect-timeout 5 --max-time 20 \
    --header "@$auth_file" --output "$result_file" --write-out '%{http_code}' \
    "$@" "http://127.0.0.1:7655$api_path") || curl_exit=$?
  printf 'HTTP %s\nPrivate response: %s\n' "$status" "$result_file"
  [ "$curl_exit" -eq 0 ] || exit "$curl_exit"
  case "$status" in
    2??) ;;
    *) exit 1 ;;
  esac
)
```

Then make a read-only request with a `monitoring:read` token:

```bash
pulse_api GET /api/state/summary
```

The default loopback origin applies only when curl runs on the Pulse host and
explicitly bypasses proxy environment settings. For remote access, replace
`http://127.0.0.1:7655` **in the helper** with your Pulse HTTPS origin, and replace
`--proto '=http' --noproxy 127.0.0.1` with `--proto '=https'`. Keep certificate
verification enabled; for a private CA, add `--cacert` with a CA file you
verified separately, never `--insecure`. Do not put credentials in the URL.

Open the reported response file privately to inspect it, including on failure;
share only the relevant redacted error, not the whole response or file. Each
call preserves earlier responses. HTTP 401 and 403 return curl's non-zero exit
and retain the error body; redirects are not followed and are not success.
The connection limit is five seconds and the whole request limit is twenty
seconds. A partial file after a transport error is not a complete result.
No request is retried automatically, especially a POST whose outcome is
uncertain: inspect the existing action and target before deciding what to do.
A 2xx response is only HTTP success, not proof that an action completed.

Keep `--disable` first: it ignores local curl configuration that could enable
trace output, bypass TLS or follow redirects. On an authentication-enabled
instance, the protected summary checks token access; `/api/health` is public
and does not verify authentication. The helper is local example code, not an
installed Pulse command; define it again in a new shell before using these
examples. Dispose of response files under your normal private-data policy.

Do not paste tokens into command lines, URLs or issue reports. Keep the header
file private and outside shared repositories and diagnostics; do not use curl
verbose/trace output when sharing a result. Share only the relevant redacted
error, not the whole infrastructure response. Revoke tokens that are no longer
needed in Pulse's API Access settings.

### Bearer Token

The same API token can use a Bearer header instead. Replace the header file's
line in the editor with the following, then use the same helper above;
do not send both authentication headers:

```text
Authorization: Bearer <token>
```

### Session Cookie

For one-off read-only diagnostics, use your signed-in Pulse browser to open
the API path on that same instance. Do not extract or paste its session cookie
into a command or report; use the scoped token-file method above for automation.
State-changing session requests also require Pulse's CSRF protection, which
the UI handles.

Session endpoints:
- `POST /api/login` (sets `pulse_session` + `pulse_csrf`)
- `POST /api/logout` (clears session)

Login body:
```json
{ "username": "admin", "password": "secret", "rememberMe": true }
```

Public endpoints include:
- `GET /api/health`
- `GET /api/version`
- `GET /api/agent/version` (agent update checks)
- `GET /api/setup-script` (requires a setup token)

## 🔏 Scopes and Admin Access

Some endpoints require admin privileges and/or scopes. Common scopes include:
- `monitoring:read`
- `monitoring:write`
- `settings:read`
- `settings:write`
- `agent:config:read`
- `agent:manage`
- `actions:plan`, `actions:approve`, `actions:execute`
- `audit:read`

Endpoints that require admin access are noted below.

---

## 📡 Core Endpoints

### System Health
`GET /api/health`
Check if Pulse is running.
```json
{
  "status": "healthy",
  "timestamp": 1700000000,
  "uptime": 3600,
  "devModeSSH": false
}
```

### System State
`GET /api/state`
Returns the complete state of your infrastructure (Nodes, VMs, Containers, Storage, Alerts). This is the main endpoint used by the dashboard.

`GET /api/state/summary`
Returns a lightweight integration summary for external dashboards and checks. Requires `monitoring:read`.

```json
{
  "activeAlerts": 1,
  "nodes": 2,
  "vms": 8,
  "containers": 12,
  "dockerHosts": [
    {
      "name": "Docker Host",
      "containers": 5,
      "uptimeSeconds": 86400,
      "cpuUsagePercent": 12.5
    }
  ],
  "verdicts": {
    "ok": 17,
    "attention": 1,
    "critical": 1,
    "stale": 1,
    "off": 2,
    "unknown": 0
  },
  "attention": [
    {
      "id": "node-1",
      "name": "PVE Node 1",
      "type": "agent",
      "platformType": "proxmox",
      "verdict": "critical",
      "topReason": { "code": "offline" }
    }
  ],
  "lastUpdate": "2026-05-24T10:11:12Z"
}
```

`verdicts` uses the canonical `ok`, `attention`, `critical`, `stale`, `off`,
and `unknown` fleet-health vocabulary. `attention` is severity ordered and
capped at 30 entries. Powered-off workloads are neutral (`off`), and stale or
missing telemetry is never reported as `ok`.

### Simple Stats (HTML)
`GET /simple-stats`
Lightweight HTML status page for quick checks.

### Unified Resources
`GET /api/resources`
Returns the unified resource list with pagination + aggregations. Requires `monitoring:read`.

Query params:
- `type`: comma-separated list (e.g., `agent`, `vm`, `system-container`, `container`, `docker-service`, `storage`, `pbs`, `pmg`, `k8s-cluster`, `k8s-node`, `pod`, `k8s-deployment`, `physical_disk`, `ceph`)
- `source`: comma-separated list (e.g., `proxmox`, `agent`, `docker`, `pbs`, `pmg`, `kubernetes`)
- `excludeSource`: comma-separated list of sources that must not be present on a matching resource
- `status`: comma-separated list (`online`, `offline`, `warning`, `unknown`)
- `parent`: parent resource ID
- `cluster`: cluster name
- `namespace`: Kubernetes namespace (filters Kubernetes resources only)
- `q`: name search (contains match)
- `tags`: comma-separated tags
- `page`: page number (default `1`)
- `limit`: page size (default `50`, max `100`)
- `sort`: `name` (default), `status`, `type`, `lastSeen`
- `order`: `asc` (default) or `desc`

Note: `GET /api/resources` is optimized for list views. Some large, platform-specific fields may be omitted from the list response and are only returned by `GET /api/resources/{id}`.

Each list resource can include a backend-owned `health` envelope with a
canonical `verdict` and stable `reasons` (`code` plus optional compact
`detail`). Consumers should use this envelope for cross-platform posture
instead of deriving health from provider-specific status strings.

#### Guest readings: availability and age

In legacy VM payloads, `disk.usage: -1` means unavailable, not negative usage.
In `GET /api/resources` and `GET /api/resources/{id}`, an unavailable disk
reading is omitted as `metrics.disk`. **Do not replace an absent metric with
zero**; a genuinely observed zero is valid. Allocated virtual-disk capacity
alone does not establish used space inside the guest.

Read `proxmox.diskStatusReason` alongside the value. A `prev-` reason means
retained disk evidence, not a fresh reading. `agent-not-running` can also
represent a general guest-agent HTTP 500 error: it does not establish that the
guest's service is absent or stopped. An absent reason alone does not prove
freshness either.

For memory, use `metrics.memory.observation` for the **selected metric's**
source and age, not a different source's retained `proxmox.memory` facet:

| Observation `state` | Meaning for the consumer |
| --- | --- |
| `current` | Accepted source observation; a normal cache hit can keep the original `observedAt`. Check its age for your use. |
| `last-known` | Retained evidence, not a new measurement. Keep its original age visible; do not record it as a fresh sample. |
| `unavailable` | No usable usage observation; do not treat retained numbers or zero-valued fields as a measurement. |
| Missing or unrecognised | Freshness is unknown, including older payloads without an observation. Do not infer `current`. |

`observedAt` belongs to that source observation. If it is absent or in the
future, its age is unknown; do not substitute the resource's `lastSeen` or
`updatedAt`. In the raw `proxmox.memory` facet, `usageUnavailable: true` can
preserve known capacity without measured usage. An absent `metrics.memory`
is not 0% usage. Conversely, an independently observed PVE or Pulse-agent
memory reading can remain current while QEMU disk collection is deferred;
disk state is not memory provenance.

A running VM, an advancing row timestamp or an OK backup does not prove fresh
guest readings or successful thaw. Do not install or restart an agent or send
live guest-agent probes merely to fill these gaps, especially during a backup,
freeze/thaw or an unresponsive-guest incident. Follow [Backup safety](VM_DISK_MONITORING.md#backup-safety)
and [missing-reading guidance](VM_DISK_MONITORING.md#a-missing-reading-is-not-an-installation-diagnosis).
Recovery needs independent thaw, fresh successful writes to every covered
filesystem and workload liveness; retained API values cannot establish it.

Availability is an additive resource facet. `availability` is the compatibility
summary used by existing clients; `availabilityChecks` contains every check
attached to the resource. Each check can include `correlationState`
(`attached`, `standalone`, `ambiguous`, or `unresolved`), its correlation
rule/reason/candidate count, and an `evidence` envelope with observation and
validity timestamps. Attached targets also add a `checks` relationship and do
not appear as separate `network-endpoint` rows.

`GET /api/resources/stats`
Returns aggregations (counts + health rollups).

`GET /api/resources/k8s/namespaces?cluster=<clusterName>`
Returns namespace-level rollups (pods + deployments) for a Kubernetes cluster. Requires `monitoring:read`.
```json
{
  "cluster": "prod-k8s",
  "data": [
    {
      "namespace": "default",
      "pods": { "total": 12, "online": 10, "warning": 2, "offline": 0, "unknown": 0 },
      "deployments": { "total": 3, "online": 3, "warning": 0, "offline": 0, "unknown": 0 }
    }
  ]
}
```

`GET /api/resources/{id}`
Fetch a single resource by ID.

`GET /api/resources/{id}/children`
Returns child resources for the parent ID.

`GET /api/resources/{id}/metrics`
Returns the resource metrics payload.

`POST /api/resources/{id}/link`
Manually link two resources.
```json
{ "targetId": "resource-id", "reason": "optional note" }
```

`POST /api/resources/{id}/unlink`
Manually unlink two resources.
```json
{ "targetId": "resource-id", "reason": "optional note" }
```

`POST /api/resources/{id}/report-merge`
Report an incorrect merge (creates exclusions).
```json
{ "sources": ["proxmox", "agent"], "notes": "optional note" }
```

### Resource Maintenance and Operator State

Use a timed maintenance window to suppress alerts during planned work on a
resource. This API, including descendant scope, is available in stable v6.4.1.
Choose the resource's `id` from `GET /api/resources` and URL-encode it in the
request path. Do not substitute a display name or a Proxmox VM number.

| Endpoint | Required scope | Result |
| --- | --- | --- |
| `GET /api/resources/{id}/operator-state` | `monitoring:read` | Returns the saved state, or HTTP 404 with `operator_state_not_set` when no state is saved. |
| `PUT /api/resources/{id}/operator-state` | `monitoring:write` | Replaces the entire state and returns the saved record. |
| `DELETE /api/resources/{id}/operator-state` | `monitoring:write` | Removes the entire state, returning HTTP 204 even if it was already absent. |

**PUT replaces the whole record, not just the fields supplied.** Read the current
state first and preserve unrelated settings, such as monitoring mode, lifecycle,
remediation policy, criticality and notes. Treat only `operator_state_not_set`
as an empty record, and stop on other read errors. Coordinate concurrent writers
so a read followed by a PUT does not overwrite someone else's changes.

For a one-time window, merge these fields into that record, replacing the example
times with your intended start and end:

```json
{
  "maintenanceStartAt": "2026-10-15T20:00:00Z",
  "maintenanceEndAt": "2026-10-15T20:30:00Z",
  "maintenanceScope": "resource_and_descendants",
  "maintenanceReason": "Proxmox node maintenance"
}
```

This is a set of fields to merge, not a complete replacement for an existing
record. Remove any `maintenanceRecurrence` when replacing a recurring schedule
with this one-time window. The API rejects a record containing both.

| Field | Meaning |
| --- | --- |
| `maintenanceStartAt`, `maintenanceEndAt` | RFC3339 timestamps. Supply both, with the end strictly after the start. The window is active from the start, inclusive, until the end, exclusive. |
| `maintenanceScope` | `resource` applies only to this resource and is the default. `resource_and_descendants` also covers descendants in Pulse's resource hierarchy, such as a node's guests. |
| `maintenanceReason` | Optional explanation of the planned work. |

The response includes `maintenanceWindowActive` and, while active,
`maintenanceActiveStartAt` and `maintenanceActiveEndAt`. The server sets the
record's `setAt` and `setBy` fields from the request, ignoring client values.

During an active window, matching new alerts and firing/recovery notifications
are suppressed. Applying an active window also clears matching existing alerts
from the active list, retaining history. This is broader than pausing delivery
for an existing incident. Resources outside the scope remain monitored normally.

The window expires automatically, with no re-enable request needed. Subsequent
observations can raise alerts again, subject to normal alert policy and any other
active maintenance window. Previously cleared alerts are not automatically
restored, and expiry does not replay a backlog of maintenance notifications.

To end maintenance early, read the latest record, remove its maintenance fields
and PUT the remaining state back. Do not DELETE the record unless you also intend
to remove its other operator settings. An inherited window must be changed on
the ancestor that owns it.

### Fleet Connections
`GET /api/connections`
Returns the canonical fleet connections ledger with per-row fleet-governance state. Requires admin access with `settings:read`.

The payload is the source of truth for enrollment, liveness, version drift, adapter health, config rollout, credential posture, update posture, and remote-control posture. Consumers must not rebuild those states from provider-specific config stores or display labels.

Define the private-response helper in [Authentication](#-authentication),
with `settings:read` on the token:

```bash
pulse_api GET /api/connections
```

### Unified Action Planning

The existing action API separates planning, approval and execution. Run these
steps separately, inspecting each response before moving on; do not paste the
whole sequence as an unattended recovery script. Use the private header file
and `pulse_api` helper from [Authentication](#-authentication), not a token in a
command or environment assignment. Inspect each reported private response file
before proceeding. For remote access, change the helper as described there,
using your Pulse HTTPS origin with certificate verification enabled.

| Endpoint | Token scope and access |
| --- | --- |
| `GET /api/agent/resource-capabilities/{id}` | `monitoring:read`; lists the resource's advertised capabilities and parameter schemas. |
| `POST /api/actions/plan` | `actions:plan` and permission to plan actions; creates a plan but does not approve or execute it. |
| `POST /api/actions/{id}/decision` | `actions:approve` and permission to approve actions; records an explicit `approved` or `rejected` decision for a pending action, without executing it. |
| `POST /api/actions/{id}/execute` | `actions:execute` and permission to execute actions; dispatches only an approved action or an approval-free executable plan. Dry-run-only plans cannot execute. |
| `GET /api/audit/actions` | `audit:read`, audit-log read permission and licensed audit logging. |
| `GET /api/audit/actions/{id}/events` | The same audit access; returns the action's lifecycle evidence. |

The legacy `ai:execute` scope also permits planning, approval and execution,
but use the narrower action scopes when possible. Approval and execution tokens
must be bound to an authorised user; a scope alone does not bypass approval
policy, separation of duties or step-up requirements. Audit reads through a
browser session additionally require admin access. A `monitoring:read` token is
not an action-control or audit token.

1. **Inspect capabilities.** Choose the canonical resource `id` from
   `GET /api/resources`, not a display name or a VM number. Replace `vm:42`
   throughout these examples and URL-encode it in request paths (`vm%3A42` here).
   An empty capabilities list means there is nothing to plan for that resource.

```bash
pulse_api GET /api/agent/resource-capabilities/vm%3A42
```

2. **Plan only.** Use an advertised capability and its actual parameter schema;
   `restart` and `mode=graceful` are examples, not universal capabilities. Choose
   a request ID for this specific intent. Pulse derives the actor from the
   authenticated credential, not a caller-supplied `requestedBy` value.

```bash
pulse_api POST /api/actions/plan <<'JSON'
{
  "requestId": "manual-recovery-123",
  "resourceId": "vm:42",
  "capabilityName": "restart",
  "params": { "mode": "graceful" },
  "reason": "Recover after confirmed outage"
}
JSON
```

Inspect the returned approval policy, blast radius, expiry and preflight checks.
Keep its `actionId` and reviewed `planHash`; replace `act_...` and `sha256:...`
below with those returned values. Planning is not proof that execution is safe
or currently available.

3. **Decide only if approval is required.** Use an authorised approver's private
   header file. Approve only the reviewed plan; stop on a refusal or stale-plan
   response rather than weakening the policy or creating another recovery action.

```bash
pulse_api POST /api/actions/act_.../decision <<'JSON'
{
  "outcome": "approved",
  "reason": "Inside maintenance window",
  "planHash": "sha256:..."
}
JSON
```

4. **Execute separately.** Only after reviewing a successful decision, or an
   approval-free executable plan, use the executor's authorised header file:

```bash
pulse_api POST /api/actions/act_.../execute <<'JSON'
{
  "reason": "Execute approved recovery",
  "planHash": "sha256:..."
}
JSON
```

If the response is lost or times out, inspect the existing action and target
before retrying: losing the connection does not prove that execution stopped.
An HTTP success is not a substitute for checking the terminal result and the
resource's observed state.

5. **Read the audit and events**, using a private header file with audit access:

```bash
pulse_api GET '/api/audit/actions?resourceId=vm%3A42&limit=10'
```

```bash
pulse_api GET /api/audit/actions/act_.../events
```

When audit logging is unavailable, use Pulse's action detail in the signed-in
UI to inspect the existing action; do not infer success or repeat execution
from an unavailable audit response.

Response:
```json
{
  "actionId": "act_...",
  "requestId": "manual-recovery-123",
  "allowed": true,
  "requiresApproval": true,
  "approvalPolicy": "admin",
  "predictedBlastRadius": ["vm:42", "node-1"],
  "rollbackAvailable": false,
  "message": "Plan created for restart on web-42. Execution requires admin approval and is not performed by this endpoint.",
  "plannedAt": "2026-05-03T10:00:00Z",
  "expiresAt": "2026-05-03T10:05:00Z",
  "resourceVersion": "resource:sha256:...",
  "policyVersion": "policy:sha256:...",
  "planHash": "sha256:...",
  "preflight": {
    "target": "vm:42",
    "currentState": "web-42 is warning",
    "intendedChange": "Restart the VM",
    "dryRunAvailable": false,
    "dryRunSummary": "No provider-supported dry run is advertised for this capability.",
    "safetyChecks": [
      "Resource was resolved from the unified resource registry.",
      "Capability is advertised by the resource contract.",
      "This endpoint plans only; it does not approve or execute the action.",
      "Execution requires admin approval."
    ],
    "verificationSteps": [
      "Refresh the resource and confirm the expected state after execution.",
      "Review /api/audit/actions/{actionId}/events for lifecycle evidence."
    ],
    "generatedAt": "2026-05-03T10:00:00Z"
  }
}
```

### Resource Metadata
User notes, tags, and custom URLs for resources.

- `GET /api/agents/metadata` (admin or `monitoring:read`)
- `GET /api/agents/metadata/{agentId}` (admin or `monitoring:read`)
- `PUT /api/agents/metadata/{agentId}` (admin or `monitoring:write`)
- `DELETE /api/agents/metadata/{agentId}` (admin or `monitoring:write`)

- `GET /api/guests/metadata` (admin or `monitoring:read`)
- `GET /api/guests/metadata/{guestId}` (admin or `monitoring:read`)
- `PUT /api/guests/metadata/{guestId}` (admin or `monitoring:write`)
- `DELETE /api/guests/metadata/{guestId}` (admin or `monitoring:write`)

- `GET /api/docker/metadata` (admin or `monitoring:read`)
- `GET /api/docker/metadata/{containerId}` (admin or `monitoring:read`)
- `PUT /api/docker/metadata/{containerId}` (admin or `monitoring:write`)
- `DELETE /api/docker/metadata/{containerId}` (admin or `monitoring:write`)

- `GET /api/docker/runtimes/metadata` (admin or `monitoring:read`)
- `GET /api/docker/runtimes/metadata/{runtimeId}` (admin or `monitoring:read`)
- `PUT /api/docker/runtimes/metadata/{runtimeId}` (admin or `monitoring:write`)
- `DELETE /api/docker/runtimes/metadata/{runtimeId}` (admin or `monitoring:write`)

### Version Info
`GET /api/version`
Returns version, build time, and update status.
Example response:
```json
{
  "version": "6.0.0",
  "buildTime": "2026-02-21T00:00:00Z",
  "channel": "stable",
  "deploymentType": "systemd",
  "updateAvailable": false,
  "latestVersion": "6.0.0"
}
```
Version fields are returned as plain semantic versions (no leading `v`).

---

## 🖥️ Nodes & Config

### Public Config
`GET /api/config`
Returns a small public config payload (update channel, auto-update enabled).

### List Nodes
`GET /api/config/nodes`

### Add Node
`POST /api/config/nodes`
```json
{
  "type": "pve",
  "name": "Proxmox 1",
  "host": "https://198.51.100.10:8006",
  "user": "root@pam",
  "password": "password"
}
```

### Test Connection
`POST /api/config/nodes/test-connection`
Validate credentials before saving.

### Test Node Config (Validation Only)
`POST /api/config/nodes/test-config`
Validates node config without saving.

### Update Node
`PUT /api/config/nodes/{id}`

### Delete Node
`DELETE /api/config/nodes/{id}`

### Test Node (Legacy)
`POST /api/config/nodes/{id}/test`

### Refresh Cluster Nodes
`POST /api/config/nodes/{id}/refresh-cluster`

### Export Configuration
`POST /api/config/export` (instance admin for the default organization, tenant
manager for a selected tenant, or an API token bound to the selected
organization with `settings:read`)
Request body:
```json
{ "passphrase": "use-a-strong-passphrase" }
```
Returns an encrypted export bundle in `data`. Passphrases must be at least 12 characters.

### Import Configuration
`POST /api/config/import` (instance admin for the default organization, tenant
manager for a selected tenant, or an API token bound to the selected
organization with `settings:write`)
Request body:
```json
{
  "data": "<exported-bundle>",
  "passphrase": "use-a-strong-passphrase"
}
```

---

## 🧭 Setup & Discovery

### Setup Script (Public)
`GET /api/setup-script`
Returns the Proxmox/PBS setup script as a shell-script download. Accepts an
optional legacy `setup_token` query for compatibility. Current downloads
contain no token: paste the separately revealed token only at the silent
terminal prompt, or supply `PULSE_SETUP_TOKEN_FILE` pointing to a mode-0600
regular file in a mode-0700 directory owned by the script's user. Never put a
token in a copied command or URL. Canonical callers must send a supported `type` of `pve` or
`pbs` plus non-empty `host` and `pulse_url`; the route no longer generates
placeholder-host scripts for later repair or reconstructs Pulse identity from
the request origin. The route now shares the same canonical type boundary as
`/api/setup-script-url`, rejecting unsupported node types instead of treating
unknown values as PBS, and it normalizes the supplied `host` before script
generation so downloaded artifacts and rerun URLs preserve the same canonical
node identity as the bootstrap response. The optional `backup_perms=true`
query is supported only for `type=pve`.

### Setup Script URL
`POST /api/setup-script-url` (auth)
Generates a one-time setup token and URL for `/api/setup-script`.
Canonical callers must send a supported `type` of `pve` or `pbs` plus a
non-empty `host`; the backend normalizes that host before minting the setup
token and now returns the canonical bootstrap identity back in the response as
`type`, `host`, `url`, `downloadURL`, `scriptFileName`, `command`, `commandWithEnv`,
`commandWithoutEnv`, `setupToken`, `tokenHint`, and `expires`. The returned
commands are canonical root-or-sudo `curl -fsSL` bootstrap commands for the
generated setup script, while `url`, `downloadURL`, and `scriptFileName` are
the runtime-owned artifact metadata used by copy and manual download surfaces.
The request body is a single canonical JSON object only; unknown fields and trailing JSON are
rejected as invalid request shape, and `backupPerms:true` is supported only for
`type:"pve"`. This route stays on the normal
authenticated bootstrap boundary: when Pulse auth is already configured it
requires a real authenticated session or API token, and setup tokens do not
authorize the request itself. Pulse-managed Proxmox monitor-token names on the
setup/bootstrap path derive from the canonical Pulse endpoint, not request-local
host fallbacks, so setup-script and turnkey node-add flows stay on one
deterministic `pulse-<canonical-scope-slug>` identity per Pulse instance.
The `command`, `commandWithEnv`, and `commandWithoutEnv` fields now contain
identical credential-free commands. They download the complete script before
running it and prompt silently in the root-or-sudo process. The token crosses
the installer boundary through a private file, not process arguments or an
exported secret. `downloadURL` equals the tokenless `url`, so a manual download
also needs the separately revealed token at runtime. `setupToken` is used for
`/api/auto-register`; `tokenHint` remains masked on the setup page. Settings
reveals the token in a separate dialog: run the command first, then copy and
paste the token only at its prompt. The artifact is reused only for the same
host and options while its five-minute expiry is live, and discarded when the
setup modal closes.
Non-frontend consumers must validate the complete artifact, including the
canonical host, type, URLs, filename, masked hint and live expiry. Current
Unified Agents and the shell installer also accept the coherent older-server
artifact during upgrades, but never execute its command text. For new
Proxmox agent enrolment against a newer server, use its current installer;
already enrolled agents keep reporting normally.

### Auto-Register (Public)
`POST /api/auto-register`
Registers a node through the canonical `/api/auto-register` contract using a temporary
setup token carried in the JSON `authToken` field. Canonical callers must send
a supported `type` of `pve` or `pbs`, an explicit `source` marker of `agent` or
`script`, a canonical Pulse-managed `tokenId` in the form
`pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>` matching the requested
type, an explicit `serverName`, and missing-token requests now fail with
`Pulse setup token required`. Incomplete token completion requests now fail
with `tokenId and tokenValue must be provided together`, and other missing
canonical request fields fail with explicit `Missing required canonical
auto-register fields: ...` guidance.
Success responses now carry the canonical stored identity and caller boundary
back to the installer or runtime-side Unified Agent:
`{"status":"success","action":"use_token","type":"pve|pbs","source":"agent|script","host":"https://...","nodeId":"<stored-name>","nodeName":"<stored-name>",...}`.

### Agent Install Command
`POST /api/agent-install-command` (auth)
Generates an API token and install command for agent-based Proxmox setup.

### Discovery
`GET /api/discover` (auth)
Runs network discovery.

### AI Discovery (Service Discovery)
Service discovery is used by Pulse Assistant and the UI to inventory web services and enrich links.

- `GET /api/discovery` (list summaries)
- `GET /api/discovery/status`
- `PUT /api/discovery/settings` (admin, `settings:write`)
- `GET /api/discovery/type/{type}`
- `GET /api/discovery/agent/{agentId}`
- `GET /api/discovery/{type}/{targetId}/{resourceId}`
- `POST /api/discovery/{type}/{targetId}/{resourceId}` (trigger discovery, optional `force`)
- `DELETE /api/discovery/{type}/{targetId}/{resourceId}`
- `GET /api/discovery/{type}/{targetId}/{resourceId}/progress`
- `PUT /api/discovery/{type}/{targetId}/{resourceId}/notes`

### Test Notification
`POST /api/test-notification` (auth)
Broadcasts a WebSocket test event.

---

## 📊 Metrics & Charts

### Chart Data
`GET /api/charts?range=1h`
Returns time-series data for CPU, Memory, and Storage.
**Ranges**: `5m`, `15m`, `30m`, `1h`, `4h`, `12h`, `24h`, `7d`

### Storage Charts
`GET /api/storage-charts`
Returns storage chart data.

### Storage Stats
`GET /api/storage/`
Detailed storage usage per node and pool.

### Recovery (formerly Backups / Snapshots)
Pulse v6 uses the recovery API to provide a platform-agnostic view of backup and snapshot artifacts.
The endpoints below carry the provider-neutral contract covering subjects,
points, rollups, posture, and filter semantics.

- `GET /api/recovery/points`
  - Query params:
    - Core filters: `provider`, `kind`, `mode`, `outcome`, `subjectResourceId`, `rollupId`
    - Time window: `from` (RFC3339), `to` (RFC3339)
    - Paging: `page`, `limit`
    - Normalized filters: `q`, `cluster`, `node`, `namespace`, `scope=workload`, `verification` (`verified` | `unverified` | `unknown`)
- `GET /api/recovery/rollups`
  - Query params: `provider`, `kind`, `mode`, `outcome`, `subjectResourceId`, `rollupId`, `from` (RFC3339), `to` (RFC3339), `page`, `limit`
- `GET /api/recovery/postures`
  - Returns server-derived per-resource protection posture and provider evidence quality.
  - Query params: repeated `resourceId` values (maximum 200), `state` (`protected` | `attention` | `unprotected` | `unknown`), `page`, `limit` (maximum 200)
  - Batch clients must make one bounded request per 200 resource ids, never one request per table row.
  - Unknown identity, permission, history, or collection completeness remains `unknown`; clients must not infer a healthier state from raw backup or snapshot artifacts.
- `GET /api/recovery/series`
  - Returns per-day counts for the activity chart.
  - Query params: same filters as `/api/recovery/points` (except paging), plus `tzOffsetMinutes` (integer; UTC offset minutes for day bucketing)
- `GET /api/recovery/facets`
  - Returns distinct filter values (clusters/nodes/namespaces) and capability flags (size/verification/entity id present).
  - Query params: same filters as `/api/recovery/points` (except paging)

---

## 🔔 Notifications

### Send Test Notification
`POST /api/notifications/test` (admin)
Triggers a test alert to all configured channels.

### Email, Apprise, and Webhooks
- `GET /api/notifications/email` (admin)
- `PUT /api/notifications/email` (admin)
- `GET /api/notifications/apprise` (admin)
- `PUT /api/notifications/apprise` (admin)
- `GET /api/notifications/webhooks` (admin)
- `POST /api/notifications/webhooks` (admin)
- `PUT /api/notifications/webhooks/<id>` (admin)
- `DELETE /api/notifications/webhooks/<id>` (admin)
- `POST /api/notifications/webhooks/test` (admin)
- `GET /api/notifications/webhook-templates` (admin)
- `GET /api/notifications/webhook-history` (admin)
- `GET /api/notifications/email-providers` (admin)

Email and webhook configurations accept optional `tagFilter` (an array of
resource-tag strings) and `tagFilterMode` (`"all"` or `"any"`). An omitted
field is preserved on update; an empty `tagFilter` clears routing and restores
delivery for all resources.

- `GET /api/notifications/health` (admin)
  - Queue health is `degraded` whenever any retained `failed` or `dlq`
    delivery exists, and `unavailable` when queue state cannot be read.
    Recoverable failed attempts that returned to `pending` for retry do not
    degrade health.
  - `queue.attention_required` is the retained terminal-failure count.
    `reason_codes` identifies failed and/or dead-letter state without exposing
    notification content. Counts are retention-bounded: sent, failed, and
    cancelled rows are retained for 7 days; dead-letter rows for 30 days.

### Audit Webhooks (Pro)
- `GET /api/admin/webhooks/audit` (admin, `settings:read`)
- `POST /api/admin/webhooks/audit` (admin, `settings:write`)
  - Body: `{ "urls": ["https://..."] }`
  - Strict JSON contract: unknown fields and trailing payload are rejected.
  - Maximum `20` webhook URLs per update request.
  - URLs are normalized (trimmed) and duplicate entries are ignored.
  - Endpoint fails closed if URL validation runtime is unavailable.

### Advanced Reporting (Pro)
- `GET /api/admin/reports/catalog` (admin, `settings:read`)
  - Returns the canonical reporting catalog for the settings surface, including locked-state teaser copy, enabled-surface guidance copy, performance-report options, canonical single-report filename subject, canonical fallback filename date style, and the nested VM inventory export definition.
  - Metadata route: readable without the `advanced_reporting` feature so locked admin surfaces can render the same reporting definition before upsell.
- `POST /api/admin/reports/generate` (admin, `settings:read`)
  - Body fields: `format` (pdf/csv, default `pdf`), `resourceType`, `resourceId`, `metricType` (optional), `start`/`end` (RFC3339, optional; defaults to last 24h), `title` (optional)
  - If `title` is omitted, the backend applies the canonical default title for that resource report.
  - When Pulse Assistant is configured, the configured AI provider narrates a PDF's executive summary; each narration counts against the AI budget and appears in AI usage. If the call fails, times out or the budget is spent, the PDF falls back to the deterministic summary. CSV output is never narrated. The settings UI uses this method.
- `GET /api/admin/reports/generate` (admin, `settings:read`)
  - Takes the same fields as query params and returns the same report, except that it never calls an AI provider: a PDF carries the deterministic summary and a note saying AI narration needs `POST`. Other methods return `405` with `Allow: GET, POST`.
- `POST /api/admin/reports/generate-multi` (admin, `settings:read`)
  - Body fields: `resources` (1-50 entries of `{resourceType,resourceId}`), `format`, `metricType` (optional), `start`/`end` (RFC3339, optional; defaults to last 24h), `title` (optional)
  - If `title` is omitted, the backend applies the canonical default fleet report title.
- `GET /api/admin/reports/inventory/vms/export` (admin, `settings:read`)
  - Query params: `format` (`csv` only; optional and defaults to `csv`)
  - Exports the current fleet-wide VM inventory as spreadsheet-friendly CSV using the canonical runtime model.

Validation and limits:
- `start` and `end` must be RFC3339 when provided.
- Malformed `start`/`end` values, `end` not strictly after `start`, or report windows over 366 days return `400 invalid_time_range`.
- `metricType` must match `[a-zA-Z0-9._:-]+` and be <= 64 chars, otherwise `400 invalid_metric_type`.
- `title` must be <= 256 chars, otherwise `400 invalid_title`.
- Report body max size is 1MB; oversized payloads return `400 body_too_large`.
- Report bodies reject trailing payload and unknown JSON fields with `400 invalid_body`.
- VM inventory export only accepts `csv`.

Common reporting error codes:
- `invalid_format`, `missing_params`, `invalid_resource_type`, `invalid_resource_id`
- `invalid_metric_type`, `invalid_title`, `invalid_time_range`
- `no_resources`, `too_many_resources`, `body_too_large`, `invalid_body`

### Queue and Dead-Letter Tools
- `GET /api/notifications/delivery-log?limit=200` (admin, `settings:read`)
  - Returns recent per-attempt delivery evidence with destination identifiers,
    alert identifiers, outcomes, timestamps, failure classes, and redacted error
    text. Completed attempts are retained for 7 days; dead-letter attempts are
    retained for 30 days, so the response reports both windows explicitly.
  - `limit` defaults to 50 and is capped at 200.
- `GET /api/notifications/queue/stats` (admin)
  - Returns counts for all rows still retained by the queue. Status counts have
    different retention windows and are not a delivery rate or lifetime total.
- `GET /api/notifications/dlq` (admin)
- `POST /api/notifications/dlq/retry` (admin)
- `POST /api/notifications/dlq/delete` (admin)
- `POST /api/notifications/terminal-failures/retry` (admin, `settings:write`)
  - Returns every retained `failed` or `dlq` delivery to `pending` with a fresh
    retry budget. Existing per-attempt delivery history is preserved.
- `POST /api/notifications/terminal-failures/dismiss` (admin, `settings:write`)
  - Marks every retained terminal delivery `cancelled`, clearing the active
    queue-health warning without deleting delivery history.

---

## 🚨 Alerts

Alert configuration and history (requires `monitoring:read`/`monitoring:write`).

For planned downtime, use [Resource Maintenance and Operator State](#resource-maintenance-and-operator-state).
For an existing incident whose notifications should be paused without clearing
it, use the snooze and unsnooze endpoints below.

- `GET /api/alerts/config`
- `PUT /api/alerts/config` — replaces only the top-level keys the body
  carries; a key left out keeps its stored value, and the merged config is
  normalized as before. The body must be a JSON object.
- `GET /api/alerts/deadman/config` — returns only whether an external watchdog
  is configured; `pingUrl` is `***REDACTED***` when present and never returns
  the credential-bearing URL
- `PUT /api/alerts/deadman/config` — body `{ "pingUrl": "..." }`; accepts a
  healthchecks-compatible base success URL, `***REDACTED***` to preserve the
  saved value, or an empty string to remove it
- `GET /api/alerts/deadman/status` — live watchdog health, monitor-loop
  progress, sanitized delivery failure state, and the most recent restart
  interruption; never includes the URL or endpoint fingerprint
- `POST /api/alerts/activate` enables notification delivery and can notify about existing unacknowledged critical alerts. It is not a maintenance toggle, and there is no matching `/api/alerts/deactivate` endpoint.
- `GET /api/alerts/active`
- `GET /api/alerts/delivery-diagnosis?alertIdentifier=<alert-id>` (omit `alertIdentifier` to get the diagnosis array for every active alert)
- `GET /api/alerts/events?alertIdentifier=<alert-id>&type=<event-type,...>&since=<RFC3339>&limit=<n>` — append-only alert event log: lifecycle transitions and notification decisions, including suppressions with reasons; all parameters optional, newest first
- `GET /api/alerts/history`
- `DELETE /api/alerts/history`
- `GET /api/alerts/incidents`
- `POST /api/alerts/incidents/note`
- `POST /api/alerts/bulk/acknowledge`
- `POST /api/alerts/bulk/clear`
- `POST /api/alerts/acknowledge` (body: `{ "alertIdentifier": "alert-id" }`)
- `POST /api/alerts/unacknowledge` (body: `{ "alertIdentifier": "alert-id" }`)
- `POST /api/alerts/snooze` (body: `{ "alertIdentifier": "alert-id", "until": "RFC3339 timestamp" }`; pauses delivery and escalation for up to 30 days while monitoring continues)
- `POST /api/alerts/unsnooze` (body: `{ "alertIdentifier": "alert-id" }`; resumes normal policy without resolving the incident)
- `POST /api/alerts/clear` (body: `{ "id": "alert-id" }`)

---

## 🛡️ Security

### Security Status
`GET /api/security/status`
Returns authentication status, proxy auth state, and security posture flags.

### Change Password
`POST /api/security/change-password`
```json
{ "currentPassword": "old-pass", "newPassword": "new-pass" }
```
In Docker installs, the response includes a restart notice.

### List API Tokens
`GET /api/security/tokens`

### Create API Token
`POST /api/security/tokens`
```json
{ "name": "ansible-script", "scopes": ["monitoring:read"] }
```

### Edit API Token Scopes
`PATCH /api/security/tokens/<id>`
```json
{ "scopes": ["monitoring:read", "settings:read"] }
```

The `scopes` field is required and must contain at least one known scope.
Wildcard access (`"*"`) cannot be combined with other scopes. The update takes
effect on the token's next request without changing its ID, secret, expiry, or
organization bindings.

### Revoke Token
`DELETE /api/security/tokens/<id>`

### Recovery (Localhost or Recovery Token)
`POST /api/security/recovery`
Supports actions:
- `generate_token` (localhost only)
- `disable_auth`
- `enable_auth`

`GET /api/security/recovery` returns recovery mode status.

### Reset Account Lockout (Admin)
`POST /api/security/reset-lockout`
```json
{ "identifier": "admin" }
```
Identifier can be a username or IP address.

### Regenerate API Token (Admin)
`POST /api/security/regenerate-token`

Returns a new raw token (shown once) and updates stored hashes:
```json
{
  "success": true,
  "token": "raw-token",
  "deploymentType": "systemd",
  "requiresRestart": false,
  "message": "New API token generated and active immediately! Save this token - it won't be shown again."
}
```

---

## 🧾 Audit Log (Pro)

These endpoints require admin access and the `settings:read` scope. On Community, the list endpoint returns an empty set and `persistentLogging: false`.

### List Audit Events
`GET /api/audit?limit=100&event=login&user=admin&success=true&startTime=2024-01-01T00:00:00Z&endTime=2024-01-31T23:59:59Z`

Response:
```json
{
  "events": [
    {
      "id": "6b3c9c3c-9a2f-4b3c-9a3b-3d0e8c5c5d45",
      "timestamp": "2024-01-12T10:15:30Z",
      "event": "login",
      "user": "admin",
      "ip": "198.51.100.10",
      "path": "/api/login",
      "success": true,
      "details": "Successful login",
      "signature": "..."
    }
  ],
  "total": 1,
  "persistentLogging": true
}
```

### Verify Audit Event Signature
`GET /api/audit/<id>/verify`

Response:
```json
{
  "available": true,
  "verified": true,
  "message": "Event signature verified"
}
```

### Validate API Token (Admin)
`POST /api/security/validate-token`
```json
{ "token": "raw-token" }
```
Returns:
```json
{ "valid": true, "message": "Token is valid" }
```

### Bootstrap Token Validation (Public)
`POST /api/security/validate-bootstrap-token`

Provide the token via header `X-Setup-Token` or JSON body:
```json
{ "token": "bootstrap-token" }
```

Returns `204 No Content` on success.

### Quick Security Setup (Public, bootstrap token required)
`POST /api/security/quick-setup`

Requires a valid bootstrap token (header `X-Setup-Token`) or an authenticated session.

```json
{
  "username": "admin",
  "password": "StrongPass!1",
  "apiToken": "token",
  "enableNotifications": false,
  "darkMode": false,
  "force": false,
  "setupToken": "optional-bootstrap-token"
}
```

### Apply Security Restart (Systemd Only)
`POST /api/security/apply-restart`
Applies auth changes by restarting the service (systemd deployments only).

---

## ⚙️ System Settings

### Get Settings
`GET /api/system/settings`
Retrieve current system settings.

### Update Settings
`POST /api/system/settings/update`
Update system settings. Requires admin + `settings:write`.

### Legacy System Settings (Read Only)
`GET /api/config/system`
Legacy system settings endpoint (read-only).

### Toggle Mock Mode
`GET /api/system/mock-mode`
`POST /api/system/mock-mode`
`PUT /api/system/mock-mode`
Enable or disable mock data generation (dev/demo only).

### SSH Config (Temperature Monitoring)
`POST /api/system/ssh-config`
Writes the SSH config used for temperature collection (requires setup token or auth).

### Verify Temperature SSH
`POST /api/system/verify-temperature-ssh`
Tests SSH connectivity for temperature collection (requires setup token or auth).

### Scheduler Health
`GET /api/monitoring/scheduler/health`
Returns scheduler health, DLQ, and breaker status. Requires `monitoring:read`.

### Updates (Admin)
- `GET /api/updates/check`
- `POST /api/updates/apply`
- `GET /api/updates/status`
- `GET /api/updates/stream`
- `GET /api/updates/plan?version=X.Y.Z` (optional `channel`, accepts `v` prefix)
- `GET /api/updates/history`
- `GET /api/updates/history/entry?id=<event_id>`

### Infrastructure Updates
- `GET /api/infra-updates` (requires `monitoring:read`)
- `GET /api/infra-updates/summary` (requires `monitoring:read`)
- `POST /api/infra-updates/check` (requires `monitoring:write`)
- `GET /api/infra-updates/agent/{agentId}` (requires `monitoring:read`)
- `GET /api/infra-updates/{resourceId}` (requires `monitoring:read`)

### Diagnostics
- `GET /api/diagnostics` (auth)
- `POST /api/diagnostics/docker/prepare-token` (admin, `settings:write`)

The Docker migration token request accepts `agentId`, optional `tokenName`, and
optional `enableHost`. When `enableHost` is omitted, the generated token and
install command enable both host and Docker monitoring so the machine appears
in both Hosts and Docker. Set `enableHost` to `false` only for an intentional
workload-only agent; that mode receives Docker-report scope only.

### Logs (Admin)
- `GET /api/logs/stream` (server-sent stream)
- `GET /api/logs/download` (bundled logs)
- `GET /api/logs/level`
- `POST /api/logs/level` (set log level)

### Server Info
`GET /api/server/info`
Returns minimal server info for installer scripts.

---

## 🔑 OIDC / SSO

### Provider Login
- `GET /api/oidc/{providerID}/login`
- `GET /api/oidc/{providerID}/callback`

OIDC and SAML configuration is managed via SSO providers.

### SSO Provider Management (Community)
- `GET /api/security/sso/providers` (admin)
- `POST /api/security/sso/providers` (admin)
- `GET /api/security/sso/providers/{id}` (admin)
- `PUT /api/security/sso/providers/{id}` (admin)
- `DELETE /api/security/sso/providers/{id}` (admin)

Provider mutation request contract:
- Max request body: 1MB.
- Strict JSON contract: unknown fields and trailing payload are rejected.
- Provider IDs must match server validation.
- OIDC and SAML providers are included with the Community SSO entitlement.

### SSO Test and Metadata Preview (Community)
- `POST /api/security/sso/providers/test` (admin)
- `POST /api/security/sso/providers/metadata/preview` (admin)

Test/preview request contract:
- Max request body: 32KB.
- Strict JSON contract: unknown fields and trailing payload are rejected.
- Common errors include `invalid_json`, `validation_error`, `rate_limited`, `body_too_large`.

---

## 💳 License (Relay / Pro / legacy Pro+ / Cloud)

### License Status (Admin)
`GET /api/license/status`

### License Features (Authenticated)
`GET /api/license/features`

### Activate License (Admin)
`POST /api/license/activate`
```json
{ "license_key": "PASTE_KEY_HERE" }
```

### Clear License (Admin)
`POST /api/license/clear`

---

## 👥 RBAC / Role Management (Pro)

Role-based access control endpoints for managing roles and user assignments. Requires admin access and the `rbac` license feature.

### List Roles
`GET /api/admin/roles`
Returns all defined roles.

### Create Role
`POST /api/admin/roles`
```json
{
  "id": "operator",
  "name": "Operator",
  "description": "Can view and manage alerts",
  "permissions": [
    { "action": "read", "resource": "alerts" },
    { "action": "write", "resource": "alerts" }
  ]
}
```

### Update Role
`PUT /api/admin/roles/{id}`
Update an existing role's name, description, or permissions.

### Delete Role
`DELETE /api/admin/roles/{id}`

### List Users
`GET /api/admin/users`
Returns all users with their role assignments and, when available, mutable SSO presentation fields (`displayName`, `email`, `providerType`, `providerId`, and `lastLoginAt`). The opaque `username` remains the stable authorization principal.

### Set User Roles
`PUT /api/admin/users/{username}/roles`
```json
{ "roleIds": ["operator", "viewer"] }
```

### Remove User Access
`DELETE /api/admin/users/{username}`

Deletes the Pulse RBAC identity and all assignments and revokes its active sessions. Administrators cannot remove their own current identity. This does not disable the upstream IdP account; a later authorized SSO login recreates the Pulse user record.

> **Note**: OIDC group-to-role mapping can automatically assign roles on login. See [OIDC.md](OIDC.md) for configuration.

---

## 🏢 Organizations (Enterprise)

Multi-tenant organization management. Requires `PULSE_MULTI_TENANT_ENABLED=true` and an Enterprise license with the `multi_tenant` feature. All endpoints require authentication.

See [MULTI_TENANT.md](MULTI_TENANT.md) for setup and architecture details.

### List Organizations
`GET /api/orgs` (requires `settings:read`)
Returns organizations accessible to the authenticated user.

### Create Organization
`POST /api/orgs` (requires `settings:write`, session auth only)
```json
{ "id": "acme-corp", "displayName": "Acme Corporation" }
```
The creator becomes the owner and first member. Organization IDs use letters, digits, periods, underscores or hyphens, 1–64 characters, but cannot be `.` or `..`. Prefer a simple lowercase/hyphen ID such as the example.

### Get Organization
`GET /api/orgs/{id}` (requires `settings:read`)
Returns organization details. User must be a member.

### Update Organization
`PUT /api/orgs/{id}` (requires `settings:write`, session auth only)
```json
{ "displayName": "Updated Name" }
```
Admin or owner role required. The default organization cannot be updated.

### Delete Organization
`DELETE /api/orgs/{id}` (requires `settings:write`, session auth only)
Admin or owner role required. The default organization cannot be deleted.

### List Members
`GET /api/orgs/{id}/members` (requires `settings:read`)
Returns all members with their roles. User must be a member of the org.

### Add or Update Member
`POST /api/orgs/{id}/members` (requires `settings:write`, session auth only)
```json
{ "userId": "jane", "role": "editor" }
```
Roles: `owner`, `admin`, `editor`, `viewer`. Admin or owner role required. A new user receives a pending invitation (`202`) and must accept it in Pulse before gaining membership. Posting an existing member's `userId` updates their role; there is no member PATCH endpoint. Setting role to `owner` transfers ownership only to an existing member, by the current owner after fresh sign-in. Default org members cannot be managed.

### Remove Member
`DELETE /api/orgs/{id}/members/{userId}` (requires `settings:write`, session auth only)
Admin or owner role required. The organization owner cannot be removed.

### List Outgoing Shares
`GET /api/orgs/{id}/shares` (requires `settings:read`)
Returns resources shared outbound from this organization to others.

### List Incoming Shares
`GET /api/orgs/{id}/shares/incoming` (requires `settings:read`)
Returns resources shared inbound to this organization from other organizations.

### Create Share
`POST /api/orgs/{id}/shares` (requires `settings:write`, session auth only)
```json
{
  "targetOrgId": "partner-org",
  "resourceType": "vm",
  "resourceId": "vm-101",
  "resourceName": "Web Server",
  "accessRole": "viewer"
}
```
Share a resource with another organization, using the resource type and ID returned by Pulse. Supported types include `vm`, `system-container`, `agent`, `node`, `docker-host`, `storage`, `pbs` and `pmg`; generic `host` and `container` types are not supported. Access roles: `viewer`, `editor`, `admin`. Admin or owner role required on the source org. The share is pending until a target-org admin or owner accepts it in Pulse; changing its access role requires acceptance again.

### Delete Share
`DELETE /api/orgs/{id}/shares/{shareId}` (requires `settings:write`, session auth only)
Revoke a resource share. Admin or owner role required.

---

## 🤖 Pulse Intelligence

**Paid gating:** endpoints labeled with a paid plan require the relevant Relay, Pro, legacy Pro+, or Cloud capability and return `402 Payment Required` if the feature is not licensed.

### Get AI Settings
`GET /api/settings/ai`
Returns current AI configuration (providers, models, patrol status). Requires admin + `settings:read`.

### Update AI Settings
`PUT /api/settings/ai/update` (or `POST /api/settings/ai/update`)
Configure AI providers, API keys, and preferences. Requires admin + `settings:write`.

### List Models
`GET /api/ai/models`
Lists models available to the configured providers (queried live from provider APIs).

### Provider Tests (Admin)
- `POST /api/ai/test`
- `POST /api/ai/test/{provider}`

### Legacy Anthropic OAuth Cleanup
Anthropic subscription OAuth is unsupported. These routes remain only for
fail-closed compatibility and token cleanup:
- `POST /api/ai/oauth/start` (admin): returns `501` with `unsupported_anthropic_oauth`
- `POST /api/ai/oauth/exchange` (admin): returns `501` with `unsupported_anthropic_oauth`
- `GET /api/ai/oauth/callback` (public): redirects to settings with `ai_oauth_error=unsupported` unless the provider supplied a specific error
- `POST /api/ai/oauth/disconnect` (admin): clears stored legacy OAuth tokens

### Execute (Chat + Tools)
`POST /api/ai/execute`
Runs an AI request which may return tool calls, findings, or suggested actions.

### Execute (Streaming)
`POST /api/ai/execute/stream`
Streaming variant of execute (used by the UI for incremental responses).

### Assistant Chat & Sessions
- `GET /api/ai/status`
- `POST /api/ai/chat` (streaming)
- `GET /api/ai/sessions` returns session summaries; scoped Assistant handoffs may include a safe `handoff_summary` marker without model-only context text or command payloads.
- `POST /api/ai/sessions`
- `DELETE /api/ai/sessions/{id}`
- `GET /api/ai/sessions/{id}/messages`
- `POST /api/ai/sessions/{id}/abort`
- `POST /api/ai/sessions/{id}/summarize`
- `POST /api/ai/sessions/{id}/fork`
- Legacy OpenCode-style file-change routes (`GET /api/ai/sessions/{id}/diff`,
  `POST /api/ai/sessions/{id}/revert`, `POST /api/ai/sessions/{id}/unrevert`)
  return `501 Not Implemented`; Pulse Assistant sessions do not own file diffs
  or file-level revert.

### Question Answers
- `POST /api/ai/question/{id}/answer`

### Kubernetes AI Analysis (Compatibility)
`POST /api/ai/kubernetes/analyze`
```json
{ "cluster_id": "cluster-id" }
```
Requires Pro, legacy Pro+, or Cloud with the `kubernetes_ai` feature enabled. This route remains
available for compatibility, but current v6 Pulse Pro marketing does not treat
Kubernetes-specific analysis as a standalone plan pillar.

### Alert Investigation (Pro)
`POST /api/ai/investigate-alert`
Runs a focused investigation for an alert payload (used by the UI).

### Patrol
- `GET /api/ai/patrol/digest`
  - Returns the "what Patrol did for you" rollup for the last `days` days
    (query `days`, 1–30, default 7): the window and whether retained run
    history covers it, the effective Patrol mode, runs (total, scheduled,
    event-triggered, manual, failed, checks, resources covered, last run),
    findings (new, still open by severity, resolved, auto-resolved,
    dismissed, suppressed), investigations by outcome, Patrol-origin actions
    (proposed, approved, rejected, executed, verified, failed, pending),
    alerts Patrol reviewed, and estimated model spend with a pricing-known
    flag.
  - Computed from records Pulse already retains (run history, the findings
    store, canonical action audits, and usage cost events); nothing new is
    persisted. Requires the `ai:execute` scope. Backs the Patrol page's
    "This week" card; see `docs/PATROL_WEEKLY_DIGEST.md`.
- `GET /api/ai/patrol/attention`
  - Returns the typed Patrol attention queue projected from canonical
    operational lifecycle records. This is the active-count and queue source
    used by both navigation and Patrol.
  - Query params: `filter` (`active` | `open` | `acknowledged` | `suppressed` |
    `stale_unknown` | `resolved` | `all`), `page` (minimum 1), and `limit`
    (1–200).
  - The response includes `data`, a lifecycle-wide `summary`, and bounded
    pagination `meta`. Protection context is joined in one bounded batch, not
    fetched per item.
- `GET /api/ai/patrol/attention/summary`
  - Returns the canonical active, open, acknowledged, suppressed,
    stale/unknown, and recent-resolved counts plus `calm`, `coverageState`, and
    `evaluatedAt`.
  - A lifecycle-read failure returns a typed unavailable error. It never
    returns a synthetic zero or healthy state.
- `GET /api/ai/patrol/attention/{id}`
  - Returns one attention item with its operational record, lifecycle
    timeline, typed evidence, recommended next step, relationships, and
    protection posture.
- `GET /api/ai/patrol/attention/{id}/evidence/{evidenceId}`
  - Returns one exact retained evidence envelope with its current freshness.
  - Returns `410 attention_evidence_detail_expired` when the operational
    record still links the ID but the bounded detail has expired.
- `POST /api/ai/patrol/attention/{id}/acknowledge`
- `POST /api/ai/patrol/attention/{id}/unacknowledge`
- `POST /api/ai/patrol/attention/{id}/suppress`
  - Body: `{ "reason": "...", "expiresAt": "<RFC3339>" }`.
  - The expiry must be in the future and no more than 30 days away.
- `POST /api/ai/patrol/attention/{id}/unsuppress`
- `POST /api/ai/patrol/attention/{id}/actions/restart/plan`
  - Creates or replays the one server-owned Docker restart plan attached to
    this operational record and its exact evidence IDs.
  - Requires the canonical action authorization, an eligible server-side
    offer, and the Pulse Pro `ai_autofix` entitlement. Clients cannot supply
    command authority or override the target.
- Attention reads require `monitoring:read`; lifecycle mutations require
  `monitoring:write`. Action decision and execution use `/api/actions` and
  retain their existing action-specific scopes.
- Attention IDs and evidence IDs are opaque and can contain slashes. Clients
  must path-escape each ID.
- `GET /api/ai/patrol/autonomy`
- `PUT /api/ai/patrol/autonomy`
- `GET /api/ai/patrol/status`
- `GET /api/ai/patrol/findings`
- `DELETE /api/ai/patrol/findings` (clear all findings)
- `GET /api/ai/patrol/objectives`
- `POST /api/ai/patrol/objectives`
- `GET /api/ai/patrol/objectives/{id}`
- `PATCH /api/ai/patrol/objectives/{id}`
- `DELETE /api/ai/patrol/objectives/{id}?revision={revision}`
  - Objective writes accept an outcome-oriented `brief`, optional
    `optional_context`, and optional canonical `resource_ids`. `PATCH` also
    accepts `status` (`active`, `paused`, or `archived`) and requires the
    current `revision` in its JSON body.
  - Responses include server-derived `coverage` (`covered`, `degraded`, or
    `uncovered`) and observer lifecycle state. Clients cannot submit either
    field or mark an objective covered. A saved objective remains uncovered
    until core records a validated, installed read-only observer with a live
    health lease.
  - Reads and writes require `ai:execute`. Stale writes return
    `409 patrol_objective_revision_conflict`.
- `GET /api/ai/patrol/history`
- `GET /api/ai/patrol/runs`
- `GET /api/ai/patrol/stream` (Pro)
- `POST /api/ai/patrol/run` (admin, Pro)
- `POST /api/ai/patrol/acknowledge` (Pro)
- `POST /api/ai/patrol/dismiss`
- `POST /api/ai/patrol/findings/note`
- `POST /api/ai/patrol/resolve`
- `POST /api/ai/patrol/snooze` (Pro)
- `POST /api/ai/patrol/suppress` (Pro)
- `GET /api/ai/patrol/suppressions` (Pro)
- `POST /api/ai/patrol/suppressions` (Pro)
- `DELETE /api/ai/patrol/suppressions/{id}` (Pro)
- `GET /api/ai/patrol/dismissed` (Pro)

### Findings & Investigations
- `GET /api/ai/unified/findings`
- `GET /api/ai/findings/{id}/investigation`
- `GET /api/ai/findings/{id}/investigation/messages`
- `POST /api/ai/findings/{id}/reinvestigate`
- `POST /api/ai/findings/{id}/reapprove` (Pro)

### Approvals & Command Execution (Pro)
- `GET /api/ai/approvals`
- `GET /api/ai/approvals/{id}`
- `POST /api/ai/approvals/{id}/approve`
- `POST /api/ai/approvals/{id}/deny`
- `POST /api/ai/run-command` (execute an approved command)
- `GET /api/ai/agents` (connected agents via `/api/agent/ws`)

### Remediation Plans (Pro)
- `GET /api/ai/remediation/plans`
- `GET /api/ai/remediation/plan?plan_id=<id>`
- `POST /api/ai/remediation/approve`
- `POST /api/ai/remediation/execute`
- `POST /api/ai/remediation/rollback`

Request bodies:
- `approve`: `{ "plan_id": "...", "approved_by": "api" }`
- `execute`: `{ "execution_id": "..." }`
- `rollback`: `{ "execution_id": "..." }`

### Intelligence & Forecasting
- `GET /api/ai/intelligence`
- `GET /api/ai/intelligence/patterns`
- `GET /api/ai/intelligence/predictions`
- `GET /api/ai/intelligence/correlations`
- `GET /api/ai/intelligence/changes` (canonical unified-resource timeline first, patrol-local memory fallback)
- `GET /api/ai/intelligence/baselines`
- `GET /api/ai/intelligence/remediations`
- `GET /api/ai/intelligence/anomalies`
- `GET /api/ai/intelligence/learning`
- `GET /api/ai/forecast` (params: `resource_id`, `metric`, optional `resource_name`, `horizon_hours`, `threshold`)
- `GET /api/ai/forecasts/overview` (params: `metric`, `horizon_hours`, `threshold`)
- `GET /api/ai/learning/preferences` (optional `resource_id`)
- `GET /api/ai/proxmox/events`
- `GET /api/ai/proxmox/correlations`
- `GET /api/ai/incidents` (optional `resource_id`, `limit`)
- `GET /api/ai/incidents/{resourceId}` (optional `limit`)
- `GET /api/ai/circuit/status`

### Knowledge Base
- `GET /api/ai/knowledge?guest_id=<id>`
- `POST /api/ai/knowledge/save`
- `POST /api/ai/knowledge/delete`
- `GET /api/ai/knowledge/export?guest_id=<id>`
- `POST /api/ai/knowledge/import`
- `POST /api/ai/knowledge/clear`

### Debug
- `GET /api/ai/debug/context` (admin)

### Cost Tracking
- `GET /api/ai/cost/summary`
- `GET /api/ai/patrol/cost-preview` (optional `model` as `provider:model`, `interval_minutes`; projects Patrol's 30-day cost for a model and schedule from Pulse's price table and the install's run history, with the per-run token assumption, 30-day spend against budget, and a recommended schedule)
- `GET /api/ai/patrol/model-guidance` (recommended / suggested / caution markers for the Patrol model pickers, plus this install's cached readiness pass)
- `POST /api/ai/cost/reset` (admin)
- `GET /api/ai/cost/export` (admin)

## 📈 Metrics Store

Auth required: `monitoring:read`.

### Store Stats
`GET /api/metrics-store/stats`
Returns stats for the persistent metrics store (SQLite-backed).

### History
`GET /api/metrics-store/history`
Returns historical metric series for a resource and time range.

Query params:
- `resourceType` (required): `node`, `storage`, `agent`, `disk`, `k8s`, `vm`, `system-container`, `oci-container`, `app-container`, `docker-host`
- `resourceId` (required)
- `metric` (optional): `cpu`, `memory`, `disk`, etc. Omit for all metrics
- `range` (optional): `1h`, `6h`, `12h`, `24h`, `1d`, `7d`, `30d`, `90d` (default `24h`; duration strings also accepted)
- `maxPoints` (optional): Downsample to a target number of points

> **License**: Requests beyond Community's `7d` floor require the paid `long_term_metrics` entitlement. Relay unlocks `14d`, Pro and legacy Pro+ unlock `90d`, and requests beyond the active tier's limit return `402 Payment Required`.
An explicit `metric` returns a `points` array; omitting it returns a `metrics`
object keyed by metric name. Use the source ID and type from the affected
chart's request, not its display name or an internal store type. Older
`container`, `dockerHost`, `dockerContainer`, `guest` and `docker` query values
are unsupported. See [Metrics History](METRICS_HISTORY.md#api-access) for
private one-shot reads, response sources and empty-history troubleshooting.

---

## 🤖 Agent Endpoints

### Unified Agent (Recommended)
`GET /download/pulse-agent`
Downloads the unified agent binary. Without `arch`, Pulse serves the local binary on the server host.

Optional query:
- `?arch=linux-amd64` (supported: `linux-amd64`, `linux-arm64`, `linux-armv7`, `linux-armv6`, `linux-386`, `darwin-amd64`, `darwin-arm64`, `freebsd-amd64`, `freebsd-arm64`, `windows-amd64`, `windows-arm64`, `windows-386`)

The response includes `X-Checksum-Sha256` for verification.

The unified agent combines host, Docker, and Kubernetes monitoring. Use `--enable-docker` or `--enable-kubernetes` to enable additional metrics.

See [UNIFIED_AGENT.md](UNIFIED_AGENT.md) for installation instructions.

### Agent Version
`GET /api/agent/version`
Returns the current server version for agent update checks.

### Agent Fleet Diagnostics
`GET /api/agents/diagnostics` (admin, `settings:read`)
Returns read-only fleet triage for reported host, Docker / Podman, and Kubernetes agents, including liveness, version drift, profile deployment drift, identity-split evidence, and supported repair handoff hints. It does not enqueue remote actions.

### Unified Agent Installer Script
`GET /install.sh`
Serves the universal `install.sh` used to install `pulse-agent` on target machines.

### Unified Agent Installer (Windows)
`GET /install.ps1`
Serves the PowerShell installer for Windows.

### Submit Reports
`POST /api/agents/agent/report` - Agent metrics
`POST /api/agents/docker/report` - Docker container metrics
`POST /api/agents/kubernetes/report` - Kubernetes cluster metrics

### Agent Management
`GET /api/agents/agent/lookup?id=<agent_id>`  
`GET /api/agents/agent/lookup?hostname=<hostname>`  
Looks up an agent by ID or hostname/display name. Requires `agent:report`.

`POST /api/agents/agent/uninstall`  
Agent self-unregister during uninstall. Requires `agent:report`.

`POST /api/agents/agent/unlink` (admin, `agent:manage`)  
Unlinks an agent from a node.

`DELETE /api/agents/agent/{agent_id}` (admin, `agent:manage`)  
Removes an agent from state.

### Agent Linking (Admin)
- `POST /api/agents/agent/link` (admin, `agent:manage`)
- `POST /api/agents/agent/unlink` (admin, `agent:manage`)

### Agent Remote Config
`GET /api/agents/agent/{agent_id}/config`  
Returns the server-side config payload for an agent (used by remote config and debugging). Requires `agent:config:read`.
The `config` object includes the merged desired settings, command enablement
decision, and desired-config metadata. When signing is configured, the
signature remains backward-compatible with legacy agents: it covers
`agentId`, `commandsEnabled`, `settings`, `issuedAt`, and `expiresAt`.
`desiredConfig` is computed from the signed command decision plus the
agent-applied settings keys and returned as tamper-evident metadata for newer
clients to recompute and compare.

```json
{
  "success": true,
  "agentId": "agent-123",
  "config": {
    "commandsEnabled": true,
    "settings": {
      "enable_docker": true
    },
    "desiredConfig": {
      "version": "host-agent-config/v1",
      "hash": "sha256:..."
    },
    "issuedAt": "2026-05-13T17:00:00Z",
    "expiresAt": "2026-05-13T17:15:00Z",
    "signature": "..."
  }
}
```

`PATCH /api/agents/agent/{agent_id}/config` (admin, `agent:manage`)  
Updates server-side config for an agent (e.g., `commandsEnabled`).

### Docker / Podman Module Management (Admin)
These routes manage Docker / Podman telemetry and container actions reported by the Docker / Podman module inside the installed `pulse-agent` binary.

- `POST /api/agents/docker/commands/{commandId}/ack` (`docker:report`)
- `DELETE /api/agents/docker/runtimes/{agentId}` (`docker:manage`, supports `?hide=true` or `?force=true`)
- `POST /api/agents/docker/runtimes/{agentId}/allow-reenroll` (`docker:manage`)
- `PUT /api/agents/docker/runtimes/{agentId}/unhide` (`docker:manage`)
- `PUT /api/agents/docker/runtimes/{agentId}/pending-uninstall` (`docker:manage`)
- `PUT /api/agents/docker/runtimes/{agentId}/display-name` (`docker:manage`)
- `POST /api/agents/docker/runtimes/{agentId}/check-updates` (`docker:manage`)
- `POST /api/agents/docker/runtimes/{agentId}/update-all` (`docker:manage`) — retired, returns `410 Gone`; container updates run as reviewed per-container actions through `/api/actions`
- `POST /api/agents/docker/containers/update` (`docker:manage`) — retired, returns `410 Gone`; use the `/api/actions` plan/decision/execute flow instead

### Kubernetes Agent Management (Admin)
- `DELETE /api/agents/kubernetes/clusters/{clusterId}` (`kubernetes:manage`, supports `?hide=true` or `?force=true`)
- `POST /api/agents/kubernetes/clusters/{clusterId}/allow-reenroll` (`kubernetes:manage`)
- `PUT /api/agents/kubernetes/clusters/{clusterId}/unhide` (`kubernetes:manage`)
- `PUT /api/agents/kubernetes/clusters/{clusterId}/pending-uninstall` (`kubernetes:manage`)
- `PUT /api/agents/kubernetes/clusters/{clusterId}/display-name` (`kubernetes:manage`)

### Agent Profiles (Pro)
`GET /api/admin/profiles` (admin, Pro)
`POST /api/admin/profiles` (admin, Pro)
`GET /api/admin/profiles/{id}` (admin, Pro)
`PUT /api/admin/profiles/{id}` (admin, Pro)
`DELETE /api/admin/profiles/{id}` (admin, Pro)
`GET /api/admin/profiles/schema` (admin, Pro)
`POST /api/admin/profiles/validate` (admin, Pro)
`POST /api/admin/profiles/suggestions` (admin, Pro)
`GET /api/admin/profiles/changelog` (admin, Pro)
`GET /api/admin/profiles/deployments` (admin, Pro)
`POST /api/admin/profiles/deployments` (admin, Pro)
`GET /api/admin/profiles/{id}/versions` (admin, Pro)
`POST /api/admin/profiles/{id}/rollback/{version}` (admin, Pro)
`GET /api/admin/profiles/assignments` (admin, Pro)
`POST /api/admin/profiles/assignments` (admin, Pro)
`DELETE /api/admin/profiles/assignments/{agent_id}` (admin, Pro)

---

## Availability Checks

Agentless availability checks monitor endpoint-only devices and services with
ICMP ping, TCP port, UDP, HTTP, or HTTPS probes. They are managed from
**Settings -> Monitoring -> Availability checks** and are also exposed through
the API for automation.

### Target Management

- `GET /api/availability-targets` (`settings:read`) - List configured targets and latest probe status.
- `POST /api/availability-targets` (`settings:write`) - Add a target.
- `PUT /api/availability-targets/{id}` (`settings:write`) - Update a target.
- `DELETE /api/availability-targets/{id}` (`settings:write`) - Remove a target.
- `POST /api/availability-targets/test` (`settings:write`) - Test an unsaved target.
- `POST /api/availability-targets/{id}/test` (`settings:write`) - Test a saved target.

Target payload fields:

- `name` - Display name.
- `targetKind` - `machine`, `service`, or `device`; defaults to `service`.
- `address` - Hostname, IP address, or URL.
- `protocol` - `icmp`, `tcp`, `udp`, `http`, or `https`. The input alias `ping` is accepted and is stored/returned as canonical `icmp`.
- `port` - Required for `tcp`/`udp`, optional for `http`/`https`, and omitted for `icmp`.
- `path` - Optional HTTP path.
- `udpMode` - For UDP, `response_required` (default) or `open_or_filtered`. The latter reports silence as indeterminate and fails only on an explicit rejection.
- `udpRequest` - UTF-8 request bytes, up to 512 bytes. Required by `response_required`; optional in `open_or_filtered` mode.
- `udpExpectedResponse` - Optional exact UTF-8 response, up to 4096 bytes.
- `enabled` - Whether the target is scheduled.
- `pollIntervalSeconds` - Minimum 10 seconds; defaults to 60.
- `timeoutMillis` - Minimum 250 milliseconds; defaults to 2000.
- `failureThreshold` - Number of consecutive failures before alerting; defaults to 2.
- `linkedResourceId` - Optional resource id hint for attaching the probe facet to an existing resource.
- `certificateMonitoringDisabled` - Explicit HTTPS-only opt-out; certificate validity monitoring is enabled by default.
- `certificateExpiryWarningDays` - HTTPS certificate expiry warning window; defaults to 30 days.
- `observationLocationIds` - Observation locations for this one logical check: `pulse:local` for this Pulse server and `agent:<host-agent-id>` for a connected agent. Agent locations require the Pro `external_probe` entitlement. Include `pulse:local` only when a local observation is wanted; agent-only sets do not also run locally. Each location retains separate evidence rather than treating one path failure as a universal outage.
- `probeAgentId` - Compatibility field for single-location clients. A non-empty registered host-agent ID selects that agent; an explicit empty string selects local execution. When updating only this field, omit `observationLocationIds`. For multi-location updates, send the complete desired `observationLocationIds` set and clear `probeAgentId` to `""` so a legacy value cannot override a single-location edit.

External agents collect observations, but alert evaluation and notification
delivery still depend on the Pulse server. Missing reports are not proof of a
target outage. See [probe coverage and outage limits](CONFIGURATION.md#external-probes-pro).

An explicit `linkedResourceId` is authoritative and fails closed when it
cannot resolve. Without it, Pulse correlates only on one exact normalized IP
or hostname match. Zero matches remain standalone and multiple matches remain
ambiguous; Pulse does not guess. Every configured target remains a distinct
`network-endpoint` in `/api/resources`, including correlated targets. A
correlated target also projects an additive `availability` /
`availabilityChecks` facet onto the matched resource and exposes an outgoing
`checks` relationship from the check resource. The check row remains the owner
of probe status, incidents, evidence, and history.

Example ping-only target:

```json
{
  "name": "Garage temperature sensor",
  "targetKind": "device",
  "address": "garage-sensor.local",
  "protocol": "ping",
  "enabled": true
}
```

The create response and subsequent reads return `"protocol": "icmp"`.

---

## 🐟 TrueNAS

TrueNAS connection management endpoints for adding, testing, and removing TrueNAS SCALE/CORE instances.

### Connection Management (Admin)
- `GET /api/truenas/connections` (admin, `settings:read`) — List configured TrueNAS connections.
- `POST /api/truenas/connections` (admin, `settings:write`) — Add a new TrueNAS connection.
- `POST /api/truenas/connections/test` (admin, `settings:write`) — Test a TrueNAS connection before saving.
- `DELETE /api/truenas/connections/{id}` (admin, `settings:write`) — Remove a TrueNAS connection.

TrueNAS resources (pools, datasets, disks, ZFS snapshots, replication tasks, alerts) are surfaced through the unified `/api/resources` endpoint with `source=truenas`.

---

## 📱 Relay / Pulse Mobile (retiring 31 March 2027)

End-to-end encrypted relay protocol for mobile connectivity.

> Relay pairing endpoints generate the QR code and deep link used by supported Pulse Mobile clients.

### Relay Configuration (Admin, Relay and Above)
- `GET /api/settings/relay` (admin, `settings:read`, Relay+) — Get current relay configuration.
- `PUT /api/settings/relay` (admin, `settings:write`, Relay+) — Update relay configuration.
- `GET /api/settings/relay/status` (admin, `settings:read`, Relay+) — Get relay connection status.

### Mobile Onboarding
- `GET /api/onboarding/qr` (`settings:read`) — Generate QR code for Pulse Mobile pairing.
- `POST /api/onboarding/validate` (`settings:read`) — Validate a mobile onboarding connection.
- `GET /api/onboarding/deep-link` (`settings:read`) — Generate deep-link URL for mobile app.

---

## 🔌 WebSocket Endpoints

- `GET /ws` – Primary UI WebSocket (browser sessions).
- `GET /api/agent/ws` – Agent WebSocket used for AI command execution.

---

> **Note**: This is a summary of the most common endpoints. For a complete list, inspect the network traffic of the Pulse dashboard or check the source code in `internal/api/router.go`.
