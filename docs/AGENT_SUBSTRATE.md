# Pulse agent substrate

A short, plain-English summary of what landed across the agent-paradigm
arc on `pulse/v6-release`. Suitable as the basis for release notes, a
GitHub announcement, or just a reminder to yourself in three weeks of
what shape this work took.

## What it is

Pulse v6 ships an agent-paradigm substrate so external agents (Claude
Desktop, Claude Code, custom MCP clients, plain HTTP consumers) can
drive Pulse with the same context an in-process Patrol or Assistant
has. The substrate has four axes:

**Discovery.** A hand-authored manifest at `/api/agent/capabilities`
lists every agent-consumable capability with its name, description,
HTTP method and path, required auth scope, response shape, and stable
error codes. The manifest is unauthenticated so an agent without a
token can introspect Pulse before asking for one.

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

**Write.** The operator-state intent loop
(`/api/resources/{id}/operator-state`) lets an agent record
per-resource commitments (intentionally offline, never auto-remediate,
maintenance window, criticality). The server populates attribution so
client values cannot spoof who-did-it. Validation failures emit the
`operator_state_invalid` stable code; reads on unset resources emit
`operator_state_not_set`.

**Push.** `/api/agent/events` is an SSE stream that fires
`finding.created`, `approval.pending`, and `action.completed` events
as state changes. Each event is a small fixed-shape payload with
enough context for an agent to decide whether to follow up. Refused
dispatches preserve their stable error tokens; successful dispatches
carry a verification block so agents close the certainty loop without
polling the audit endpoint.

## What ships consuming it

Two reference consumers, both standard-library-only, both
manifest-driven:

- **`cmd/agent-probe`** is a small Go binary that walks the
  discovery, triage, depth, push flow against a running Pulse
  instance. Useful as a smoke test or worked example for someone
  building their own integration.

- **`cmd/pulse-mcp`** is the MCP server adapter. Wire it into Claude
  Desktop or Claude Code per the README at `cmd/pulse-mcp/README.md`
  and Pulse's tools appear natively. The adapter projects each
  manifest capability into one MCP tool with auto-derived input
  schema; adding capabilities to Pulse extends the MCP surface
  without changes in the adapter. Run with `--emit-notifications`
  to also translate Pulse's SSE events (`finding.created`,
  `approval.pending`, `action.completed`) into JSON-RPC
  notifications on the stdio channel so autonomous MCP-bound agents
  can react to push events without holding a separate HTTP
  connection.

## What it does not do yet

- The substrate does not yet expose the governed action-execution
  surface (`/api/actions/plan`, `/api/actions/{id}/decision`,
  `/api/actions/{id}/execute`) as agent-stable manifest entries.
  Those handlers exist and are wired through the action audit store,
  but they emit a different error envelope from the agent surface
  (`APIError` shape: stable code under `code`, human message under
  `error`) versus the agent-stable shape (stable code under `error`,
  human under `message`). Adding them to the manifest as-is would
  force agents to remember which envelope each capability uses.
  Resolving that mismatch is a focused slice of its own. Until then,
  action governance flows through the existing approval store and
  the `action.completed` push channel: the AI service plans, an
  operator (or operator-acting agent) approves, the substrate emits
  the event.

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
- MCP adapter: `cmd/pulse-mcp/` (with README).
- HTTP worked example: `cmd/agent-probe/`.
- Subsystem dependencies: relevant paragraphs in `agent-lifecycle.md`,
  `performance-and-scalability.md`, and `storage-recovery.md` under
  `docs/release-control/v6/internal/subsystems/`.
