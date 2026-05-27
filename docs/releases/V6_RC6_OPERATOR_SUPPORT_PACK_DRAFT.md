# Pulse v6 RC6 Draft Operator Support Pack

_Draft only. Use this as the working support brief for the planned
`v6.0.0-rc.6` candidate until the final prerelease notes are published._

## Support Stance

- Pulse v5.1.32 remains the current stable line.
- Pulse v6 `rc.6` is an opt-in evaluation build, not the default
  production recommendation. The framing is closer to "pre-release
  for testing" than to "GA candidate"; the build is published on the
  existing `rc` update channel because the prerelease update path is
  RC-shaped (`internal/updates/version.go`,
  `internal/config/config.go`), not because the RC is days from GA.
- `rc.6` should be described as the frontend-reset RC. The headline
  change is that the unified `Infrastructure` / `Workloads` /
  `Storage` / `Recovery` top-level pages that shipped across
  `rc.1`-`rc.5` are reverted, and the frontend is platform-shaped
  again (Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines, plus
  Alerts, Patrol, and Settings). The unified resource model and
  `/api/resources` contract remain on the backend; the platform-shaped
  pages consume that contract and add platform-shaped presentation.
- Beyond the IA revert, `rc.6` ships vSphere as a first-class platform,
  the Hosts-to-Machines rename with native detail UX,
  TrueNAS native detail rendering, Proxmox backup recovery coverage
  and tab polish, FilterBar adoption with SavedViews, Patrol
  capacity-forecast and reliability-finding additions, the free-first
  self-hosted commercial posture, and `install.sh` release-pipeline
  hardening.
- Self-hosted SSO is included with Community and higher tiers. Do not
  describe SAML or multi-provider SSO as a Pro-only upgrade path for
  this RC.
- Stable-channel installer resolution must stay on the latest stable
  semver tag even if GitHub's floating latest-release redirect
  currently points at an RC.
- Systems pinned to the historical `rc.2` update trust root should use
  a manual reinstall or explicit trust migration for later prerelease
  or GA builds.

## Short Answers

### Is `rc.6` the stable release?

No. The current stable release is v5.1.32. `rc.6` is still a v6
prerelease for controlled evaluation.

### Should production v5 users upgrade immediately?

No. The recommended RC posture is still staging, lab, or controlled
evaluation first.

### Why does the frontend look like v5 again?

That is the point of `rc.6`. Across `rc.1` through `rc.5`, operator
feedback on the unified `Infrastructure` / `Workloads` / `Storage` /
`Recovery` layout consistently preferred the platform-shaped
navigation v5 already had. `rc.6` reverts the frontend information
architecture to platform-shaped top-level pages while keeping the
unified resource model on the backend. Same backend, the navigation
shape v5 operators already know.

### What is the rollback target?

Use v5.1.32:

`./scripts/install.sh --version v5.1.32`

### Can `rc.2` systems auto-update directly to `rc.6`?

Do not promise unattended continuity from `rc.2` to `rc.6`. Hosts
pinned to the historical `rc.2` update trust root need a manual
reinstall or explicit trust-migration path for later prerelease or GA
builds.

### Will my `rc.1`-`rc.5` bookmarks still work in `rc.6`?

Bookmarks to the platform-shaped top-level routes (`/proxmox`,
`/docker`, `/kubernetes`, `/truenas`, `/vmware`, `/standalone`,
`/alerts`, `/patrol`, `/settings`) continue to resolve unchanged.
Bookmarks into the briefly-shipped unified routes (`/infrastructure`,
`/workloads`, `/storage`, `/recovery`) are retired and need to move to
the platform-shaped equivalents. The unified resource model and
`/api/resources` contract remain on the backend, so automation
targeting the API contract is unaffected.

### Does self-hosted v6 still cap monitored systems?

No for the current public self-hosted plans. Community, Relay, and Pro
include core monitoring included by default.

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

### What changed from `rc.5` that users should notice immediately?

- The frontend top-level navigation is platform-shaped again. The
  unified `Infrastructure` / `Workloads` / `Storage` / `Recovery`
  pages, their aggregate route aliases, and the orphaned summary
  components and aggregate state hooks that fed them are removed.
  Drawer shells are unified across Workloads, Docker, K8s, and host
  detail drawers so platform pages share their detail surface. The
  keyboard shortcuts moved to platform keys: `g p` Proxmox, `g d`
  Docker, `g k` Kubernetes, `g n` TrueNAS, `g v` vSphere, `g s`
  Machines, `g a` Alerts, `g r` Patrol, `g t` Settings.
- vSphere is a first-class platform in `rc.6`, parallel to Proxmox,
  Docker, Kubernetes, and TrueNAS. The surface ships VMs through the
  shared workloads pipeline, network inventory, Hosts table with
  version and uptime, cluster services, VM hardware config, VMware
  Tools status, vCenter MoRef in the workload ID, snapshot trees, and
  a vSphere placement card in the workload drawer.
