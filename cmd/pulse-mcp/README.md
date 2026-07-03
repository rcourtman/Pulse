# pulse-mcp

`pulse-mcp` is the Model Context Protocol (MCP) adapter for Pulse
Intelligence. It exposes the same governed operations substrate that
Pulse Assistant uses inside the app, but as tools for Claude Desktop,
Claude Code, OpenCode, and any other MCP-speaking client: list findings,
read the fleet, drill into a resource, set operator intent, and run
governed actions.

## Pulse Intelligence surface contract

<!-- pulse-mcp-surface-contract:start -->
- **Pulse Intelligence Core**: Canonical context, governed actions, safety gates, approval state, action audit, and verification shared by Pulse Assistant, Pulse MCP, and Pulse Patrol.
- **Pulse Patrol**: Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.
- **Pulse Assistant** (the native Pulse surface): The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions. Affordances: tools and interactive questions.
- **Pulse MCP**: The external-agent adapter that projects canonical Pulse Intelligence capabilities as MCP tools. Affordances: tools, resources, prompts, and capability metadata.
<!-- pulse-mcp-surface-contract:end -->

The adapter is manifest-driven. Every tool it exposes is one entry in
Pulse's canonical capabilities manifest at `/api/agent/capabilities`.
Adding a request/response capability to the manifest-owned Pulse MCP surface
tool contract extends this server automatically. There is no hardcoded tool
list to keep in sync, and there is no separate MCP-only action path to maintain
beside Pulse Assistant. `tools/list` uses the shared Pulse Intelligence
projection to expose the manifest metadata in each tool description, including
route, auth scope, action mode (`read`, `mixed`, `write`), approval policy
(`scope_only`, `action_plan`), request/response shape, and stable error codes.
Operator-state writes plus governed action and finding lifecycle tools also
expose typed `inputSchema` definitions for their top-level arguments, including
stable enums for criticality, approval decisions, and dismissal reasons.
The same shared projection exposes `resources/list` and `resources/read`
when the manifest contains the canonical fleet and per-resource context
capabilities. Resource URIs use `pulse://resource/<resource-id>` and
`resources/read` returns the same JSON bundle as `get_resource_context`.
It also exposes `prompts/list` and `prompts/get` from the manifest-owned
`workflowPrompts` catalogue. Those prompts are workflow hints over the
manifest tools and resource URIs, not a separate MCP playbook layer.

## Install

Three ways to get the binary, in order of friction.

### 1. One-line installer (recommended)

```sh
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh | bash
```

Detects your platform, downloads the matching binary from the latest
Pulse release, verifies SHA256, and places it at `~/.local/bin/pulse-mcp`
(or `/usr/local/bin/pulse-mcp` if `~/.local/bin` is not writable).
Override the install location with `PULSE_MCP_BIN_DIR=/some/path` or
the version with `PULSE_MCP_VERSION=v6.0.0-rc.5`.

Windows (PowerShell):

```powershell
irm https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.ps1 | iex
```

### 2. Download from a GitHub Release

Pick the binary that matches your platform from the Pulse release assets:

- `pulse-mcp-darwin-arm64`, `pulse-mcp-darwin-amd64` for macOS
- `pulse-mcp-linux-amd64`, `pulse-mcp-linux-arm64`, `pulse-mcp-linux-armv7` for Linux
- `pulse-mcp-windows-amd64.exe` for Windows
- (full matrix on the release page)

`chmod +x` it on Unix, drop it on your `PATH`. SHA256 sums for every
binary are in `checksums.txt` on the same release.

