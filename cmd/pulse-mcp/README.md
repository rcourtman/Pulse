# pulse-mcp

`pulse-mcp` is a Model Context Protocol (MCP) server that wraps Pulse's
agent surface as a tool set. It lets Claude Desktop, Claude Code, and any
other MCP-speaking client drive Pulse natively: list findings, read the
fleet, drill into a resource, set operator intent, run governed actions.

The adapter is manifest-driven. Every tool it exposes is one entry in
Pulse's hand-authored capabilities manifest at `/api/agent/capabilities`.
Adding a capability there extends this server automatically. There is no
hardcoded tool list to keep in sync.

## Quick start

### 1. Build

From the repo root:

```sh
go build -o pulse-mcp ./cmd/pulse-mcp
```

Drop the binary somewhere on your `PATH` (or reference its full path in
the config snippets below).

### 2. Mint an API token

Pulse needs a token with `monitoring:read` for the read tools, and
`monitoring:write` if you want the write tools (`set_operator_state`,
`clear_operator_state`) to work. Mint one in **Settings → Security → API
Tokens**.

### 3. Wire it into your client

#### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json`
(macOS) or the platform equivalent. Add a `pulse` entry under
`mcpServers`:

```json
{
  "mcpServers": {
    "pulse": {
      "command": "/usr/local/bin/pulse-mcp",
      "args": ["--base-url", "http://localhost:7655"],
      "env": {
        "PULSE_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

Restart Claude Desktop. The Pulse tools appear in the tool picker.

#### Claude Code

Add the server to your project's `.mcp.json` (or your user-level config):

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
`capabilities.experimental.pulseNotifications.kinds`. Clients that
don't understand the experimental block ignore it silently.

## What the tools do

The exact set is whatever your Pulse instance's manifest declares. As of
this writing, the published capabilities are:

**Context (read-only):**

- `get_resource_context` returns the situated picture of one resource:
  identity, operator-set state, active findings, pending approvals,
  recent actions including refused dispatches with their stable error
  tokens.
- `get_fleet_context` returns a thin per-resource rollup across the org
  for triage: identity, operator flags, per-severity finding counts,
  pending-approval count.

**Operator state (per-resource intent):**

- `get_operator_state` reads the operator-set state for a resource
  (intentionally offline, never-auto-remediate, maintenance window,
  criticality).
- `set_operator_state` replaces the entire record. The server populates
  attribution (`setAt`, `setBy`) so client values cannot spoof it.
- `clear_operator_state` removes the entry. Idempotent.

**Findings (Patrol lifecycle):**

- `list_findings` returns every Patrol finding (active, dismissed,
  resolved). Filter client-side.
- `acknowledge_finding` marks a finding as seen but keeps it visible.
- `snooze_finding` hides a finding for a duration in hours.
- `dismiss_finding` permanently dismisses a finding with a reason
  (`not_an_issue`, `expected_behavior`, `will_fix_later`).
- `resolve_finding` manually marks a finding resolved.

Run `tools/list` from your MCP client to see the live set.

## Stable error envelope

Every Pulse error reaches your agent verbatim, in this shape:

```json
{ "error": "<stable_code>", "message": "<human readable>" }
```

The MCP tool result wraps that JSON in a text content block with
`isError: true`. Branch on `error` (snake_case stable codes); use
`message` for surfacing to humans, never for branching.

Capability-specific codes the substrate currently emits include
`resource_not_found` (depth read on an unknown id),
`operator_state_not_set` (read on a resource with no operator entry), and
`operator_state_invalid` (write rejected by the validator). Cross-cutting
codes from the auth / multi-tenant middleware (`invalid_org`,
`org_suspended`, `access_denied`) can apply to any authenticated tool
call.

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

- **No resource URIs.** MCP supports a `resources/` channel in addition
  to `tools/`. The adapter exposes only tools today; this is sufficient
  for the substrate's current surface and keeps the adapter small.

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

**Tools work, but `set_operator_state` returns 403 access_denied.**
Your token is missing `monitoring:write`. Either mint a new token with
both scopes, or restrict yourself to the read tools.

**Tools list is empty.**
The adapter filters `subscribe_events` out (it is not request/response
shaped). If literally nothing else shows up, your Pulse instance's
manifest is empty, which is a Pulse-side bug; check
`curl http://your-pulse/api/agent/capabilities` directly.

## Implementation notes

The adapter is one stdlib-only Go file (`main.go`) in around 430 lines.
It speaks JSON-RPC 2.0 over stdio with line-delimited framing, logs to
stderr to keep the JSON-RPC channel on stdout clean, and derives each
tool's input schema from the manifest entry: `{name}` segments in the
declared path become required string properties, and non-GET/DELETE
tools accept a free-form `body` object.

The companion worked example, `cmd/agent-probe`, walks the same
substrate as a plain HTTP client and is a useful reference for anyone
building a non-MCP integration. Together the two binaries demonstrate
the substrate's two consumer profiles: stdio MCP and HTTP API. Both are
manifest-driven and stdlib-only, so adding capabilities to Pulse extends
both without code changes here.
