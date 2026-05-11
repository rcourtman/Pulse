# Pulse v6.0.0-rc.5 Draft Release Notes

_Draft only. Do not treat this as published until the governed
`v6.0.0-rc.5` tag and GitHub prerelease exist._

`v6.0.0-rc.5` is a feature-bearing RC after the published `rc.4`
prerelease. Pulse v5.1.29 remains the current stable line.

The purpose of this RC is to carry the latest post-`rc.4` release-branch
work into a retestable v6 candidate:

- the new agent-substrate HTTP contract gives external agents the same
  situated context Patrol and Assistant have, with worked examples in
  `cmd/pulse-mcp` (MCP adapter) and `cmd/agent-probe` (plain HTTP)
- Patrol findings evolve into a proactive Pulse Intelligence surface with
  in-place verbs (Investigate, Why, Verify fix, Create rule, Mark
  resolved), structured investigation records that carry operator-facing
  Impact and rollback, and a first-class resolved-finding lifecycle
- a new resource operator-state foundation lets operators record
  per-resource intent (intentionally offline, do-not-remediate,
  maintenance window, criticality), and Patrol plus action dispatch honor
  it
- action governance fails closed on plan-hash drift, runs class-derived
  post-dispatch verification, redacts known secret shapes from the audit
  log, and authors per-command-class preflight context for approval
  review
- the AI reporting engine gets a real narrative layer (per-tenant, fleet
  level, cost-tracked) and is exposed to Assistant through
  `pulse_summarize`
- agentless availability probes let Pulse monitor network-reachable
  services without an installed Pulse Agent
- an accessibility sweep across roughly fifteen surfaces (keyboard
  reachability, screen-reader announcements, WCAG AA muted text, focus
  indicators)
- DeepSeek V4 Patrol support including tool-turn coercion and direct
  DeepSeek model selection
- platform identity, cluster, and infrastructure-presentation work for
  Proxmox VE, PBS, PMG, and Unraid agent hosts
- a sizable alerts-manager refactor (decomposition only, no API change)

