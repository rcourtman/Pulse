# Pulse v6.0.0-rc.2 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the shipped `v6.0.0-rc.1` tag. Do not treat it as published until the
governed `v6.0.0-rc.2` prerelease exists._

## What `rc.2` changes compared with `rc.1`

`v6.0.0-rc.2` is a corrective RC. It does not try to reopen the whole v6 shape.
It tightens the areas where `rc.1` created the most avoidable friction for
early testers.

## Major changes

### 1. Self-hosted core monitoring is no longer paywalled by monitored-system count

Community, Relay, and Pro now keep self-hosted core monitoring outside
monitored-system paid gating.

The current self-hosted paid model is:

- Community:
  free core self-hosted monitoring, 7-day history
- Relay:
  core self-hosted monitoring, 14-day history, remote access/Pulse Mobile handoff/push
- Pro:
  core self-hosted monitoring, 90-day history, AI operations, automation,
  and advanced admin features

Legacy `Pro+` remains continuity-only for existing holders. It is not a public
self-hosted checkout tier for the current core-monitoring-included model.

### 2. Paid-customer continuity is stricter and less surprising

`rc.2` carries the fixes that make the actual continuity policy match the
intended policy:

- lifetime licenses are not metered by self-hosted monitoring volume
- active legacy recurring v5 Pulse Pro continuity is not metered by
  self-hosted monitoring volume while the
  subscription stays active
- other supported legacy paid migrations can still exchange into v6 without
  losing self-hosted monitoring access
- stale self-hosted capped entitlements are normalized back to the current
  core-monitoring-included contract on refresh

### 3. Billing, plan, and account surfaces now match the core-monitoring-included model

The product and Pulse Account no longer frame self-hosted upgrades as buying
more monitored-system capacity.

Instead, the self-hosted paid story is now consistently:

- Relay:
  remote web access, Pulse Mobile pairing for handoff, push, and convenience
- Pro:
  Relay plus AI operations, automation, governance, and longer history

The core-monitoring-included model now flows through the local billing plan surface, Pulse
Account handoff, and the shared plan-definition owners rather than being only a
docs promise.

### 4. Monitoring-capacity messaging is less misleading where finite continuity still matters

For the remaining bounded fallback, hosted, MSP, or legacy-continuity cases,
the product now treats monitored-system overage as an admission-freeze state
instead of presenting it like a hard runtime blackout. Existing monitoring
continues, and any finite admission freeze applies only to those explicit
policy contexts. It is not a public self-hosted capacity upsell.

For the normal current self-hosted Community / Relay / Pro path, the plan
surface now presents core monitoring as included by default instead of carrying
stale cap-era UI chrome.

### 5. Two early `rc.1` regressions are fixed

- `pulse-agent --version` now exits cleanly instead of surfacing a misleading
  unified-agent configuration error during reinstall or quick CLI checks.
- Proxmox settings deep links now stay aligned with the selected platform table,
  so `PVE`, `PBS`, and `PMG` no longer collapse back to `PVE` after reload or
  remount.

## What existing v5 users should re-test in `rc.2`

1. Paid-license continuity after upgrade:
   - lifetime
   - active recurring legacy subscriptions
   - other supported paid-license migrations
2. Pulse Account self-hosted upgrade/purchase handoff.
3. Proxmox `Platform Connections` navigation for `PVE`, `PBS`, and `PMG`.
4. The v5-to-v6 unified-agent path, especially reinstall/version checks.
5. Any runbooks, expectations, or screenshots that were influenced by the
   old capped self-hosted story from `rc.1`.

## Evidence Appendix

For the code-backed evidence packet that maps these claims to the current
release line, see:

- `docs/release-control/v6/internal/records/v6-rc2-changelog-evidence-2026-04-16.md`
