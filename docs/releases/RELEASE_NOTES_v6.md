# Pulse v6.0.0 Release Notes

`v6.0.0` is the first stable release of Pulse v6. It promotes the validated
`v6.0.0-rc.6` candidate into the default supported v6 release.

Pulse v6 keeps the platform-shaped top-level navigation existing v5 operators
already know (Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines, plus
Alerts, Patrol, and Settings), rebuilds the runtime behind it on a unified
resource model and contract (`/api/resources`), ships first-class vSphere and
TrueNAS support, adds the Patrol intelligence and agent-substrate surfaces,
keeps the governed v5-to-v6 upgrade and Unified Agent continuity path, and
ships the corrected self-hosted commercial model that was validated across the
RC line.

The v6 line briefly shipped a unified `Infrastructure` / `Workloads` /
`Storage` / `Recovery` top-level layout across `rc.1` through `rc.5`. Operator
feedback consistently preferred the platform-shaped navigation v5 already had,
so I reverted the frontend information architecture in `rc.6` to platform-
shaped pages while keeping the unified resource model on the backend. Same
backend, the navigation shape you already know.

## Pulse v5 Support Transition

Pulse v5 entered maintenance-only support on `2026-06-04`.
I will ship only critical security, data-loss, licensing or billing blocker,
installer or updater failure, and safe migration blocker fixes for existing v5 users until `2026-09-02`.
After `2026-09-02`, Pulse v5 is end-of-support and new fixes land on v6 unless
I publish an explicit exception.

## What Is In v6.0.0

### Platform-shaped frontend on a unified backend

The top-level navigation is platform-shaped. Each backing source has its own
top-level page (Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines),
alongside Alerts, Patrol, and Settings. Behind those pages, Pulse v6 runs on
a unified resource model: a single canonical `Resource` type normalising data
across the backing sources, served from `/api/resources`. The platform-shaped
pages consume that contract and add platform-shaped presentation.

vSphere is a first-class platform in v6, parallel to Proxmox, Docker,
Kubernetes, and TrueNAS. Machines (the v5 "Hosts" page) carries Pulse
Agent resources as their own top-level page, with native detail UX
for IP, disk I/O, RAID, network, and SMART temperature.

### Patrol intelligence and the agent substrate

`Patrol` is a top-level intelligence surface with in-place verbs on findings
(Investigate, Why, Verify fix, Create rule, Mark resolved), structured
investigation records that carry operator-facing Impact and rollback, a first-
class resolved-finding lifecycle, capacity-forecast action templates, and a
reliability finding that fires when an alert starts flapping. Patrol consumes
an HTTP alert bridge for PDM (Proxmox Datacenter Manager).

Pulse v6 also exposes a stable HTTP contract for external agents
(`/api/agent/capabilities`, `/api/agent/resource-context/{id}`,
`/api/agent/fleet-context`, `/api/agent/events`) so Claude Desktop, Claude
Code, custom MCP clients, and plain HTTP consumers can drive Pulse with the
same situated context Patrol and Assistant have. Worked examples ship in
`cmd/pulse-mcp` (MCP adapter) and `cmd/agent-probe` (plain HTTP).

### Recovery and infrastructure onboarding

Backup, snapshot, and replication state is served from `/api/recovery/*`
(PBS snapshots, ZFS snapshots, replication tasks) and consumed by the
platform-shaped pages that present it. Infrastructure onboarding is split by
ownership inside Settings:

- `Settings → Infrastructure → Install on a host` for direct Unified Agent
  deployment
- `Settings → Infrastructure → Platform connections` for API-backed systems
  such as Proxmox, TrueNAS, and VMware

### Self-hosted packaging is corrected from the early RC posture

Self-hosted core monitoring is no longer sold by monitored-system count on the
current public v6 plans.

| Plan | Core monitoring | Metric history | Paid value |
|---|---|---:|---|
| Community | Included | 7 days | Full self-hosted monitoring |
| Relay | Included | 14 days | Remote web access, Pulse Mobile pairing for handoff, push, and convenience |
| Pro | Included | 90 days | Relay plus AI operations, automation, and advanced admin features |

Legacy `Pro+` remains continuity-only for existing holders. It is not a public
self-hosted checkout tier.

### Existing paid customer continuity is explicit

- Existing lifetime customers remain valid, with self-hosted monitoring volume
  not metered under the current v6 policy.
- Legacy recurring Pulse Pro subscribers who were already active before the
  public v6 pricing cutover keep their existing recurring price while that
  subscription stays active.
- Supported legacy paid migrations can still exchange into the v6 activation
  model without repurchasing.
- If a self-hosted v6 install still shows a bounded monitored-system cap after
  activation or migration, treat that as a bug rather than intended policy.

### Commercial account and upgrade surfaces match the current model

Pulse Account, the in-product `Plans & Billing` surface, and related pricing
copy now describe self-hosted upgrades as plan selection plus paid extras
instead of buying more monitored-system capacity.

### Pulse Cloud launches with v6

Pulse Cloud is the hosted version of Pulse. Each Cloud account gets a
dedicated, isolated workspace at `*.cloud.pulserelay.pro` with managed hosting,
daily automated backups, and Relay pre-configured for Pulse Mobile pairing. All
Cloud tiers include the full Pro feature set.

| Plan | Price | Monitored systems | Support |
|---|---|---:|---|
| Cloud Starter | $29/month or $249/year | 10 | Community |
| Cloud Power | $49/month or $449/year | 30 | Priority |
| Cloud Max | $79/month or $699/year | 75 | Priority |

A 14-day trial is included on every Cloud plan, no credit card required. An
early-signup founding rate of $19/month is available on Cloud Starter; see
`docs/architecture/v6-pricing-and-tiering.md` for current eligibility. Cloud
signup is handled at `cloud.pulserelay.pro`. Full Cloud setup, migration, and
FAQ live in `docs/CLOUD.md`. The MSP multi-tenant ladder remains a separate
hosted product; see the MSP section of the pricing doc.

## Upgrade Guidance For Existing v5 Users

1. Back up the current system and keep direct console access available.
2. Re-test bookmarks and saved links. The platform-shaped top-level pages
   (`/proxmox`, `/docker`, `/kubernetes`, `/truenas`, `/vmware`, `/standalone`)
   are the canonical v6 routes. The unified `/infrastructure`, `/workloads`,
   `/storage`, and `/recovery` routes that briefly shipped across `rc.1`-`rc.5`
   are retired in `rc.6` and onward; any bookmark or runbook pointing at those
   should move to the platform-shaped equivalent.
3. Re-test custom automation or dashboards that depended on v5-style
   `/api/state` or websocket payloads.
4. Re-test recovery workflows and any backup-era assumptions.
5. Verify license activation or paid-license migration immediately after first
   boot on upgraded systems.
6. Upgrade Unified Agents separately only when you are explicitly testing the
   v5-to-v6 agent path.

## Operator References

- `docs/UPGRADE_v6.md`
- `docs/releases/V6_CHANGELOG.md`
- `docs/PULSE_PRO.md`
