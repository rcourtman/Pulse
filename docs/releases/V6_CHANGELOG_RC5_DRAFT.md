# Pulse v6.0.0-rc.5 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the published `v6.0.0-rc.4` tag. Do not treat it as published until the
governed `v6.0.0-rc.5` prerelease exists._

## What `rc.5` changes compared with `rc.4`

`v6.0.0-rc.5` is a feature-bearing RC, not a targeted hardening pass. It does
not reopen the `rc.2` commercial model or replace v5.1.29 as the stable line.
It carries the post-`rc.4` Pulse Intelligence, agent-substrate, action-
governance, operator-state, AI reporting, availability-probe, accessibility,
DeepSeek V4, and platform-identity work into a single retestable candidate.

The main user-visible surfaces in this RC are the new agent-substrate HTTP
contract that gives external agents the same context Patrol and Assistant
have, the Patrol findings UX evolution into a proactive Pulse Intelligence
surface, and the resource operator-state foundation that lets operators
record per-resource intent (offline, do-not-remediate, maintenance window,
criticality) and have Patrol and action dispatch honor it.

## Commit Coverage Audit

The changelog was audited against every feature/runtime commit in the exact
code-backed release-validation range for the current candidate:

- `v6.0.0-rc.4`: `4aa91f6af37be08c93ff19e44f307735d1b9cb70`
- validation-risk commit: `52416cec6fdb42c0bf753d52ad870f4dfede5e1e`
- range: `v6.0.0-rc.4..52416cec6fdb42c0bf753d52ad870f4dfede5e1e`
- commit count: `426`
- changed scope: `687` files, `84642` insertions, `16632` deletions

Those commits are grouped in this changelog rather than listed one by one.
The range carries: the agent-substrate HTTP contract and `cmd/pulse-mcp` /
`cmd/agent-probe` worked examples, the Patrol findings UX expansion and
resolved-finding lifecycle, the resource operator-state foundation and
maintenance-window scheduler, action-governance hardening on plan-hash
drift and post-dispatch verification, the AI reporting narrative layer,
agentless availability probes, an accessibility sweep across roughly fifteen
surfaces, DeepSeek V4 tool-turn and tool-choice support, platform-identity
guardrails for Proxmox VE / PBS / PMG agent hosts, an alerts-manager
decomposition refactor, and a set of correctness fixes around fingerprint
loss, PVE version detection, PBS poll failure attribution, OpenAI streaming,
and self-hosted web-listener fail-fast.

## Major Changes

### 1. Agent substrate: external agents get the same context Patrol and Assistant have

Pulse v6 now exposes a stable HTTP contract for external agents so Claude
Desktop, Claude Code, custom MCP clients, and plain HTTP consumers can
drive Pulse with the situated context an in-process Patrol or Assistant
run has. The contract has four axes:

- discovery: `/api/agent/capabilities` returns a hand-authored unauthenticated
  manifest of every agent-consumable capability with name, description, HTTP
  method/path, required auth scope, response shape, and stable error codes
- depth: `/api/agent/resource-context/{id}` returns the situated picture of
  one resource in a single read: identity, operator-set state, active
  findings, pending approvals, recent actions including refused dispatches
  and verification probe outcomes, with stable token prefixes like
  `plan_drift:` and `resource_remediation_locked:` reaching the wire verbatim
- breadth: `/api/agent/fleet-context` returns a thin per-resource rollup
  across the whole org for "where do I focus?" reads, with per-resource
  bundles for follow-up depth
- realtime: `/api/agent/events` is an SSE stream that translates the existing
  Pulse SSE event firehose into MCP-shaped notifications for agents that
  opt in
- write surfaces: the operator-state intent loop
  (`/api/resources/{id}/operator-state` GET / PUT / DELETE) and the
  pre-existing action governance loop (`/api/actions/plan`,
  `/api/actions/{id}/decision`, `/api/actions/{id}/execute`) round out the
  substrate

Two worked examples ship in the tree: `cmd/agent-probe` consumes the
substrate end to end as a plain HTTP client, and `cmd/pulse-mcp` wraps the
substrate as an MCP adapter for clients that prefer that transport. The
substrate is the canonical surface; MCP is an adapter over it. The arc is
summarized in `docs/AGENT_SUBSTRATE.md`.

### 2. Patrol evolves into a Pulse Intelligence surface

Patrol findings now carry the verbs an operator needs in-place rather than
funneling everything through Assistant. Each active finding can expose
contextual actions (Investigate, Why, Verify fix, Create rule from this,
Explain, Mark resolved, Copy summary), the per-finding action area is
grouped into semantic clusters, and the row carries regression-count,
investigation confidence, post-dispatch verification outcome, and trust
metrics at a glance. The Patrol page header surfaces overall trust state,
verified-resource coverage, and the recommended next step.

