# Pulse v6.0.0-rc.2 Draft Release Notes

_Draft only. Do not treat this as published until the governed `v6.0.0-rc.2`
tag and GitHub prerelease exist._

`v6.0.0-rc.2` is intended to be the first corrective RC after the public
`rc.1` release. Pulse v5 remains the current stable line.

The purpose of this RC is not to expand the product again. It is to correct the
main sources of friction that `rc.1` exposed in real user feedback:

- self-hosted monitored-system caps should not be the v6 monetization boundary
- existing paid customer continuity must be unambiguous and uncapped where
  promised
- the product and account surfaces should explain the current commercial model
  clearly instead of carrying stale cap-era copy
- early RC regressions in platform settings and agent CLI behavior should be
  corrected before broader retesting

## Support Stance

- Pulse v5 remains the current stable line.
- Pulse v6 `rc.2` is still an opt-in evaluation build, not the default
  production recommendation.
- Existing v5 users should still prefer staging, lab, or otherwise controlled
  evaluation first.

## What Changed Since `rc.1`

### Self-Hosted Monitoring Is No Longer Capped

Self-hosted core monitoring is no longer sold by monitored-system count.

Current self-hosted v6 packaging is:

| Plan | Core monitoring | Metric history | Paid value |
|---|---|---:|---|
| Community | Unlimited | 7 days | Full self-hosted monitoring |
| Relay | Unlimited | 14 days | Remote access, mobile, push, and convenience |
| Pro | Unlimited | 90 days | Relay plus AI operations, automation, and advanced admin features |

Legacy `Pro+` remains continuity-only for existing holders. It is not a public
no-cap self-hosted checkout tier.

### Existing Paid Customer Continuity Is Explicit

- Existing lifetime licenses remain valid and uncapped.
- Legacy recurring Pulse Pro subscribers who were already active before the
  public v6 pricing cutover remain uncapped while that subscription stays
  active.
- Supported legacy paid migrations can still exchange into the v6 activation
  model without losing self-hosted monitoring access.
- If a self-hosted v6 install still shows a bounded monitored-system cap after
  activation or migration, treat that as a bug rather than intended policy.

### Billing and Upgrade Surfaces Match the No-Cap Model

The local billing plan surface, Pulse Account upgrade handoff, and related
pricing copy now describe self-hosted upgrades as plan selection plus paid
extras rather than buying more monitored-system capacity.

For current self-hosted plans, the product now presents:

- unlimited core monitoring
- Relay as the remote/mobile convenience tier
- Pro as the AI/admin/history tier
- legacy continuity only where legacy continuity really applies

### Bug Fixes Called Out From Early RC Feedback

- Fixed `pulse-agent --version` so reinstall and CLI version checks exit
  cleanly instead of surfacing a misleading unified-config failure.
- Fixed Proxmox settings deep-link selection so `PVE`, `PBS`, and `PMG` routes
  stay aligned with the selected table after reload/remount.

## What Existing v5 Users Should Re-Test In `rc.2`

1. Paid-license continuity after upgrade or migration:
   - lifetime
   - active recurring legacy subscribers
   - other supported legacy paid migrations
2. Self-hosted upgrade and purchase handoff through Pulse Account.
3. Proxmox `Platform Connections` navigation across `PVE`, `PBS`, and `PMG`.
4. The v5-to-v6 unified-agent path, especially reinstall/version checks and
   normal in-place agent updates.
5. Any old runbooks or expectations that still assume monitored-system caps are
   part of the self-hosted commercial story.

## Feedback

Use the `Pulse v6 pre-release feedback` issue template for regressions, upgrade
failures, licensing continuity problems, platform-specific breakage, or
actionable UX friction:

- `https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml`

When reporting an `rc.2` problem, include:

- Pulse version
- upgrade path or fresh-install path
- installation type
- license cohort
- what you expected
- what happened instead
- sanitized logs, screenshots, or diagnostics when helpful

## Operator References

- `docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/PULSE_PRO.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