This packet was audited against `428` commits in the exact code-backed
`rc.4` to `rc.5` pre-publication validation range, from the published
`v6.0.0-rc.4` tag commit `4aa91f6af37be08c93ff19e44f307735d1b9cb70` through
validation-risk commit `e36945741e1db5d763ab63eeeda18a58acda23c5`. That
range covers 694 files changed, 85715 insertions, 16639 deletions, and
includes the agent-substrate, Patrol intelligence, operator-state,
action-governance, AI reporting, availability-probe, accessibility,
DeepSeek V4, platform-identity, and alerts-decomposition work, plus the
RC5 packet and release-validation commits including the plain-JSON
tool-call sanitisation for weak local models.

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.5` is still an opt-in evaluation build, not the default
  production recommendation.
- Existing v5 users should still prefer staging, lab, or otherwise
  controlled evaluation first.
- Hosts already pinned to the historical `rc.2` update trust root should
  not assume unattended auto-update continuity into later prerelease or
  GA builds. Use a manual reinstall or an explicit trust migration path
  for `rc.5`.
- The stable rollback target for this candidate is `v5.1.29`:
  `./scripts/install.sh --version v5.1.29`

## What Changed Since `rc.4`

### Agent Substrate (External Agents Get The Same Context Patrol Has)

Pulse v6 now exposes a stable HTTP contract for external agents:
`/api/agent/capabilities` for discovery, `/api/agent/resource-context/{id}`
for one-resource depth, `/api/agent/fleet-context` for org-wide breadth,
and `/api/agent/events` for SSE-to-MCP realtime notifications. The
operator-state and action-governance write surfaces round out the
contract. Two worked examples ship: `cmd/agent-probe` (plain HTTP) and
`cmd/pulse-mcp` (MCP adapter wrapping the same substrate). Stable token
prefixes like `plan_drift:` and `resource_remediation_locked:` reach the
wire verbatim so agents branch on codes, not human text. See
`docs/AGENT_SUBSTRATE.md` for the arc summary.

### Patrol As A Pulse Intelligence Surface

Patrol findings now carry the verbs operators need in-place: Investigate,
Why, Verify fix, Create rule from this, Explain, Mark resolved, Copy
summary. Per-finding action areas are grouped into semantic clusters, and
the row carries regression-count, investigation confidence, post-dispatch
verification outcome, and trust metrics at a glance. The page header
surfaces overall trust state, verified-resource coverage, and the
recommended next step. Resolved findings now have a first-class
lifecycle: server-side loading on the Resolved and All tabs,
include-resolved persisting across polling, manual Mark resolved
attributed to the operator, and `patrol_resolve_finding` failing closed
when the deterministic verifier is inconclusive on event findings.
Legacy "Active alert detected" mirror findings are retired.

Patrol investigation records are structured: each carries an
operator-facing Impact statement, aggregated rollback steps lifted from
the remediation plan, and previous resolved-fix context that survives
regressions.

### Resource Operator State And Maintenance Windows

A new resource operator-state foundation:

- `/api/resources/{id}/operator-state` GET / PUT / DELETE
- intentionally-offline, do-not-auto-remediate, maintenance-window, and
  criticality intent
- maintenance-window scheduler in the resource detail drawer
- new findings on intentionally-offline resources auto-acknowledge
- new findings during an operator-set maintenance window auto-acknowledge
- operator-state-suppressed findings skip autonomous investigation, then
  wake when the suppression lifts
- action dispatch refuses when a resource is operator-locked against
  remediation, with the refusal recorded in the action audit

### Action Governance Hardening

- approved plan-hash drift refuses dispatch and persists a refused audit
  record
- a class-derived verification check runs after successful dispatch and
  the outcome is rendered on action history rows
- per-command-class preflight context is authored for approval review,
  including a new Proxmox VM/CT lifecycle preflight
- known secret shapes are redacted from action-audit log persistence
- the `action.completed` SSE payload now surfaces action verification

### AI Reporting Gets A Real Narrative Layer

The reporting engine now has a real narrative layer: an optional
AI-generated report path, fleet-level AI narrative for multi-resource
reports, per-tenant narrators threaded into `pulse_summarize` via the
chat session, cost recording on chat and report narration, and structured
telemetry on reporting and summarize invocations. The detection-boundary
invariant is documented: report narrators are not parallel detectors.

### Agentless Availability Probes

Pulse can now monitor network-reachable services without an installed
Pulse Agent through agentless availability targets. The poller,
configuration, API handlers, and mock fixtures (including an ESPHome
example) ship in this RC, with probe evidence surfaced on infrastructure
rows.

### Accessibility

A focused accessibility pass across roughly fifteen surfaces: skip-to-
content, keyboard-reachable tabs/cells/rows, screen-reader announcements
across login, AI settings, K8s namespaces, PMG, ProLicense activation,
toasts, DataHandlingPanel, and Alerts; labeled icon-only buttons; form
controls labeled rather than placeholder-only; decorative dashes hidden
from screen readers; light-mode muted text raised to WCAG AA; focus
indicator restored on FilterBar search inputs; `document.title` reflects
the active tab and PageHeader.

### DeepSeek V4

Direct DeepSeek Patrol models stay selectable, V4 defaults are aligned,
V4 multi-turn tool calls are supported, `tool_choice` is coerced to
`auto` so Patrol stops failing on providers that reject other values, the
over-greedy "tools not supported" classifier is split into three
distinct causes, and double-pipe DeepSeek DSML tool-call markers are
sanitised in chat.

### Platform Identity, Cluster, And Infrastructure

Hybrid-source platform version precedence is pinned with dual-source
identity guardrails. Agent host profile detection covers Proxmox VE,
PBS, PMG, and Unraid. The cluster source badge aggregates across attached
agents, cluster member rows are clarified, the cluster deploy banner
hides on offline PVE nodes and now reads "ready for Pulse Agent" rather
than "unmonitored", and PVE version detection on agent hosts is fixed.
Silent fingerprint loss for LXC and VMs is fixed. PBS poll failures are
attributed and locked down with regression tests.

### Self-Hosted Licensing Continuity Carries Through

The `rc.4` self-hosted licensing posture stands unchanged in `rc.5`:
monitored-system and child-resource volume are not metered in the current
public self-hosted plans, continuity paths do not write raw monitored-
system caps back into runtime state, Relay remains secure remote access
to the Pulse web UI, Pulse Mobile pairing for handoff, push
notifications, and 14-day history, and Pro remains Relay plus AI
operations, automation, advanced admin features, and 90-day history.

### Refactor And Runtime Stability

A multi-commit alerts-manager decomposition splits the manager into
per-checker and per-runtime modules with no public-API change.
Self-hosted startup web-listener fails fast on configuration errors,
OpenAI incomplete SSE streams fail closed, audit-list 500s log the
underlying error, SSHSIG is verified on in-app update artifacts, the v6
demo release signing key deployment is fixed, and the patrol-main
session is bounded at 200 messages to stop unbounded disk growth.

## What Existing v5 Users Should Re-Test In `rc.5`

1. Server upgrade from the current v5 stable line to v6, including the
   manual or explicit trust-migration path needed for builds after
   `rc.2`.
2. Fresh Proxmox LXC install and rollback to v5.1.29.
3. The new `/api/agent/*` substrate from an external HTTP or MCP client.
4. Patrol findings with the new in-place verbs and the Resolved tab,
   including the auto-acknowledge behavior on intentionally-offline and
   maintenance-window resources.
5. Resource operator-state writes through the resource detail drawer and
   action dispatch refusal against an operator-locked resource.
6. AI reporting with the new narrative layer in scheduled reports and
   through `pulse_summarize` from Assistant.
7. Agentless availability targets and the infrastructure-row evidence.
8. Keyboard navigation and screen-reader announcements across the
   refreshed surfaces.
9. DeepSeek V4 in Patrol, including direct DeepSeek model selection and
   multi-turn tool calls.
10. Platform identity on agent-hosted Proxmox VE, PBS, PMG, and Unraid
    hosts, plus the aggregated cluster source badge.
11. Release asset download, checksum/signature, installer, and draft-
    release validation paths before broader retesting.

## Feedback

Use the `Pulse v6 pre-release feedback` issue template for regressions,
upgrade failures, licensing continuity problems, platform-specific
breakage, or actionable UX friction:

- `https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml`

When reporting an `rc.5` problem, include:

- Pulse version
- upgrade path or fresh-install path
- installation type
- whether the host was previously on `rc.1`, `rc.2`, `rc.3`, `rc.4`, or
  v5
- whether a manual reinstall or trust migration was used after `rc.2`
- what you expected
- what happened instead
- sanitized logs, screenshots, or diagnostics when helpful

## Operator References

- `docs/releases/V6_RC5_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC5_DRAFT.md`
- `docs/AGENT_SUBSTRATE.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
