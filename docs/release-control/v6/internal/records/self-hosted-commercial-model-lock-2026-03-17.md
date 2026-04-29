# Self-Hosted Commercial Model Lock (Superseded)

Date: 2026-03-17
Target: `v6-rc-stabilization`
Lanes: `L2`, `L13`
Status: Superseded on 2026-04-16 by `self-hosted-core-monitoring-free`; this record now preserves the capped RC1 self-hosted model as historical context only and is not the final GA pricing direction.

Current replacement: Pulse v6 GA treats self-hosted core monitoring as included
for Community, Relay, and Pro. Relay is presented annual-first at `$39/year` with
`$4.99/month` as the secondary option, and it sells secure remote access to the
Pulse web UI, mobile app pairing, push notifications, and 14-day history rather
than monitored-system headroom or native mobile monitoring.

## Historical Decision

On 2026-03-17, Pulse v6 self-hosted commercial packaging was locked to this capped RC1 model:

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

The capped RC1 copy that originally lived here is intentionally not retained as
reusable wording. Current user-facing copy must come from the replacement policy
above and from the shared self-hosted plan definitions, where self-hosted core
monitoring is included and Relay is positioned around remote web access, mobile
app pairing, push notifications, and 14-day history.

## Implementation Slices

1. Runtime counting
   Replace agent-only commercial enforcement with monitored-system counting derived from canonical unified-resource roots and transport-agnostic deduplication.

2. Frontend/commercial UI
   Rename commercial copy from agents to monitored systems, replace the commercial ledger with counted-system truth, and update pricing/paywall language to the locked bands.

3. License server / checkout / public site
   Create the new self-hosted Stripe prices, update plan mappings and purchase flows, and cut all public pricing copy over to the monitored-system model without disturbing v5 grandfathered continuity.

## Implementation Transition

Explicit `legacy_v5` compatibility files may still decode older `max_agents` / `max_nodes` inputs at import boundaries. That is migration support, not the canonical commercial contract.