On macOS the first launch may show a Gatekeeper warning ("cannot be
opened because the developer cannot be verified"). Either right-click
the binary and pick Open the first time, or run
`xattr -d com.apple.quarantine pulse-mcp` to clear the quarantine flag.
Notarization is intentionally skipped for v1; the install-script path
above downloads the same unsigned binary.

### 3. Build from source

If you have Go installed:

```sh
go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-mcp@latest
```

Or from a Pulse repo checkout:

```sh
go build -o pulse-mcp ./cmd/pulse-mcp
```

Drop the binary somewhere on your `PATH` (or reference its full path in
the config snippets below).

## Quick start

### 1. Get the binary

Use any of the install paths above. The rest of this guide assumes
`pulse-mcp` is on your `PATH`.

### 2. Mint an API token

Pulse fetches the live capabilities manifest first, and every tool
description includes its required auth scope. For a read-only external
agent, start with `monitoring:read`. For the full published Pulse
Intelligence surface, mint a token with the current manifest scopes:
<!-- pulse-mcp-scope-list:start -->
`monitoring:read`, `monitoring:write`, `settings:read`, `settings:write`, and `ai:execute`
<!-- pulse-mcp-scope-list:end -->

You can also mint narrower tokens for specific workflows. For example,
omit `settings:write` if the client should not add or edit monitored
infrastructure sources, and omit `ai:execute` if it should not review
Patrol findings or plan, approve, or execute governed actions. Mint the
token in **Settings →
Security → API Tokens**.

### 3. Wire it into your client

<!-- pulse-mcp-client-config:start -->
Most MCP clients need the same manifest-owned runtime facts: server name `pulse`, command `pulse-mcp`, base URL flag `--base-url`, default URL `http://localhost:7655`, and token environment variable `PULSE_API_TOKEN`.
The generated examples below cover the currently declared config families: `OpenCode`, `Claude-style clients`, and `custom MCP clients`.
If your client uses a different outer config format, keep those runtime facts and adapt only the wrapper.

#### OpenCode

Uses OpenCode's top-level mcp object.
Add this to `opencode.json`, `opencode.jsonc`, and `~/.config/opencode/opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "pulse": {
      "type": "local",
      "command": ["pulse-mcp", "--base-url", "http://localhost:7655"],
      "enabled": true,
      "environment": {
        "PULSE_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

Restart OpenCode or reload its config after saving. Pulse tools appear under the `pulse` MCP server name.

#### Claude-style clients

Uses the common mcpServers object supported by Claude Desktop and Claude Code.
Use this shape for `Claude Desktop` and `Claude Code` in `~/Library/Application Support/Claude/claude_desktop_config.json` and `.mcp.json`:

```json
{
  "mcpServers": {
    "pulse": {
      "command": "pulse-mcp",
      "args": ["--base-url", "http://localhost:7655"],
      "env": {
        "PULSE_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

Restart your client after saving the config. If the client cannot resolve `pulse-mcp` from `PATH`, use the full binary path.

#### custom MCP clients

Keeps the Pulse MCP command, base URL flag, and token environment variable while adapting the outer client config shape.
Use command `pulse-mcp`, pass `--base-url http://localhost:7655`, set `PULSE_API_TOKEN` to the API token, and keep the server name `pulse` when the client asks for one.
<!-- pulse-mcp-client-config:end -->

## Configuration

| Flag | Default | Purpose |
| ---- | ------- | ------- |
| `--base-url` | `http://localhost:7655` | Pulse instance to talk to |
| `--token-env` | `PULSE_API_TOKEN` | Env var holding the API token |
| `--emit-notifications` | `false` | Translate Pulse SSE events into JSON-RPC notifications on stdout |

The token is always read from an environment variable, never a flag, so
it does not appear in process listings.

### About `--emit-notifications`

When enabled, the server opens a long-lived connection to
`/api/agent/events` after `initialize` and writes a JSON-RPC
notification on stdout for every non-transport event:

```json
{ "jsonrpc": "2.0", "method": "notifications/finding.created", "params": { ... } }
{ "jsonrpc": "2.0", "method": "notifications/approval.pending",  "params": { ... } }
{ "jsonrpc": "2.0", "method": "notifications/action.completed",  "params": { ... } }
```

The `params` object is the SSE event's `data` payload verbatim, so an
agent that already knows the substrate's wire shape sees identical
content to what an HTTP SSE consumer would. Transport plumbing
(`stream.connected`, `heartbeat`) is filtered out.

It is off by default because not every MCP client surfaces
server-initiated notifications. Enable it when wiring an autonomous
agent that processes the JSON-RPC stream programmatically. Claude
Desktop today does not surface arbitrary `notifications/*` methods
in the chat UI; if your client falls in that category, leave the
flag off and consume the SSE stream directly.

When `--emit-notifications` is on, the `initialize` response
advertises the supported event kinds under
`capabilities.experimental.pulseNotifications.kinds`. That list is
projected from the shared Pulse Intelligence event vocabulary, so it
stays aligned with `/api/agent/events`. Clients that don't understand
the experimental block ignore it silently.

## What the tools do

The exact request/response tool set is whatever your Pulse instance's
manifest declares. The generated inventory below reflects the canonical
manifest in this checkout; run `tools/list` from your MCP client to see
the live set.

<!-- pulse-mcp-tools:start -->
**Context (read-only):**

- `get_resource_context` (Get resource context, `GET /api/agent/resource-context/{resourceId}`, scope `monitoring:read`, mode `read`, approval `scope_only`): Return the situated picture of a resource — identity, operator-set state with maintenance-window-active flag, active findings, pending approvals scoped to this resource, recent actions including refused dispatches. Command fields are redacted for monitoring-read API tokens unless the token also has ai:execute.
- `list_resource_capabilities` (List resource capabilities, `GET /api/agent/resource-capabilities/{resourceId}`, scope `monitoring:read`, mode `read`, approval `scope_only`): Return the structured governed capabilities a resource advertises (name, type, approval level, platform, and full parameter schemas). Companion to get_resource_context, which renders capabilities as count-limited prose; this is the structured surface an agent reads before calling plan_action so it can populate capabilityName and params without guessing. A resource with no advertised capabilities returns an empty array.
- `get_fleet_context` (Get fleet context, `GET /api/agent/fleet-context`, scope `monitoring:read`, mode `read`, approval `scope_only`): Return a thin per-resource triage rollup across every resource visible to the org — identity, operator flags (intentionallyOffline, neverAutoRemediate, maintenanceWindowActive), per-severity finding counts (total/critical/warning/info), and pending-approval count. One read for 'where do I focus?'; follow up via get_resource_context for depth. Optional additive filters (hasFindings, severity, technology, resourceType) narrow the result to a relevant subset so agents triaging a large fleet do not receive or page through healthy resources.
- `get_patrol_control_status` (Get Patrol work status, `GET /api/agent/patrol-control/status`, scope `monitoring:read`, mode `read`, approval `scope_only`): Return the current content-safe Patrol work status: Patrol issue evidence, pending approvals, governed decisions/actions, verified outcomes, Patrol control outcome evidence, compatibility aliases, and optional MCP readiness. The payload is count-only and deliberately omits finding ids, action ids, prompts, commands, resource names, and output.

**Operator state (per-resource intent):**

- `get_operator_state` (Get operator state, `GET /api/resources/{resourceId}/operator-state`, scope `monitoring:read`, mode `read`, approval `scope_only`): Read the operator-set state for a resource (intentionally offline, never auto-remediate, maintenance window, criticality).
- `set_operator_state` (Set operator state, `PUT /api/resources/{resourceId}/operator-state`, scope `monitoring:write`, mode `write`, approval `scope_only`): Replace the operator-set state for a resource. URL canonicalId wins over body; server populates setAt and setBy from the authenticated identity.
- `clear_operator_state` (Clear operator state, `DELETE /api/resources/{resourceId}/operator-state`, scope `monitoring:write`, mode `write`, approval `scope_only`): Remove any operator-set state for a resource. Idempotent — succeeds whether or not an entry was present.

**Findings (Patrol lifecycle):**

- `list_findings` (List findings, `GET /api/ai/patrol/findings`, scope `ai:execute`, mode `read`, approval `scope_only`): List all Patrol findings (active, dismissed, resolved). Filter client-side on returned shape.
- `acknowledge_finding` (Acknowledge finding, `POST /api/ai/patrol/acknowledge`, scope `ai:execute`, mode `write`, approval `scope_only`): Mark a finding as seen but keep it visible. Auto-resolves when the underlying condition clears.
- `snooze_finding` (Snooze finding, `POST /api/ai/patrol/snooze`, scope `ai:execute`, mode `write`, approval `scope_only`): Hide a finding for a defined duration in hours.
- `dismiss_finding` (Dismiss finding, `POST /api/ai/patrol/dismiss`, scope `ai:execute`, mode `write`, approval `scope_only`): Dismiss a finding with a reason: not_an_issue (permanent suppression), expected_behavior (acknowledged forever), or will_fix_later (7-day reminder commitment).
- `resolve_finding` (Resolve finding, `POST /api/ai/patrol/resolve`, scope `ai:execute`, mode `write`, approval `scope_only`): Manually mark a finding as resolved when the underlying issue has been fixed out-of-band.

**Actions (governed plan/approval/execute):**

- `plan_action` (Plan action, `POST /api/actions/plan`, scope `ai:execute`, mode `write`, approval `action_plan`): Plan an action against a resource. The planner validates the request, looks up the capability on the resource, checks executor-owned live availability, and returns an ActionPlan with the approval policy, blast radius, plan hash, and preflight summary. The plan is persisted to the audit history at the planned/pending state only after the live availability check passes, so subsequent decide_action and execute_action calls can reference it by id. Plan-and-execute is a two-step flow when the resulting plan requires approval, one-step otherwise.
- `decide_action` (Decide action, `POST /api/actions/{actionId}/decision`, scope `ai:execute`, mode `write`, approval `action_plan`): Record an approval decision (approved or rejected) on a previously planned action. The actor is taken from the authenticated identity; an explicit reason can be passed in the body. Idempotent on the persisted decision: re-deciding a non-pending action surfaces the action_not_pending stable code so agents can branch on the conflict rather than retrying blindly.
- `execute_action` (Execute action, `POST /api/actions/{actionId}/execute`, scope `ai:execute`, mode `write`, approval `action_plan`): Execute a previously planned and (when required) approved action. Returns the persisted audit record with the execution result attached. Refuses with stable codes when the action is in the wrong lifecycle state (action_not_approved, action_already_executing, action_execution_final, action_dry_run_only, action_plan_expired), when the approved plan no longer matches the current resource/capability contract (action_plan_drift), when the target is operator-locked against automated remediation (resource_remediation_locked), or when the API instance has no executor wired (action_executor_unavailable). action.completed SSE events fire on every terminal state so agents watching the stream do not need to poll this endpoint after dispatch.

**Provisioning (infrastructure onboarding):**

- `list_nodes` (List nodes, `GET /api/config/nodes`, scope `settings:read`, mode `read`, approval `scope_only`): List configured infrastructure sources that Pulse can monitor or manage. Credential secret values are redacted; use the returned id with update_node, remove_node, test_node_connection, or refresh_node_cluster_membership.
- `add_node` (Add node, `POST /api/config/nodes`, scope `settings:write`, mode `write`, approval `scope_only`): Add a Proxmox VE, Proxmox Backup Server, or Proxmox Mail Gateway source to Pulse after credentials have been collected, generated, or approved.
- `update_node` (Update node, `PUT /api/config/nodes/{nodeId}`, scope `settings:write`, mode `write`, approval `scope_only`): Update a configured infrastructure source. Omitted fields preserve the current value; tokenValue or password only changes when supplied.
- `remove_node` (Remove node, `DELETE /api/config/nodes/{nodeId}`, scope `settings:write`, mode `write`, approval `scope_only`): Remove a configured infrastructure source from Pulse by node id.
- `test_node_credentials` (Test node credentials, `POST /api/config/nodes/test-config`, scope `settings:write`, mode `mixed`, approval `scope_only`): Validate proposed source credentials and connection details without saving them to Pulse.
- `test_node_connection` (Test node connection, `POST /api/config/nodes/{nodeId}/test`, scope `settings:write`, mode `mixed`, approval `scope_only`): Validate the saved connection for an existing configured infrastructure source.
- `refresh_node_cluster_membership` (Refresh node cluster membership, `POST /api/config/nodes/{nodeId}/refresh-cluster`, scope `settings:write`, mode `write`, approval `scope_only`): Re-detect Proxmox VE cluster membership and endpoint metadata for a configured source.
- `discover_lan` (Discover LAN, `POST /api/discover`, scope `settings:write`, mode `mixed`, approval `scope_only`): Scan a subnet, or return cached scan results, to find candidate infrastructure hosts before deciding which sources to add to Pulse.
<!-- pulse-mcp-tools:end -->

## Workflow prompts

Clients that support MCP prompts can also discover Pulse workflow hints.
They are projected from the manifest-owned `workflowPrompts` catalogue:

<!-- pulse-mcp-prompts:start -->
- `pulse_triage_fleet` (Triage fleet): Triage the Pulse fleet using the canonical fleet context capability, then choose where deeper investigation is warranted.
- `pulse_operations_loop` (Ask Patrol to handle an issue): Have Patrol investigate active findings, follow the configured Patrol mode, take approved actions, verify the outcome, and record what happened.
- `pulse_investigate_resource` (Investigate resource; required argument `resourceId`): Investigate one Pulse resource using the canonical resource context capability and resource URI projection.
- `pulse_review_finding` (Review finding; required argument `finding_id`): Review one Patrol finding and propose the safest governed next step.
<!-- pulse-mcp-prompts:end -->

Run `prompts/list` from your MCP client to see the live prompt set.

## Resource browser

Clients that support MCP resources can call `resources/list` to browse
the same canonical resource IDs returned by `get_fleet_context`, then
`resources/read` to fetch the full `get_resource_context` JSON bundle
for one resource.

Example URI:

```text
pulse://resource/vm:101
```

The adapter does not maintain a separate resource registry. The resource
list is rebuilt through the live fleet-context capability, and each read
is a normal per-resource context capability call with the same auth scope,
redaction rules, and error envelope as the tool surface.

## Stable error envelope

Every Pulse error reaches your agent verbatim, in this shape:

```json
{ "error": "<stable_code>", "message": "<human readable>" }
```

The MCP tool result wraps that JSON in a text content block with
`isError: true`. Branch on `error` (snake_case stable codes); use
`message` for surfacing to humans, never for branching.

Capability-specific stable codes are advertised by the manifest:

<!-- pulse-mcp-errors:start -->
- `get_resource_context`: `resource_not_found`
- `list_resource_capabilities`: `resource_not_found`
- `get_operator_state`: `operator_state_not_set`
- `set_operator_state`: `operator_state_invalid`
- `acknowledge_finding`: `invalid_finding_request`, `finding_not_found`, `finding_action_not_allowed`, and `patrol_unavailable`
- `snooze_finding`: `invalid_finding_request`, `finding_not_found`, `finding_action_not_allowed`, and `patrol_unavailable`
- `dismiss_finding`: `invalid_finding_request`, `finding_not_found`, `finding_action_not_allowed`, and `patrol_unavailable`
- `resolve_finding`: `invalid_finding_request`, `finding_not_found`, `finding_action_not_allowed`, and `patrol_unavailable`
- `plan_action`: `invalid_action_request`, `resource_not_found`, `capability_not_found`, and `action_execution_unavailable`
- `decide_action`: `missing_id`, `invalid_id`, `invalid_action_decision`, `action_not_found`, `action_not_pending`, and `action_plan_expired`
- `execute_action`: `missing_id`, `invalid_id`, `invalid_action_execution`, `action_not_found`, `action_not_approved`, `action_already_executing`, `action_execution_final`, `action_dry_run_only`, `action_plan_expired`, `action_plan_drift`, `resource_remediation_locked`, and `action_executor_unavailable`
<!-- pulse-mcp-errors:end -->

Cross-cutting codes from the auth / multi-tenant middleware
(`invalid_org`, `org_suspended`, `access_denied`) can apply to any
authenticated tool call.

## Known limitations

- **No `subscribe_events` tool.** SSE streaming does not fit the MCP
  request/response tool shape, so the adapter does not expose the
  `/api/agent/events` stream as a callable tool. Agents that want
  real-time push have two options: consume the SSE stream directly
  via HTTP (works with any MCP client), or run with
  `--emit-notifications` so the bridge translates SSE events into
  JSON-RPC notifications on the stdio channel (requires a client
  that processes server-initiated notifications).

- **Manifest is fetched once.** The server fetches `/api/agent/
  capabilities` at startup and does not refresh during the process
  lifetime. Restart `pulse-mcp` to pick up new capabilities after
  upgrading Pulse.

## Troubleshooting

**"env var PULSE_API_TOKEN is empty" on startup.**
The adapter refuses to start without a token. Mint one in Settings and
make sure your client's `env` block (or shell environment) sets
`PULSE_API_TOKEN`.

**"manifest GET returned 401" on startup.**
Discovery is supposed to be unauthenticated. If your Pulse instance is
behind a reverse proxy that adds auth in front of the public paths,
the proxy is gating the manifest endpoint. Make sure
`/api/agent/capabilities` is reachable without a credential, the same
way `/api/health` is.

**Tools work, but a write or action tool returns 403 access_denied.**
Your token is missing that tool's required scope. Run `tools/list` and
check the `required scope` line in the tool description. Operator-state
writes need `monitoring:write`, provisioning tools need `settings:read`
or `settings:write`, and Patrol finding plus governed action tools need
`ai:execute`. Mint a narrower or broader token depending on which
external-agent workflows you want to allow.

**Tools list is empty.**
The adapter filters `subscribe_events` out (it is not request/response
shaped). If literally nothing else shows up, your Pulse instance's
manifest is empty, which is a Pulse-side bug; check
`curl http://your-pulse/api/agent/capabilities` directly.

## Implementation notes

The adapter binary is intentionally thin. It speaks JSON-RPC 2.0 over
stdio with line-delimited framing, logs to stderr to keep the JSON-RPC
channel on stdout clean, and delegates method semantics, tool projection,
call-argument normalization, HTTP projection, and result envelopes to the
shared Pulse Intelligence contracts in `internal/agentcapabilities`.
Tool input schemas come from the manifest-owned JSON Schema when present;
the shared projection only falls back to deriving path placeholders and a
permissive body object for capabilities that have not yet authored a
stricter schema.

The companion worked example, `cmd/agent-probe`, walks the same
substrate as a plain HTTP client and is a useful reference for anyone
building a non-MCP integration. Together the two binaries demonstrate
Pulse Intelligence's external-agent profiles: stdio MCP and direct HTTP
API. Pulse Assistant remains the first-party in-app surface over the
same governed contracts; `pulse-mcp` is the adapter for users who prefer
an external agent host.
