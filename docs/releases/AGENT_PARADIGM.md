# Pulse Agent Paradigm

_Release-notes-ready blurb for the agent-paradigm work that landed
across slices 39-59 on `pulse/v6-release`. Drop this into a release
announcement, GitHub prerelease description, or the body of a
`docs/releases/RELEASE_NOTES_v6_*.md` "What Changed" section.
Trim or expand as the cut requires; this file is the source draft._

## Headline

Pulse v6 ships an agent-paradigm substrate so external agents
(Claude Desktop, Claude Code, custom MCP clients, plain HTTP
consumers) can drive Pulse with the same context an in-process
Patrol or Assistant has.

## What an operator gets

- **Discoverable from Pulse itself.** Settings → API Access now
  has an Agent Integrations section listing every capability the
  running instance declares, grouped by category (Context,
  Operator state, Patrol findings, Action governance), with
  scope, method, path, and the stable error codes each emits.
  The section also generates a Claude Desktop / Claude Code
  config snippet pre-filled with the deployment's own URL, so
  wiring an agent is copy, paste, and add a token.

- **Drivable from Claude in one command.** A new `pulse-mcp`
  server adapter ships in the Pulse repo (`cmd/pulse-mcp/`)
  with a published distribution path. Install from the Pulse
  GitHub Release using the one-line installer:
  `curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh | bash`
  (or the matching `install-mcp.ps1` PowerShell installer on
  Windows). Wire it into Claude Desktop or Claude Code per
  [`cmd/pulse-mcp/README.md`](../../cmd/pulse-mcp/README.md) and
  Pulse's tools appear natively. Each MCP tool is one entry in
  the canonical capabilities manifest; adding a capability on
  the backend extends the MCP surface automatically. An optional
  `--emit-notifications` flag translates Pulse's SSE event
  stream (`finding.created`, `approval.pending`,
  `action.completed`) into JSON-RPC notifications so autonomous
  agents react to push events without holding a separate HTTP
  connection.

- **Contracts that hold across the surface.** The agent surface
  uses one stable error envelope
  (`{"error": "<code>", "message": "<human>", "details"?: {...}}`)
  across read, write, push, and action-governance capabilities.
  Stable error codes (`resource_not_found`,
  `operator_state_invalid`, `action_dry_run_only`,
  `resource_remediation_locked:`, etc.) reach the wire verbatim
  so agents branch on codes rather than parsing human messages.
  Two end-to-end tests exercise discovery → triage → depth and
  the operator-state write loop through the actual HTTP
  boundary; a third walks the action surface.

## Four axes

- **Discovery.** `/api/agent/capabilities` is a hand-authored,
  unauthenticated manifest agents fetch at startup to learn
  what's available.
- **Read.** `/api/agent/resource-context/{id}` returns the
  situated picture of one resource in a single read;
  `/api/agent/fleet-context` returns a thin per-resource rollup
  across the org for triage.
- **Write.** Two write surfaces. The operator-state intent loop
  (`/api/resources/{id}/operator-state`) records per-resource
  commitments (intentionally offline, never auto-remediate,
  maintenance window). The action governance loop
  (`/api/actions/plan`, `/api/actions/{id}/decision`,
  `/api/actions/{id}/execute`) plans, approves, and executes
  capability invocations through the canonical audit store.
- **Push.** `/api/agent/events` is an SSE stream that fires
  `finding.created`, `approval.pending`, and `action.completed`
  events as state changes. `action.completed` carries a
  verification block from the broker's read-after-write probe
  so agents close the "did it actually work?" loop without
  polling the audit endpoint.

## For integrators

- **HTTP example:** [`cmd/agent-probe`](../../cmd/agent-probe/).
  A small standard-library Go program that walks discovery →
  triage → depth → push against a running Pulse instance.
  Reads as a worked example for anyone building a custom
  integration in any language.
- **MCP server:** [`cmd/pulse-mcp`](../../cmd/pulse-mcp/). Stdio
  JSON-RPC adapter for Claude Desktop and Claude Code. Mint a
  Pulse API token with `monitoring:read` (and
  `monitoring:write` for operator-state writes), set
  `PULSE_API_TOKEN` in the client's env block, and Pulse's
  capabilities appear as MCP tools.
- **Contract reference:**
  [`docs/AGENT_SUBSTRATE.md`](../AGENT_SUBSTRATE.md) is the
  in-repo session marker. The full subsystem contract lives in
  `docs/release-control/v6/internal/subsystems/api-contracts.md`
  under the agent-surface paragraphs in `## Current State`.

## What it does not do yet

- **No notarization on macOS.** The first launch of
  `pulse-mcp` on macOS shows a Gatekeeper warning. The
  README documents the right-click-Open or
  `xattr -d com.apple.quarantine` bypass; SHA256 verification
  is preserved through the installer. Notarization is a
  natural follow-up if usage signal points at this as a
  friction point.
- **No Homebrew tap or core formula.** The one-line installer
  fetches from GitHub Releases directly. Homebrew is a layer
  that can sit on top of this foundation when audience scale
  warrants the ongoing maintenance.
- **Real-world consumer feedback is the gating signal for
  what's next.** The substrate ships with end-to-end tests,
  two reference adapters, and a published distribution path,
  but no external integration has been load-bearing on it yet.
  The next meaningful work item is whatever friction first
  usage surfaces.

## Audit trail

- 16 commits on `pulse/v6-release` between slices 39 and 59
  cover the substrate, the two adapters, the e2e tests, the
  contract pins, the in-Pulse Settings panel, and the
  subsystem documentation.
- Contract pins enforce manifest honesty
  (`TestContract_AgentSurfaceErrorCodesMatchManifestDeclarations`),
  cross-org isolation
  (`TestPendingApprovalsForResource_FiltersByOrg`), the
  unauthenticated discovery posture
  (`TestContract_AgentCapabilitiesManifestIsPublic`), and the
  action-endpoint envelope migration
  (`TestAgentSubstrate_ActionEndpointsEmitAgentStableEnvelope`).
- The full Pulse test suite is green on the agent-substrate
  packages; flaky SLO tests in `internal/api/` and unrelated
  unraid drift in `internal/mock/` (from a separate
  platform-support work track) are the two known maintenance
  items not in this arc.
