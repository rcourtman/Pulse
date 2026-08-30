# Pulse agent integrations

Pulse exposes a set of HTTP endpoints so an external agent can read your
infrastructure and act on it with the same context Pulse Patrol and the Pulse
Assistant have. Claude Desktop, Claude Code, OpenCode, other MCP clients, and
plain HTTP consumers can all drive it.

This page explains what those endpoints offer and what ships to help you
connect to them. To generate a ready-made client configuration for your own
instance, open Settings, then API Access, then Agent integrations.

## What the endpoints offer

**Discovery.** A manifest at `/api/agent/capabilities` lists every
agent-consumable capability with its name, description, HTTP method and path,
required auth scope, response shape, stable error codes, and a deduplicated
`requiredScopes` summary for the whole surface. It also carries the Pulse
Intelligence Core, Patrol, Assistant, and MCP surface contract, including
which affordances each supported operator surface exposes. The manifest needs
no token, so an agent can introspect Pulse before you issue it credentials.

**Depth.** `/api/agent/resource-context/{id}` returns the situated picture of
one resource in a single read. That covers identity, operator-set state,
active findings, pending approvals, and recent actions including refused
dispatches and verification probe outcomes. Stable token prefixes such as
`plan_drift:` and `resource_remediation_locked:` reach the wire verbatim, so
an agent can branch on codes rather than on human-readable text.

**Breadth.** `/api/agent/fleet-context` returns a thin per-resource rollup
across the whole organisation, covering identity, operator flags, per-severity
finding counts, and pending-approval count. It answers "where do I focus" in
one read, with the per-resource endpoint available for follow-up depth.

**Write.** There are two write surfaces. The operator-state intent loop
(`/api/resources/{id}/operator-state`) records per-resource commitments such
as intentionally offline, never auto-remediate, maintenance window, and
criticality. The action governance loop (`/api/actions/plan`,
`/api/actions/{id}/decision`, `/api/actions/{id}/execute`) plans, approves,
and executes capability invocations against a resource through the canonical
audit store. Pulse populates attribution itself, so a client cannot spoof who
did something. Validation failures emit the `operator_state_invalid` and
`invalid_action_request` codes. Lifecycle conflicts on the action loop emit
`action_not_pending`, `action_not_approved`, `action_already_executing`,
`action_execution_final`, and `action_dry_run_only`, so an agent can branch on
the specific conflict instead of retrying blindly.

**Push.** `/api/agent/events` is an SSE stream that fires `finding.created`,
`approval.pending`, and `action.completed` as state changes. Each event is a
small fixed-shape payload carrying enough context for an agent to decide
whether to follow up. Refused dispatches keep their stable error tokens, and
successful dispatches carry a verification block, so an agent can confirm an
outcome without polling the audit endpoint.

## What ships to help you connect

- **Settings, then API Access, then Agent integrations** is the in-app
  surface. It reads `/api/agent/capabilities` from your running instance,
  lists the declared capabilities by category, shows the surface contract and
  affordance badges, and shows each capability's method, path, scope, and
  stable error codes. It also generates client-ready `pulse-mcp` configuration
  snippets with your instance's URL already filled in, covering OpenCode's
  native `opencode.json` shape and the `mcpServers` shape used by
  Claude-style clients. API tokens are minted on the same tab, so one place
  covers both what agents can do and which token unlocks it.

- **`cmd/pulse-mcp`** is the MCP server adapter. Wire it into any MCP client
  that can launch a local server. It projects each manifest capability into
  one MCP tool with an auto-derived input schema, so capabilities added to
  Pulse extend the MCP surface without an adapter change. Run it with
  `--emit-notifications` to translate Pulse's SSE events into JSON-RPC
  notifications on the stdio channel, which lets an autonomous MCP-bound agent
  react to push events without holding a separate HTTP connection. The
  one-line installers `install-mcp.sh` and `install-mcp.ps1` fetch the
  matching binary from the latest Pulse release, verify the checksum manifest
  against Pulse's pinned release key, and then verify the binary's checksum.
  They refuse installation when any integrity evidence is unavailable or
  invalid. Building from source stays available.

- **`cmd/agent-probe`** is a small Go binary that walks the discovery,
  triage, depth, and push flow against a running instance. Use it as a smoke
  test, or as a worked example if you are building your own HTTP integration.

## Guarantees

- **The manifest matches the implementation.** Every error code an
  agent-surface handler can emit is declared in the manifest, and every code
  the manifest declares is one a handler can emit. Drift in either direction
  fails the build, so the manifest can be trusted as the contract.

- **Discovery needs no token.** `/api/agent/capabilities` serves without
  credentials by design. The capabilities it describes keep their own auth
  scopes, so introspection does not grant access to anything.

- **Error codes come in two layers.** Capability-specific codes such as
  `resource_not_found`, `operator_state_not_set`, and
  `operator_state_invalid` are declared per capability in the manifest.
  Cross-cutting codes such as `invalid_org`, `org_suspended`, and
  `access_denied` come from the auth and multi-tenant middleware and apply to
  every authenticated endpoint.

- **The surfaces compose.** Discovery, triage, depth, and the operator-state
  write loop are exercised together through the real HTTP boundary on every
  build, rather than only in isolation.

## Known rough edges

- **Unsigned macOS binary.** The installer verifies release checksums, but the
  first launch of the macOS `pulse-mcp` binary can still show a Gatekeeper
  warning because the binary is not notarised. Homebrew and other
  package-manager distribution may follow.

- **Limited field usage.** These endpoints ship with the in-app panel, two
  reference adapters, release installers, and end-to-end contract tests, but
  no external integration has been load-bearing on them yet. If something is
  awkward in practice, open an issue, because that feedback is what shapes
  what comes next.

## Where to read more

- [Configuration](CONFIGURATION.md) for API tokens and their scopes.
- [API reference](API.md) for the wider Pulse HTTP surface.
- [AI features](AI.md) for how Patrol and the Assistant use the same context.
- `cmd/pulse-mcp/README.md` in the repository for adapter setup examples
  covering OpenCode, Claude Desktop, and Claude Code.
