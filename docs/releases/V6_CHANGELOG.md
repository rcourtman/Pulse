# Pulse v6.0.0-rc.1

_This changelog is based on the shipped `v6.0.0-rc.1` tag compared with `v5.1.27`. It does not describe later branch-only work on `pulse/v6-release`._

## What v6 changes at a high level

Pulse v6 changes both the shape of the product and the shape of the runtime behind it. Pulse v5 was organized mainly around Proxmox and separate platform-specific views. Pulse v6 is organized around five primary surfaces: `Dashboard`, `Infrastructure`, `Workloads`, `Storage`, and `Recovery`.

For existing Pulse v5 operators, this is not just a visual refresh. The default routes change, the main live-state contract changes, install and onboarding are split differently, and licensing now revolves around canonical monitored systems rather than older v5 assumptions. The right way to test v6 is to verify real operator workflows against those new surfaces and contracts, not just to look for familiar pages.

## Major product and workflow changes

- **The top-level product layout is different.** Opening Pulse no longer drops directly into a Proxmox overview. The v6 default route lands on `Dashboard`, with separate primary views for `Infrastructure`, `Workloads`, `Storage`, and `Recovery`.

- **Recovery is a first-class surface.** In v5, backup-related behavior was centered on backup-specific pages and route families. In v6, recovery is treated as its own primary surface, and `Recovery` replaces the older backup-first page model.

- **Infrastructure setup is split by ownership.** `Install on a host` is the path for machines that should run the unified agent directly. `Platform connections` is the path for API-backed systems such as Proxmox, TrueNAS, and VMware.

- **Adding infrastructure is more structured.** The shipped RC includes cluster agent deployment workflows with candidate discovery, preflights, jobs, event streams, cancel, and retry paths. That is a real workflow change from v5's more manual install-command model.

- **Licensing and activation behave differently.** V6 tracks entitlement state, monitored-system limits, commercial posture, and trial eligibility more explicitly. It also has dedicated behavior for upgrading supported paid v5 licenses into v6 entitlements.

- **Hosted, org, and relay/mobile capabilities are also part of the shipped RC.** They are present in `v6.0.0-rc.1`, but most existing self-hosted v5 operators can treat them as second-wave testing rather than the first things to validate.

## What existing Pulse v5 users should re-test first

1. **Navigation and bookmarks.** Re-test saved links, runbooks, screenshots, and operator habits that assumed the old Proxmox-first route structure.

2. **Any custom automation that reads Pulse state.** If there are scripts, dashboards, browser extensions, or internal tooling that depended on v5-style `/api/state` or websocket payloads, re-test those before anything else.

3. **Backup and recovery workflows.** Re-test any workflow that assumed the old backup page model or backup-focused API responses. Treat recovery as a migration area.

4. **Install and bootstrap flows.** Re-test copied install commands, setup-script flows, and any automation that provisions new systems into Pulse. V6 keeps install automation, but the generated bootstrap artifacts are more structured and more controlled.

5. **License activation after upgrade.** If the v5 system has a paid license, verify the v6 entitlement state, monitored-system count, grandfathering behavior, and downgrade safety immediately after first boot.

6. **Upgrade against a real v5 data copy.** Use a copy of an actual v5 data directory and verify sessions, alert configuration, AI settings, metrics history, audit history, and filesystem assumptions after the v6 startup migration.

7. **Platform-specific onboarding.** If the deployment uses Proxmox, TrueNAS, VMware, relay, or mobile onboarding, re-test those as separate tracks rather than assuming the old host-agent path covers them.

## Breaking or compatibility-sensitive changes

- **The default route and settings structure changed.** Existing links to old primary pages should be reviewed, even where legacy redirects still exist.

- **The v5 live-state contract is not the v6 contract.** If custom code depends on `nodes`, `vms`, `containers`, `dockerHosts`, `hosts`, `storage`, or `backups` in `/api/state` or websocket payloads, it should be treated as migration work.

- **Backup-specific assumptions are a migration point.** The v5 backup route family and backup-heavy global state shape should not be treated as the v6 public model.

- **The canonical agent API changed.** New integrations should target `/api/agents/agent/*`. Older `/api/agents/host/*` paths remain as compatibility aliases, not as the preferred v6 contract.

- **Several legacy type names are no longer safe assumptions.** Custom code that still assumes names such as `host`, `container`, `docker_container`, or `host:` resource IDs should be reviewed against v6 naming and identity rules.

- **The on-disk data layout changes during upgrade.** V6 migrates local state into `orgs/default` and leaves compatibility symlinks behind. Any backup, restore, packaging, or filesystem tooling that assumes the flat v5 layout should be re-tested.

- **Bootstrap material is more controlled.** Setup-script flows now use explicit short-lived setup tokens and generated commands with governed privilege-escalation wrappers. Automation that assumed the older install-command shape may need changes.

## Under the hood but important

- **V6 ships with a real v5 upgrade path.** The shipped migration tests cover config loading, encrypted config round-trips, session continuity, CSRF token continuity, metrics database migration, and audit database preservation.

- **Paid v5 licenses are handled explicitly.** Supported v5 paid licenses can auto-exchange into v6 entitlements, while unresolved paid-license migration states are recorded and surfaced instead of being silently ignored.

- **Monitored-system counting is more deliberate.** Limits are applied to canonical top-level monitored systems, including cases where Pulse sees the same machine through more than one collection path.

- **Grandfathering is explicit rather than implied.** When a migrated v5 installation qualifies for a higher monitored-system floor than the base v6 plan limit, that continuity state is tracked and surfaced in v6.

- **Released install assets are tied to the release tag.** For released builds, install-script resolution is pinned to the shipped release asset, not to later branch state.

- **Relay now has persistent runtime identity.** If relay is enabled, v6 persists an instance identity keypair so the relay runtime has stable identity across restarts.
