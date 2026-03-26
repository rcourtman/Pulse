# Self-Hosted Commercial Model Lock

Date: 2026-03-17
Target: `v6-rc-stabilization`
Lanes: `L2`, `L13`

## Decision

Pulse v6 self-hosted commercial packaging is locked to this model:

| Plan | Price | Included limit | History | Purpose |
|---|---:|---:|---:|---|
| Community | Free | 5 monitored systems | 7 days | One real small lab end to end |
| Relay | $4.99/mo or $39/yr | 8 monitored systems | 14 days | Cheap headroom plus remote access |
| Pro | $8.99/mo or $79/yr | 15 monitored systems | 90 days | Automation and operations tier |
| Pro+ | $14.99/mo or $129/yr | 50 monitored systems | 90 days | Larger self-hosted labs |

Cloud and MSP pricing are unchanged by this lock.

## Counted Unit

Pulse sells monitored coverage. The counted unit is a **monitored system**, not an installed agent.

One monitored system counts once regardless of collection path.

Counted examples:
- Proxmox PVE node
- PBS / PMG server
- standalone Linux / Windows / macOS host
- Docker host
- TrueNAS / Unraid system
- Kubernetes cluster

Not counted separately:
- VMs
- containers
- pods
- disks
- pools
- datastores
- backup jobs
- other child resources under a counted top-level system

Rules:
- API-backed monitoring and agent-backed monitoring consume the same cap
- If the same system is seen through both paths, it counts once
- Deduplication must follow canonical unified-resource identity, not transport-specific state

## Migration Policy

- Existing paid v5 customers keep their grandfathered recurring continuity until cancellation, per the existing governed policy
- Existing free users above the new Community cap must not be hard-broken on rollout day
- During grace, existing monitoring keeps working
- During grace, only new counted-system additions are blocked until the user removes systems or upgrades

## User-Facing Copy

Counted-unit explainer:

> Pulse counts monitored systems, not everything underneath them. Each top-level machine or cluster counts once, no matter how Pulse collects it. VMs, containers, pods, disks, backups, and services under that system are included.

Plan taglines:
- Community: Monitor up to 5 systems for free.
- Relay: Get a bit more room and monitor from anywhere.
- Pro: Pulse does not just watch your infrastructure. It helps operate it.
- Pro+: Everything in Pro, with more room for larger labs.

Boundary-upgrade copy:
- Community to Relay: Need a little more room? Upgrade to Relay for 3 extra monitored systems plus remote access, mobile, and push notifications.
- Relay to Pro: Want Pulse to do more than alert? Upgrade to Pro for AI investigation, auto-fix, and 90-day history.
- Grace copy: Your existing monitoring will keep working for now, but new systems will not be added until you remove one or upgrade.

## Implementation Slices

1. Runtime counting
   Replace agent-only commercial enforcement with monitored-system counting derived from canonical unified-resource roots and transport-agnostic deduplication.

2. Frontend/commercial UI
   Rename commercial copy from agents to monitored systems, replace the commercial ledger with counted-system truth, and update pricing/paywall language to the locked bands.

3. License server / checkout / public site
   Create the new self-hosted Stripe prices, update plan mappings and purchase flows, and cut all public pricing copy over to the monitored-system model without disturbing v5 grandfathered continuity.

## Implementation Transition

Explicit `legacy_v5` compatibility files may still decode older `max_agents` / `max_nodes` inputs at import boundaries. That is migration support, not the canonical commercial contract.
