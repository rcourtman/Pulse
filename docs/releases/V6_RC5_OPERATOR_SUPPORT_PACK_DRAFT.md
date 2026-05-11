# Pulse v6 RC5 Draft Operator Support Pack

_Draft only. Use this as the working support brief for the planned
`v6.0.0-rc.5` candidate until the final prerelease notes are published._

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.5` is an opt-in evaluation build, not the default
  production recommendation.
- `rc.5` should be described as a feature-bearing RC covering the
  agent-substrate HTTP contract, Patrol findings as a Pulse Intelligence
  surface, resource operator-state and maintenance windows, action-
  governance plan-hash drift and post-dispatch verification, an AI
  reporting narrative layer, agentless availability probes, an
  accessibility sweep, DeepSeek V4 Patrol support, platform-identity work
  for Proxmox VE / PBS / PMG / Unraid agent hosts, and a sizable
  alerts-manager decomposition refactor.
- Self-hosted SSO is included with Community and higher tiers. Do not
  describe SAML or multi-provider SSO as a Pro-only upgrade path for this
  RC.
- Stable-channel installer resolution must stay on the latest stable
  semver tag even if GitHub's floating latest-release redirect currently
  points at an RC.
- Systems pinned to the historical `rc.2` update trust root should use a
  manual reinstall or explicit trust migration for later prerelease or GA
  builds.

## Short Answers

### Is `rc.5` the stable release?

No. The current stable release is v5.1.29. `rc.5` is still a v6
prerelease for controlled evaluation.

### Should production v5 users upgrade immediately?

No. The recommended RC posture is still staging, lab, or controlled
evaluation first.

### What is the rollback target?

Use v5.1.29:

`./scripts/install.sh --version v5.1.29`

### Can `rc.2` systems auto-update directly to `rc.5`?

Do not promise unattended continuity from `rc.2` to `rc.5`. Hosts pinned
to the historical `rc.2` update trust root need a manual reinstall or
explicit trust-migration path for later prerelease or GA builds.

### Does self-hosted v6 still cap monitored systems?

No for the current public self-hosted plans. Community, Relay, and Pro
include core self-hosted monitoring by default.

Current plan shorthand:

- Community:
  core monitoring included, OIDC/SAML SSO with multi-provider support,
  7-day history
- Relay:
  core monitoring included, secure remote access to the Pulse web UI,
  Pulse Mobile pairing for handoff, push notifications, and 14-day
  history
- Pro:
  Relay plus AI operations, automation, advanced admin features, and
  90-day history

### What happens to existing paid Pulse Pro customers in v6?

Use this cohort breakdown:

- Legacy recurring monthly or annual subscribers from v5 or earlier who
  were already active before the public v6 pricing cutover:
  keep the current recurring price, with self-hosted monitoring and
  child-resource volume not metered while the subscription remains
  continuously active under the current v6 policy.
- Existing lifetime customers:
  remain permanently valid, with self-hosted monitoring and child-
  resource volume not metered under the current v6 policy.
- Legacy paid v5 licenses migrated into v6 outside the recurring
  grandfathered path:
  can still exchange into the v6 activation model without repurchasing.
- Former recurring subscribers who already canceled or later lapse:
  any later return uses current public v6 pricing rather than resuming
  the old grandfathered terms.

### What changed from `rc.4` that users should notice immediately?

- a new `/api/agent/*` substrate exposes Patrol-level context to external
  agents through capabilities, resource-context, fleet-context, and SSE
  event streams; `cmd/pulse-mcp` and `cmd/agent-probe` ship as worked
  examples
- Patrol findings carry in-place verbs (Investigate, Why, Verify fix,
  Create rule from this, Mark resolved, Copy summary), structured
  investigation records with operator-facing Impact and rollback, and a
  first-class Resolved-finding lifecycle; legacy "Active alert detected"
  mirror findings are retired
- a resource operator-state foundation lets operators set intentionally-
  offline, do-not-remediate, maintenance-window, and criticality intent;
  Patrol auto-acknowledges new findings during a maintenance window and
  on intentionally-offline resources; action dispatch refuses against
  operator-locked resources
- action governance refuses dispatch when the approved plan hash drifts,
  records a refused audit entry, runs a class-derived verification check
  after successful dispatch, redacts known secret shapes from the audit
  log, and surfaces verification on the `action.completed` SSE payload
- AI reporting now has an optional narrative layer (per-tenant, fleet
  level, cost-tracked) and is exposed to Assistant through
  `pulse_summarize`
- agentless availability targets let Pulse probe network-reachable
  services without an installed Pulse Agent
- an accessibility sweep raises keyboard reachability, screen-reader
  announcements, and WCAG AA muted-text contrast across roughly fifteen
  surfaces
- DeepSeek V4 is supported in Patrol, including direct model selection
  and `tool_choice` coercion to `auto`
- platform identity, cluster source aggregation, and infrastructure
  presentation are improved for Proxmox VE, PBS, PMG, and Unraid agent
  hosts

### What if an external agent cannot read `/api/agent/capabilities`?

The capabilities manifest is unauthenticated by design so an agent
without a token can introspect Pulse before asking for one. If the
endpoint returns an auth error or 404, collect the Pulse version, exact
URL, transport, and reverse-proxy configuration and escalate.

### What if `/api/actions/{id}/execute` returns `plan_drift:` or `resource_remediation_locked:`?

That is the expected fail-closed behavior:

- `plan_drift:` means the executed payload no longer matches the approved
  plan hash; an audit entry is persisted and the operator should review
  the plan before re-approving
- `resource_remediation_locked:` means the target resource is operator-
  locked against remediation, and the operator must either remove the
  lock or reroute the action

Both prefixes reach the wire verbatim so agents and UI can branch on
codes.

### What if a Patrol finding does not auto-acknowledge during a maintenance window?

Confirm that the resource has an active maintenance-window operator-state
entry (`/api/resources/{id}/operator-state`), and that the finding's
created-at timestamp falls inside the window. If both are true and the
finding still surfaces, collect the resource ID, the operator-state
payload, the finding ID, and Patrol session logs and escalate.

### What if a hosted, checkout, magic-link, or SSO flow still keys access by email?

Escalate it as an identity regression. `rc.5` continues the `rc.4`
expectation that stable user and organization principals are used at
those trust boundaries.

### What if a fresh Proxmox LXC stable install lands on a v6 RC?

Treat that as a release-blocking install regression. The stable path
should resolve to v5.1.29 unless the user intentionally chose a v6
prerelease.

### What if a Docker agent duplicates or loses identity after recreation?

Collect logs and escalate. `rc.5` keeps the prior reconnect-token and
host-identity binding work and adds stricter root-agent and Proxmox token
ACL hardening from `rc.4`.

### Are public issue comments or closures required for the RC?

No public GitHub state changes are required just to prepare this packet.
Draft comments, closures, or retitles still need explicit maintainer
approval before posting.

## Recommended Evaluation Path

1. Back up the current system and keep direct console access available.
2. Confirm the current stable rollback command:
   `./scripts/install.sh --version v5.1.29`
3. If the host was on `rc.2`, use a manual reinstall or explicit trust-
   migration path rather than assuming unattended auto-update continuity.
4. Upgrade the Pulse server in a staging or otherwise controlled
   environment.
5. Verify server health, version, logs, and update UI before upgrading
   agents.
6. Exercise the new `/api/agent/*` substrate from `cmd/agent-probe`,
   `cmd/pulse-mcp`, or an external client. Confirm `plan_drift:` and
   `resource_remediation_locked:` token prefixes reach the wire verbatim.
7. Exercise Patrol findings with the new in-place verbs (Investigate,
   Why, Verify fix, Create rule, Mark resolved, Copy summary) and the
   Resolved tab.
8. Write operator-state through the resource detail drawer for at least
   one resource (intentionally offline, do-not-remediate, maintenance
   window) and confirm new findings auto-acknowledge as expected.
9. Approve an action plan, drift the payload, and confirm dispatch
   refuses with `plan_drift:`. Approve another plan, run it cleanly, and
   confirm the verification outcome renders on the action history row
   and on `action.completed` SSE.
10. Run a Pulse Pro report with the AI narrative layer enabled and
    confirm cost is recorded in the cost ledger. Call `pulse_summarize`
    from Assistant and confirm the same narrator is reachable.
11. Configure an agentless availability target and confirm the
    infrastructure row surfaces probe evidence.
12. Re-test DeepSeek V4 Patrol runs with direct DeepSeek model selection
    and multi-turn tool calls.
13. Re-test platform identity on agent-hosted Proxmox VE, PBS, PMG, and
    Unraid hosts, and confirm the cluster source badge aggregates across
    attached agents.
14. Walk the keyboard and screen-reader paths through the refreshed
    surfaces.
15. Re-test release artifact download, checksum/signature, installer,
    and draft validation paths before broader retesting.
16. Upgrade agents separately only when the user is explicitly testing
    the v5-to-v6 agent path.

## Ask For These Details

When a user reports an `rc.5` problem, ask for:

- current version and prior version
- install type
- whether the host was previously on v5, `rc.1`, `rc.2`, `rc.3`, or
  `rc.4`
- whether a manual reinstall or trust migration was used after `rc.2`
- whether the issue happened during server upgrade, agent upgrade,
  identity handoff, checkout, SSO, action planning, action execution,
  agent-substrate use, Patrol, alerting, AI reporting, availability
  probes, backup/recovery, platform inventory, or first use
- whether Unified Agents were upgraded yet
- expected result
- actual result
- sanitized logs, screenshots, and diagnostics

## Escalate Immediately

Escalate without asking the user to keep experimenting when the report
involves:

- failed install or failed upgrade with no recovery path
- stable install path unexpectedly landing on a v6 prerelease
- duplicate or missing agent identity after a v5-to-v6 upgrade
- hosted, checkout, magic-link, SSO, webhook, or token access granted to
  the wrong principal
- action execution that proceeds when dry-run or plan-hash validation
  failed
- agent-substrate endpoints returning data scoped to a tenant other than
  the requester
- Patrol auto-acknowledge or operator-state suppression behaving
  inconsistently with the recorded operator intent
- monitoring or reporting that stops entirely after upgrade
- rollback failure or inability to return to v5.1.29
- SSO setup or login blocked by an unexpected paid-license requirement
- data-loss, destructive behavior, or security-sensitive regressions

## Canonical References

- `docs/releases/RELEASE_NOTES_v6_RC5_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC5_DRAFT.md`
- `docs/AGENT_SUBSTRATE.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
