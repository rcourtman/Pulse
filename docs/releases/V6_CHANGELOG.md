# Pulse v6.0.0

_This changelog describes the shipped stable `v6.0.0` release compared with
`v5.1.27`. It includes the corrective changes that were validated across
`v6.0.0-rc.1` and `v6.0.0-rc.2`._

## What v6 changes at a high level

Pulse v6 keeps the platform-shaped top-level navigation existing v5 operators
already know (Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Standalone, plus
Alerts, Patrol, and Settings) and rebuilds the runtime behind it on a unified
resource model. The default top-level shape is the same shape v5 had; the
data flowing into those pages is the v6 unified `Resource` contract served
from `/api/resources`.

For existing Pulse v5 operators, this is not just a visual refresh. The live-
state contract changes, install and onboarding are split differently inside
Settings, self-hosted commercial posture now revolves around core monitoring
included for self-hosted installs plus paid convenience, history, and AI/admin
surfaces rather than capped monitored-system volume, and there are new top-
level pages (vSphere, Standalone, Patrol).

The v6 line briefly shipped a unified `Infrastructure` / `Workloads` /
`Storage` / `Recovery` layout across `rc.1` through `rc.5`. Operator feedback
consistently preferred the platform-shaped navigation v5 already had, so I
reverted the frontend information architecture in `rc.6` to platform-shaped
pages while keeping the unified resource model on the backend. Same backend,
the navigation shape you already know.

## Major product and workflow changes

- **The top-level product layout stays platform-shaped, on a unified
  backend.** Proxmox, Docker, Kubernetes, TrueNAS, vSphere, and Standalone
  are each their own top-level page, alongside Alerts, Patrol, and Settings.
  Behind those pages, Pulse v6 runs on a unified `Resource` contract
  (`/api/resources`) and per-platform pages consume that contract.

- **vSphere is a first-class platform.** vSphere has a top-level page
  parallel to Proxmox, Docker, Kubernetes, and TrueNAS, with VMs through
  the shared workloads pipeline, network inventory, hosts with version and
  uptime, cluster services, VM hardware config, VMware Tools status,
  vCenter MoRef on the workload ID, snapshot trees, and a vSphere placement
  card in the workload drawer.

- **Patrol is a first-class intelligence surface.** Patrol findings carry
  in-place verbs (Investigate, Why, Verify fix, Create rule, Mark resolved),
  structured investigation records with operator-facing Impact and rollback,
  and a first-class resolved-finding lifecycle. Capacity-forecast action
  templates and a reliability finding for flapping alerts are part of the
  shipped surface.

- **An external agent substrate ships.** `/api/agent/capabilities`,
  `/api/agent/resource-context/{id}`, `/api/agent/fleet-context`, and
  `/api/agent/events` give external agents (Claude Desktop, Claude Code,
  custom MCP clients, plain HTTP consumers) the same situated context
  Patrol and Assistant have. Worked examples ship in `cmd/pulse-mcp` and
  `cmd/agent-probe`.

- **Recovery is served as a backend contract.** `/api/recovery/*` aggregates
  PBS snapshots, ZFS snapshots, and replication tasks. The platform-shaped
  pages consume that contract; there is no top-level Recovery page in the
  shipped frontend.

- **Infrastructure onboarding is split by ownership inside Settings.**
  `Settings → Infrastructure → Install on a host` is the path for machines
  that should run the Unified Agent directly. `Settings → Infrastructure →
  Platform connections` is the path for API-backed systems such as Proxmox,
  TrueNAS, and VMware.

- **Adding infrastructure is more structured.** The shipped v6 line includes
  cluster agent deployment workflows with candidate discovery, preflights,
  jobs, event streams, cancel, and retry paths. That is a real workflow change
  from v5's more manual install-command model.

- **Licensing and activation behave differently.** V6 tracks entitlement
  state, continuity cohorts, commercial posture, and trial eligibility more
  explicitly. It also has dedicated behavior for upgrading supported paid v5
  licenses into v6 entitlements.

- **Self-hosted core monitoring is no longer commercially capped on current
  public plans.** Community, Relay, and Pro include self-hosted core
  monitoring by default. Relay sells remote/mobile convenience plus 14-day
  history, while Pro adds AI operations, automation, advanced admin surfaces,
  and 90-day history.

- **Hosted, org, and relay/mobile capabilities are part of the shipped stable
  line.** They were already present in the RC line, and `v6.0.0` keeps them as
  governed product surfaces rather than beta-only sidecars.

