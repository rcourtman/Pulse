# Pulse v6 RC2 Draft Operator Support Pack

_Draft only. Use this as the working support brief for the planned
`v6.0.0-rc.2` candidate until the final prerelease notes are published._

## Support Stance

- Pulse v5 remains the current stable line.
- Pulse v6 `rc.2` is an opt-in evaluation build, not the default production
  recommendation.
- `rc.2` should be described as a corrective RC: it keeps the v6 evaluation
  posture, but it removes the main self-hosted cap friction from `rc.1` and
  fixes a small set of real regressions found during early testing.

## Short Answers

### Do I need to uninstall Pulse v5 first?

No. Upgrade the existing Pulse server installation in place.

### Do I need to uninstall existing Pulse Unified Agents first?

No. Existing Unified Agents should still be upgraded in place when testing them
against a v6 server.

### Does upgrading the Pulse server to v6 automatically upgrade my agents?

No. Server upgrade and Unified Agent upgrade remain separate operations.

### Does self-hosted v6 still cap monitored systems?

No for the current public self-hosted plans. Community, Relay, and Pro include
core self-hosted monitoring by default.

Monitored systems still matter for product understanding, migration truth, and
inventory language, but they are no longer the self-hosted paid gate.

### What are the current self-hosted paid plans in `rc.2`?

- Community:
  core monitoring included, 7-day history
- Relay:
  Community plus remote access, mobile, push notifications, and 14-day history
- Pro:
  Relay plus AI operations, automation, advanced admin features, and 90-day
  history

Legacy `Pro+` remains continuity-only for existing holders and should not be
described as a public self-hosted checkout tier.

### What happens to existing paid Pulse Pro customers in v6?

Use this cohort breakdown:

- Legacy recurring monthly or annual subscribers from v5 or earlier who were
  already active before the public v6 pricing cutover:
  keep the current recurring price, with self-hosted monitoring and
  child-resource volume not metered while the subscription remains continuously
  active under the current v6 policy.
- Existing lifetime customers:
  remain permanently valid, with self-hosted monitoring and child-resource
  volume not metered under the current v6 policy.
- Legacy paid v5 licenses migrated into v6 outside the recurring grandfathered
  path:
  can still exchange into the v6 activation model without repurchasing.
  Migration records can preserve the original cohort for support and audit, but
  self-hosted monitoring volume is no longer the paid gate.
- Former recurring subscribers who already canceled or later lapse:
  any later return uses current public v6 pricing rather than resuming the old
  grandfathered terms.

Canonical short reply:

`Current self-hosted v6 plans no longer charge by monitored-system count. Lifetime customers remain valid, and self-hosted monitoring volume is no longer the paid gate. Legacy recurring Pulse Pro subscribers who were already active before the public v6 pricing cutover keep their existing recurring price while that subscription stays active. Other supported legacy paid installs can still exchange into the v6 activation model without repurchasing.`

### What if a self-hosted v6 install still shows a monitored-system cap?

Treat that as a product bug or continuity-state defect, not as intended current
policy for Community, Relay, or Pro.

### What changed from `rc.1` that users should notice immediately?

- self-hosted core monitoring is no longer commercially capped
- paid-customer continuity is clearer and less surprising
- Pulse Account and in-product billing surfaces now describe self-hosted
  upgrades as plan selection plus paid extras rather than buying more monitored
  systems
- `pulse-agent --version` no longer reports a misleading unified-config error
- Proxmox `PVE` / `PBS` / `PMG` settings routes stay aligned after reload

### Can I keep Pulse v5 stable while I test Pulse v6?

Yes. That remains the recommended RC posture.

## Recommended Evaluation Path

1. Back up the current system and keep direct console access available.
2. Upgrade the Pulse server in place on a staging or otherwise controlled
   environment.
3. Verify basic runtime health before touching agents.
4. Re-test paid-license continuity immediately after first boot if the system
   is a migrated or existing paid install.
5. Re-test Pulse Account upgrade/purchase handoff if the environment uses
   self-hosted paid features.
6. Re-test Proxmox `PVE` / `PBS` / `PMG` settings navigation if the deployment
   uses those platform connections.
7. Upgrade agents separately only if the user is explicitly testing the v5-to-v6
   agent path.

## Ask For These Details

When a user reports an `rc.2` problem, ask for:

- current version and prior version
- install type
- whether the issue happened during server upgrade, agent upgrade, licensing,
  account handoff, or first use
- license cohort
- whether this is a clean lab install or upgraded v5 install
- whether Unified Agents were upgraded yet
- expected result
- actual result
- sanitized logs, screenshots, and diagnostics

## Escalate Immediately

Escalate without asking the user to keep experimenting when the report involves:

- failed install or failed upgrade with no recovery path
- duplicate or missing agent identity after a v5-to-v6 upgrade
- monitoring or reporting that stops entirely after upgrade
- license migration failure that blocks paid functionality unexpectedly
- any current self-hosted Community / Relay / Pro install showing a bounded
  monitored-system cap as if that were normal policy
- rollback failure or inability to return to the previous stable state
- data-loss, destructive behavior, or security-sensitive regressions

## Canonical References

- `docs/releases/RELEASE_NOTES_v6_RC2_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC2_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/PULSE_PRO.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