Resolved findings now have a first-class lifecycle: the Resolved tab loads
them server-side, the All tab includes them, include-resolved persists
across polling, manual Mark resolved is wired through the existing
patrol_resolve path and attributes the closure to the operator, and
LLM-driven `patrol_resolve_finding` calls fail closed when the deterministic
verifier is inconclusive on event findings. Legacy "Active alert detected"
mirror findings have been retired and are purged on load.

Patrol investigation records are now structured: each record carries an
operator-facing Impact statement (consequence-if-ignored), aggregated
rollback steps lifted from the remediation plan, and previous resolved-fix
context that survives regressions.

### 3. Resource operator state and maintenance windows

A new resource operator-state foundation lets operators record per-resource
intent and have Pulse honor it:

- `/api/resources/{id}/operator-state` GET / PUT / DELETE handlers
- intentionally-offline, do-not-auto-remediate, maintenance-window, and
  criticality intent
- maintenance-window scheduler in the operator-overrides section of the
  resource detail drawer
- new findings on intentionally-offline resources auto-acknowledge
- new findings during an operator-set maintenance window auto-acknowledge
- operator-state-suppressed findings skip autonomous investigation, then
  wake when the suppression lifts
- action dispatch refuses when a resource is operator-locked against
  remediation, with the refusal recorded in the action audit
- operator-driven Mark resolved closures attribute to "Resolved by you"

### 4. Action governance: plan-hash drift, post-dispatch verification, and audit hardening

Action governance now fails closed on more boundaries:

- approved plan hash is verified at execution time; payload drift refuses
  dispatch and persists a refused audit record
- a class-derived verification check runs after successful dispatch and the
  outcome is rendered on action history rows alongside the action
- per-command-class preflight context is authored for approval review,
  including the new Proxmox VM/CT lifecycle preflight
- known secret shapes are redacted from action audit log persistence
- bundle-level approvals pin cross-org and cross-resource isolation on the
  pending approvals it carries
- the `action.completed` SSE payload now surfaces action verification so
  agents and UI can branch on it

### 5. AI reporting gets a real narrative layer

The non-rendering reporting engine now has documented entry points so
non-rendering callers can compose summaries:

- a heuristic report-narrative path is now an optional AI-generated layer
- fleet-level AI narrative for multi-resource reports
- per-tenant AI narrators are threaded into `pulse_summarize` via the chat
  session, so the same narration is available from Assistant
- chat-package Chat/ChatStream callers record token usage to the cost
  ledger, and report-narration records cost events the same way
- structured telemetry is emitted on reporting and summarize invocations
- the report narrators are explicitly forbidden from acting as parallel
  detectors; the detection-boundary invariant is now documented in the
  ai-runtime contract

### 6. Agentless availability probes

A new agentless availability path lets Pulse probe network-reachable
services without an installed Pulse Agent:

- `internal/config/availability.go` configures targets
- `internal/monitoring/availability_poller.go` runs probes
- `internal/api/availability_handlers.go` exposes the API surface
- mock availability fixtures (including an ESPHome example) ship in
  `internal/mock/availability_fixtures.go`
- availability probe evidence is surfaced on infrastructure rows
- availability target actions stay visible and the row presentation is
  refined for the smaller resource model

### 7. Patrol-to-Assistant handoffs are context-first and approval-bound

The Patrol-to-Assistant handoff path now carries structured context rather
than free text:

- Patrol findings, runs, assessments, and approval handoffs all route
  through model context (resource scope, action references, finding
  lifecycle, approval status, resource relationships, resource timeline,
  resource policy)
- the briefings carry safe action metadata, evidence cues, attention
  reasons, and the live approval lifecycle
- handoff payloads are one-shot and refresh before the picker opens so
  stale context cannot leak between turns
- chat-side handoffs and Patrol finding handoffs are clamped to the
  requested approval mode and require approval mode for governed
  investigation commands
- the chat-package narrator now uses session-bound per-tenant tools
- `patrol_summarize` is exposed to Assistant via the chat session
- Patrol runs are stateless and drop prior session history; `patrol-main`
  is bounded at 200 messages to stop unbounded disk growth

### 8. Accessibility sweep

A focused accessibility pass covering roughly fifteen surfaces:

- skip-to-content link for keyboard users
- AppLayout desktop tabs, alert metric edit cells, Patrol finding rows,
  mobile nav alert tab are now keyboard-accessible
- success and error feedback is announced to screen readers across
  login/change-password, AISettings/K8s namespaces, PMG/AI cost/merge
  modals, DataHandlingPanel, ProLicense activation, toasts, and Alerts
- icon-only buttons are labeled across the tree
- form controls that previously only had placeholders now have labels
- decorative dashes are hidden from screen readers and across more tables
- light-mode muted text now meets WCAG AA on alt surfaces and the rest of
  body-text muted callers move to the semantic token