## What existing Pulse v5 users should re-test first

1. **Navigation and bookmarks.** The platform-shaped top-level pages (`/proxmox`, `/docker`, `/kubernetes`, `/truenas`, `/vmware`, `/standalone`) are the canonical v6 routes. The unified `/infrastructure`, `/workloads`, `/storage`, and `/recovery` routes that briefly shipped across `rc.1`-`rc.5` are retired in `rc.6` and onward; any bookmark or runbook pointing at those should move to the platform-shaped equivalent.

2. **Any custom automation that reads Pulse state.** If there are scripts, dashboards, browser extensions, or internal tooling that depended on v5-style `/api/state` or websocket payloads, re-test those before anything else.

3. **Backup and recovery workflows.** Re-test any workflow that assumed the old backup page model or backup-focused API responses. Treat recovery as a migration area.

4. **Install and bootstrap flows.** Re-test copied install commands, setup-script flows, and any automation that provisions new systems into Pulse. V6 keeps install automation, but the generated bootstrap artifacts are more structured and more controlled.

5. **License activation and paid continuity after upgrade.** If the v5 system
   has a paid license, verify the v6 entitlement state, lifetime or recurring
   continuity cohort, and core-monitoring-included commercial posture immediately
   after first boot.

6. **Upgrade against a real v5 data copy.** Use a copy of an actual v5 data directory and verify sessions, alert configuration, AI settings, metrics history, audit history, and filesystem assumptions after the v6 startup migration.

7. **Platform-specific onboarding.** If the deployment uses Proxmox, TrueNAS, VMware, relay, or mobile onboarding, re-test those as separate tracks rather than assuming the old host-agent path covers them.

## Breaking or compatibility-sensitive changes

- **The settings structure changed.** Settings is reorganised in v6; existing links into specific settings sub-pages should be reviewed. The platform-shaped top-level routes match the v5 shape, so most navigation bookmarks will continue to resolve; bookmarks into the briefly-shipped unified routes (`/infrastructure`, `/workloads`, `/storage`, `/recovery`) are retired and need to move to the platform-shaped equivalents.

- **The v5 live-state contract is not the v6 contract.** If custom code depends on `nodes`, `vms`, `containers`, `dockerHosts`, `hosts`, `storage`, or `backups` in `/api/state` or websocket payloads, it should be treated as migration work.

- **Backup-specific assumptions are a migration point.** The v5 backup route family and backup-heavy global state shape should not be treated as the v6 public model.

- **The canonical agent API changed.** New integrations should target `/api/agents/agent/*`. Older `/api/agents/host/*` paths remain as compatibility aliases, not as the preferred v6 contract.

- **Several legacy type names are no longer safe assumptions.** Custom code that still assumes names such as `host`, `container`, `docker_container`, or `host:` resource IDs should be reviewed against v6 naming and identity rules.

- **The on-disk data layout changes during upgrade.** V6 migrates local state into `orgs/default` and leaves compatibility symlinks behind. Any backup, restore, packaging, or filesystem tooling that assumes the flat v5 layout should be re-tested.

- **Bootstrap material is more controlled.** Setup-script flows now use explicit short-lived setup tokens and generated commands with governed privilege-escalation wrappers. Automation that assumed the older install-command shape may need changes.

## Under the hood but important

- **V6 ships with a real v5 upgrade path.** The migration tests cover config
  loading, encrypted config round-trips, session continuity, CSRF token
  continuity, metrics database migration, and audit database preservation.

- **Paid v5 licenses are handled explicitly.** Supported v5 paid licenses can
  auto-exchange into v6 entitlements, while unresolved paid-license migration
  states are recorded and surfaced instead of being silently ignored.

- **Monitored-system counting is still deliberate where continuity needs it.**
  Current public self-hosted plans include core monitoring by default, but canonical monitored-system
  identity still matters for migration truth, inventory language, and the
  remaining continuity cohorts that are preserved inside the v6 model.

- **Grandfathering is explicit rather than implied.** When a migrated v5
  installation qualifies for a special continuity cohort, that state is tracked
  and surfaced in v6 instead of being left to support-side interpretation.

- **Released install assets are tied to the release tag.** For released builds,
  install-script resolution is pinned to the shipped release asset, not to
  later branch state.

- **Relay now has persistent runtime identity.** If relay is enabled, v6
  persists an instance identity keypair so the relay runtime has stable
  identity across restarts.
