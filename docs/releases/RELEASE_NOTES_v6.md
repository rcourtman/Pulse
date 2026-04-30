# Pulse v6.0.0 Release Notes

`v6.0.0` is the first stable release of Pulse v6. It promotes the validated
`v6.0.0-rc.1` and `v6.0.0-rc.2` line into the default supported v6 release.

Pulse v6 reorganizes the product around `Infrastructure`, `Workloads`,
`Storage`, and `Recovery`, keeps the governed v5-to-v6 upgrade and Unified Agent
continuity path, and ships the corrected self-hosted commercial model that was
validated during `rc.2`.

## Pulse v5 Support Transition

Pulse v5 entered maintenance-only support on `2026-04-20`.
I will ship only critical security, data-loss, licensing or billing blocker, installer or updater failure, and safe migration blocker fixes for existing v5 users until `2026-07-19`.
After `2026-07-19`, Pulse v5 is end-of-support and new fixes land on v6 unless
I publish an explicit exception.

## What Is In v6.0.0

### Unified v6 product layout

Pulse v6 changes the default product shape. Authenticated users now land on
`Infrastructure`, and the primary surfaces are:

- `Infrastructure`
- `Workloads`
- `Storage`
- `Recovery`

Existing bookmarks, old screenshots, and operator runbooks that assumed the v5
Proxmox-first layout should be reviewed during upgrade.

### Recovery and infrastructure are first-class

`Recovery` is now a primary surface rather than a backup-only page family, and
infrastructure onboarding is split by ownership:

- `Install on a host` for direct Unified Agent deployment
- `Platform connections` for API-backed systems such as Proxmox, TrueNAS, and
  VMware

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

## Upgrade Guidance For Existing v5 Users

1. Back up the current system and keep direct console access available.
2. Re-test navigation, bookmarks, and any saved links that depended on the old
   route structure.
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
- `docs/MIGRATION_UNIFIED_NAV.md`