- The Pulse Agent inventory page that v5 operators know as "Hosts"
  is "Machines" in `rc.6`. The Machines table gained row identity
  context, expansion affordance, IP / disk I/O / RAID / network /
  temperature detail tooltips, an aggregate disk summary, SMART
  temperature fallback, and machine discovery promoted into drawer
  tabs. Machines is restricted to Pulse Agent resources.
- TrueNAS gained native inline detail rendering across storage,
  system, service, protection, and health rows. TrueNAS health alerts
  surface on the overview and alert detail rows appear in the drawer.
- Proxmox backup tabs got click-to-sort across all three tabs, visual
  density alignment with Storage and Ceph pages, a canonical
  ProgressBar for metric bars, a workload-row backup-age display, and
  a coverage view that surfaces which workloads have recent PBS
  artifacts. The Replication tab is hidden when no replication signals
  exist.
- FilterBar pattern (chips plus `+ Filter`) expanded into Alerts
  history, the audit log filter form, the embedded workloads filter,
  and Storage, with URL-backed filter state and SavedViews. Workloads
  search and `statusMode` moved to URL params; the localStorage backup
  for `viewMode` and `containerRuntime` was dropped. URL state is the
  single source of truth.
- Patrol gained a capacity-forecast action template registry, forecast
  proposals that attach onto a Patrol `RemediationPlan`, a reliability
  finding that fires when an alert starts flapping, a PDM (Proxmox
  Datacenter Manager) HTTP alert source and bridge, and a
  verification-outcome and capability-postcondition substrate for
  finding lifecycle.
- Self-hosted commercial framing moved to free-first. Trial start
  route, trial signup control plane, trial activation callback, AI
  quickstart surfaces, hosted AI quickstart runtime, monitored-system
  handoff prompts, and inactive Pro upsell helpers are retired.
  Self-hosted guest capacity caps are removed; self-hosted Pro
  continuity holds with no caps.
- The release pipeline now ships `install.sh` as a GitHub Release
  asset, gates the release on an end-to-end `install.sh` smoke test
  against the published release, and self-tests the smoke gate on
  every workflow edit. The archive install path requires a `.sshsig`
  sidecar. Windows agent onboarding moved to a seamless install flow.

### What if a user evaluated `rc.1`-`rc.5` on the unified IA and held back?

`rc.6` is the build to retest. Explain plainly: the unified IA
feedback was consistently in favour of the platform-shaped navigation
v5 had, and the frontend reverted on the same v6 backend. No data
migration is needed because the resource model and `/api/resources`
contract did not change.

### What if a user complains that "the navigation keeps changing"?

Acknowledge it. `rc.1`-`rc.5` was a frontend information architecture
experiment that operator feedback did not support. `rc.6` resets the
frontend to the platform-shaped shape v5 had, and the intent is for
that to remain the v6 frontend shape going forward. The unified
backend remains so future features can rely on a single resource
contract without reshuffling the top-level navigation again.

### What if a Patrol finding does not auto-acknowledge during a maintenance window?

Confirm that the resource has an active maintenance-window operator-
state entry (`/api/resources/{id}/operator-state`), and that the
finding's created-at timestamp falls inside the window. If both are
true and the finding still surfaces, collect the resource ID, the
operator-state payload, the finding ID, and Patrol session logs and
escalate.

### What if `/api/actions/{id}/execute` returns `plan_drift:` or `resource_remediation_locked:`?

That is the expected fail-closed behavior:

- `plan_drift:` means the executed payload no longer matches the
  approved plan hash; an audit entry is persisted and the operator
  should review the plan before re-approving
- `resource_remediation_locked:` means the target resource is
  operator-locked against remediation, and the operator must either
  remove the lock or reroute the action

Both prefixes reach the wire verbatim so agents and UI can branch on
codes.

### What if a hosted, checkout, magic-link, or SSO flow still keys access by email?

Escalate it as an identity regression. `rc.6` continues the `rc.5`
expectation that stable user and organization principals are used at
those trust boundaries.

### What if a fresh Proxmox LXC stable install lands on a v6 RC?

Treat that as a release-blocking install regression. The stable path
should resolve to v5.1.32 unless the user intentionally chose a v6
prerelease.

### What if a Docker agent duplicates or loses identity after recreation?

Collect logs and escalate. `rc.6` keeps the prior reconnect-token and
host-identity binding work and the root-agent and Proxmox token ACL
hardening from `rc.4` / `rc.5`.

### What if `install.sh` does not validate the `.sshsig` sidecar?

Escalate as a release-pipeline regression. The archive install path
in `rc.6` requires a `.sshsig` sidecar; a path that completes without
the sidecar check is a regression in the installer extraction
hardening.

### Are public issue comments or closures required for the RC?

No public GitHub state changes are required just to prepare this
packet. Draft comments, closures, or retitles still need explicit
maintainer approval before posting.

## Recommended Evaluation Path

1. Back up the current system and keep direct console access
   available.
2. Confirm the current stable rollback command:
   `./scripts/install.sh --version v5.1.32`
3. If the host was on `rc.2`, use a manual reinstall or explicit
   trust-migration path rather than assuming unattended auto-update
   continuity.
