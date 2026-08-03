# Pulse agent substrate

A short, plain-English summary of what landed across the agent-paradigm
arc on `pulse/v6-release`. Suitable as the basis for release notes, a
GitHub announcement, or just a reminder to yourself in three weeks of
what shape this work took.

## What it is

Pulse v6 ships an agent-paradigm substrate so external agents (Claude
Desktop, Claude Code, OpenCode, other MCP clients, plain HTTP consumers) can
drive Pulse with the same context an in-process Patrol or Assistant
has. The substrate has four axes:

**Discovery.** A canonical manifest at `/api/agent/capabilities`
lists every agent-consumable capability with its name, description,
HTTP method and path, required auth scope, response shape, stable
error codes, and the deduplicated `requiredScopes` summary for the
current full surface. It also carries the Pulse Intelligence Core,
Patrol, Assistant, and MCP surface contract, including which
affordances each supported operator surface exposes. The manifest is
unauthenticated so an agent without a token can introspect Pulse before
asking for one.

**Depth.** `/api/agent/resource-context/{id}` returns the situated
picture of one resource in a single read: identity, operator-set
state, active findings, pending approvals, recent actions including
refused dispatches and verification probe outcomes. Stable token
prefixes (`plan_drift:`, `resource_remediation_locked:`) reach the
wire verbatim so agents branch on codes, not human text.

**Breadth.** `/api/agent/fleet-context` returns a thin per-resource
rollup across the whole org: identity, operator flags, per-severity
finding counts, pending-approval count. One read for "where do I
focus?", with the per-resource bundle for follow-up depth.

**Write.** Two write surfaces. The operator-state intent loop
(`/api/resources/{id}/operator-state`) lets an agent record
per-resource commitments (intentionally offline, never
auto-remediate, maintenance window, criticality). The action
governance loop (`/api/actions/plan`, `/api/actions/{id}/decision`,
`/api/actions/{id}/execute`) lets an agent plan, approve, and
execute capability invocations against a resource through the
canonical audit store. The server populates attribution so client
values cannot spoof who-did-it. Validation failures emit the
`operator_state_invalid` and `invalid_action_request` stable
codes; lifecycle conflicts on the action loop emit
`action_not_pending`, `action_not_approved`,
`action_already_executing`, `action_execution_final`, and
`action_dry_run_only` so agents branch on the conflict rather
than retrying blindly.

**Push.** `/api/agent/events` is an SSE stream that fires
`finding.created`, `approval.pending`, and `action.completed` events
as state changes. Each event is a small fixed-shape payload with
enough context for an agent to decide whether to follow up. Refused
dispatches preserve their stable error tokens; successful dispatches
carry a verification block so agents close the certainty loop without
polling the audit endpoint.

## What ships consuming it

Three first-party consumers are built on the same manifest:

- **Settings -> API Access -> Agent integrations** is the in-app
  operator surface. It fetches `/api/agent/capabilities` from the
  running instance, lists the declared capabilities by category, shows
  the manifest-owned surface contract and affordance badges, shows each
  capability's method, path, scope, and stable error codes, and
  generates client-ready `pulse-mcp` config snippets from the
  manifest-owned MCP adapter setup contract: server name, command,
  base URL flag, token environment variable, and the supported client config families.
  The panel fills in the current Pulse URL for OpenCode's native `opencode.json` / `mcp` shape
  and the common `mcpServers` shape for Claude-style clients.
  Tokens are still minted in API Access, so the same settings tab covers
  "what agents can do" and "which token unlocks it."

- **`cmd/agent-probe`** is a small Go binary that walks the
  discovery, triage, depth, push flow against a running Pulse
  instance. Useful as a smoke test or worked example for someone
  building their own integration.

- **`cmd/pulse-mcp`** is the MCP server adapter. Wire it into any
  MCP client that can launch a local server; the README at
  `cmd/pulse-mcp/README.md` includes generated setup plus OpenCode,
  Claude Desktop, and Claude Code examples from the same adapter contract,
  and the in-app Agent integrations panel shows both
  OpenCode's native `opencode.json` shape and the common `mcpServers`
  block. The adapter projects each manifest capability into one MCP
  tool with auto-derived input schema; adding capabilities to Pulse
  extends the MCP surface without changes in the adapter. Run with
  `--emit-notifications` to also translate Pulse's SSE events (`finding.created`,
  `approval.pending`, `action.completed`) into JSON-RPC
  notifications on the stdio channel so autonomous MCP-bound agents
  can react to push events without holding a separate HTTP
  connection.

`pulse-mcp` also has a published distribution path: the one-line
installers (`install-mcp.sh` and `install-mcp.ps1`) download the
matching binary from the latest Pulse GitHub Release and verify the
release checksum. Building from source remains available for local
development.

## What it does not do yet

- Real-world consumer feedback. The substrate ships with the in-app
  Agent integrations panel, two reference adapters (HTTP and MCP),
  release installers, and end-to-end contract tests, but no external
  integration has been load-bearing on it yet. The next meaningful
  work item is whatever friction first usage surfaces, not more
  substrate plumbing.

- macOS notarization and package-manager polish. The installer
  verifies release checksums, but the first launch of the unsigned
  macOS binary can still show a Gatekeeper warning. Homebrew or other
  package-manager distribution can sit on top of the release binary
  path when usage signal warrants the maintenance.

## Provable claims

- **Manifest is honest.** A contract pin
  (`TestContract_AgentSurfaceErrorCodesMatchManifestDeclarations`)
  parses every `writeJSONError` call from agent-surface handlers and
  every `ErrorCodes` declaration from the manifest, asserting
  symmetry both directions. Drift either way fails the test.

- **The substrate composes.** Two paired end-to-end tests in
  `internal/api/agent_substrate_e2e_test.go` boot the full router
  stack and walk discovery, triage, depth, and the operator-state
  write loop through the actual HTTP boundary. They are the
  substantive proof that the four axes work as one.

- **Discovery is unauthenticated.** Pinned by
  `TestContract_AgentCapabilitiesManifestIsPublic` after a slice 47
  fix added the path to `publicPaths`. Slice 40 had it 401'ing
  despite the docs.

- **Stable error envelope is two-layer.** Capability-specific codes
  (`resource_not_found`, `operator_state_not_set`,
  `operator_state_invalid`) are declared per-capability in the
  manifest. Cross-cutting codes (`invalid_org`, `org_suspended`,
  `access_denied`) come from the auth and multi-tenant middleware
  and apply to every authenticated endpoint. Documented in
  `api-contracts.md`; a contract test enforces no drift.

## Where to read more

- Full contract: `docs/release-control/v6/internal/subsystems/api-contracts.md`,
  agent-surface paragraphs in the `## Current State` section.
- Implementation: `internal/api/agent_*.go` and
  `internal/api/resources_operator_state.go`.
- In-app setup surface:
  `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`.
- MCP adapter: `cmd/pulse-mcp/` (with README).
- MCP installers: `scripts/install-mcp.sh` and
  `scripts/install-mcp.ps1`.
- HTTP worked example: `cmd/agent-probe/`.
- Subsystem dependencies: relevant paragraphs in `agent-lifecycle.md`,
  `performance-and-scalability.md`, and `storage-recovery.md` under
  `docs/release-control/v6/internal/subsystems/`.