- focus indicator is restored on FilterBar search inputs
- `document.title` reflects the active tab and PageHeader title

### 9. DeepSeek V4 support

- direct DeepSeek Patrol models stay selectable
- DeepSeek V4 Patrol defaults are aligned
- DeepSeek V4 tool turns are supported in Patrol
- DeepSeek `tool_choice` is coerced to `auto` so Patrol stops failing on
  providers that reject other values, with the rationale recorded in the
  ai-runtime contract
- the over-greedy "tools not supported" classifier is split into three
  distinct causes
- double-pipe DeepSeek DSML tool-call markers are sanitised in chat

### 10. Platform identity, cluster surfaces, and infrastructure presentation

- hybrid-source platform version precedence is pinned and dual-source
  identity guardrails are in place
- agent host profile detection covers Proxmox VE, PBS, PMG, and Unraid
- agent host profiles are propagated to resources
- the cluster source badge aggregates across attached agents and cluster
  member rows are clarified
- the cluster deploy banner hides on offline PVE nodes and uses
  "ready for Pulse Agent" wording instead of "unmonitored"
- platform versions show in system badges, with Proxmox `pve-manager`
  wrapper stripped
- PVE version detection on agent hosts is fixed
- PBS poll failures are attributed and locked down with regression tests
- silent fingerprint loss for LXC and VMs is fixed
- versioned infrastructure system badges are deduplicated
- the infrastructure source picker leads with the two primary paths,
  copy/order is tightened, and discovered sources stay in the base group

### 11. Self-hosted licensing continuity carries through unchanged

`rc.5` preserves the `rc.4` self-hosted licensing posture:

- monitored-system and child-resource volume are not metered in the
  current public self-hosted plans
- continuity paths do not write raw monitored-system caps back into
  runtime state
- Relay remains secure remote access to the Pulse web UI, Pulse Mobile pairing for handoff,
  push notifications, and 14-day history
- Pro remains Relay plus AI operations, automation, advanced admin
  features, and 90-day history

### 12. Alerts-manager decomposition

A multi-commit refactor splits the alerts manager into per-checker and
per-runtime modules: Proxmox guest alerts, Proxmox disk health, backup
snapshot, host, node, PBS/storage, Docker, PMG, alert-config runtime,
alert-metric runtime, alert-health-assessment runtime, alert-notification
policy, alert-read model, active-alert lifecycle/persistence/cleanup. The
public alerts API is unchanged.

### 13. Correctness and runtime stability

- self-hosted startup web listener fails fast on configuration errors
- OpenAI incomplete SSE streams fail closed
- audit list 500s now log the underlying error
- the v6 demo release signing key deployment is fixed
- SSHSIG is verified on in-app update artifacts
- patrol-main session bounded at 200 messages to stop unbounded disk growth
- AGENT-side update signer trust migration guidance carries through

## What existing v5 users should re-test in `rc.5`

1. v5.1.29 to v6 server upgrade and rollback to v5.1.29.
2. The explicit post-`rc.2` trust migration or manual reinstall path.
3. The new `/api/agent/*` substrate from an external HTTP or MCP client.
4. Patrol findings with the new in-place verbs: Investigate, Why, Verify
   fix, Create rule from this, Mark resolved, Copy summary.
5. The Patrol Resolved tab, include-resolved across polling, and the All
   tab containing resolved findings.
6. Resource operator-state writes through the resource detail drawer
   (intentionally offline, do-not-remediate, maintenance window,
   criticality), and confirmation that new findings during a maintenance
   window auto-acknowledge.
7. Action dispatch against an operator-locked resource (should refuse with
   `resource_remediation_locked:`) and against drifted plan hash (should
   refuse with `plan_drift:`).
8. AI reporting with the new narrative layer, both in scheduled reports
   and through `pulse_summarize` from Assistant.
9. Agentless availability targets and the infrastructure-row evidence
   that surfaces them.
10. Keyboard navigation and screen-reader announcements across the
    refreshed surfaces.
11. DeepSeek V4 in Patrol, including direct DeepSeek model selection and
    multi-turn tool calls.
12. Platform identity on agent-hosted Proxmox VE, PBS, PMG, and Unraid
    hosts; cluster source badge aggregation.
13. Release artifact download, checksum/signature, installer, and draft
    validation paths before broad retesting.

## Evidence Appendix

For the code-backed evidence packet that maps these claims to the current
release line, see:

- `docs/AGENT_SUBSTRATE.md`
- `docs/release-control/v6/internal/subsystems/ai-runtime.json`
- `docs/release-control/v6/internal/subsystems/api-contracts.json`
- `docs/release-control/v6/internal/subsystems/patrol-intelligence.json`
- `docs/release-control/v6/internal/subsystems/unified-resources.json`
- `docs/release-control/v6/internal/subsystems/monitoring.json`