4. Upgrade the Pulse server in a staging or otherwise controlled
   environment.
5. Verify server health, version, logs, and update UI before
   upgrading agents.
6. Walk the frontend top-level navigation: each of Proxmox, Docker,
   Kubernetes, TrueNAS, vSphere, Machines loads as its
   own page. Confirm no `/infrastructure`, `/workloads`, `/storage`,
   or `/recovery` aggregate routes remain. Confirm Command Palette
   (`Cmd/Ctrl+K`) navigates across platforms and the platform-keyed
   shortcuts (`g p / g d / g k / g n / g v / g s / g a / g r / g t`)
   work.
7. Exercise vSphere as a first-class platform: VMs through workloads,
   network inventory, hosts with version and uptime, cluster services,
   VM hardware config, VMware Tools status, vCenter MoRef in workload
   ID, snapshot trees, vSphere placement card in the drawer.
8. Exercise Machines: row identity, expansion affordance, IP / disk
   I/O / RAID / network / temperature detail tooltips, aggregate disk
   summary, SMART fallback, machine discovery drawer tabs.
9. Exercise TrueNAS native details: storage, system, service,
   protection rows, overview health alerts, alert detail rows in the
   drawer.
10. Exercise Proxmox backup tabs: click-sort across the three tabs,
    canonical ProgressBar, hidden Replication tab on no-signals,
    workload-row backup-age, recovery coverage view.
11. Exercise FilterBar + SavedViews: Alerts history, audit log
    filter, embedded workloads filter, Storage SavedViews, default-
    star visible, URL-backed workload search and `statusMode`.
12. Exercise Patrol: capacity-forecast action template, forecast
    proposals on RemediationPlan, flapping-alert reliability finding,
    PDM HTTP alert bridge.
13. Exercise the external agent substrate from `cmd/agent-probe`,
    `cmd/pulse-mcp`, or an external client. Confirm `plan_drift:` and
    `resource_remediation_locked:` token prefixes reach the wire
    verbatim.
14. Approve an action plan, drift the payload, and confirm dispatch
    refuses with `plan_drift:`. Approve another plan, run it cleanly,
    and confirm the verification outcome renders on the action
    history row and on `action.completed` SSE.
15. Run a Pulse Pro report with the AI narrative layer enabled and
    confirm cost is recorded. Call `pulse_summarize` from Assistant
    and confirm the same narrator is reachable.
16. Confirm the free-first self-hosted posture: no trial start route,
    no AI quickstart surfaces, no guest capacity caps, Pro continuity
    holds.
17. Re-test the `install.sh` end-to-end smoke gate against the
    published release; confirm the archive path requires a `.sshsig`
    sidecar; confirm Windows agent onboarding completes through the
    seamless install flow.
18. Re-test release artifact download, checksum/signature, installer,
    and draft validation paths before broader retesting.
19. Upgrade agents separately only when the user is explicitly
    testing the v5-to-v6 agent path.

## Ask For These Details

When a user reports an `rc.6` problem, ask for:

- current version and prior version
- install type
- whether the host was previously on v5, `rc.1`, `rc.2`, `rc.3`,
  `rc.4`, or `rc.5`
- whether a manual reinstall or trust migration was used after `rc.2`
- whether the issue happened during server upgrade, agent upgrade,
  identity handoff, checkout, SSO, action planning, action execution,
  agent-substrate use, Patrol, alerting, AI reporting, availability
  probes, backup/recovery, platform inventory, frontend navigation,
  vSphere inventory, Machines detail, TrueNAS detail, Proxmox backup
  tab interaction, FilterBar/SavedViews interaction, or first use
- whether Unified Agents were upgraded yet
- expected result
- actual result
- sanitized logs, screenshots, and diagnostics

## Escalate Immediately

Escalate without asking the user to keep experimenting when the
report involves:

- failed install or failed upgrade with no recovery path
- stable install path unexpectedly landing on a v6 prerelease
- duplicate or missing agent identity after a v5-to-v6 upgrade
- hosted, checkout, magic-link, SSO, webhook, or token access granted
  to the wrong principal
- action execution that proceeds when dry-run or plan-hash validation
  failed
- agent-substrate endpoints returning data scoped to a tenant other
  than the requester
- Patrol auto-acknowledge or operator-state suppression behaving
  inconsistently with the recorded operator intent
- monitoring or reporting that stops entirely after upgrade
- rollback failure or inability to return to v5.1.32
- SSO setup or login blocked by an unexpected paid-license
  requirement
- `install.sh` archive path completing without `.sshsig` sidecar
  validation
- data-loss, destructive behavior, or security-sensitive regressions

## Canonical References

- `docs/releases/RELEASE_NOTES_v6_RC6_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC6_DRAFT.md`
- `docs/releases/RELEASE_NOTES_v6.md`
- `docs/releases/V6_CHANGELOG.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SUBSTRATE.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md` (historical reference for the
  `rc.1`-`rc.5` unified-IA migration that was reverted in `rc.6`)
