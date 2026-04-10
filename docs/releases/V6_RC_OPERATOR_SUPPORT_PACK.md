# Pulse v6 RC Operator Support Pack

This document is the launch-time support sheet for the first public Pulse v6
release candidate, `v6.0.0-rc.1`.

Use it when handling rollout questions from current Pulse v5 users, preparing
FAQ answers, or deciding how to position the v6 preview in public-facing
surfaces such as the demo site.

## Support Stance

- Pulse v5 remains the current stable line.
- Pulse v6 `rc.1` is an opt-in evaluation build, not the default recommendation
  for broad production rollout.
- Support answers should push users toward staging or non-critical first-run
  evaluation, not toward tearing down a healthy v5 install blindly.
- The first public answer should be clear and confident, not apologetic. Treat
  v6 as a real release candidate that needs validation, not as a throwaway
  preview that "probably breaks".

## Short Answers

### Do I need to uninstall Pulse v5 first?

No. Upgrade the existing Pulse server installation in place.

### Do I need to uninstall existing Pulse Unified Agents first?

No. Existing Unified Agents should be upgraded in place when testing them
against a v6 server.

### Does upgrading the Pulse server to v6 automatically upgrade my agents?

No. Server upgrade and Unified Agent upgrade are separate operations.

### Will an upgraded v5 agent keep the same identity in v6?

Yes. The supported v5-to-v6 path is expected to preserve one canonical agent
identity rather than duplicating the machine during upgrade.

### Do I need new agent tokens just because the server moved to v6?

No. Existing installed agents are expected to continue through the v6
compatibility boundary for legacy persisted agent scopes.

### Can one installed Pulse Unified Agent report to both a Pulse v5 instance and a Pulse v6 instance at the same time?

Not as a supported in-place setup. A running Unified Agent installation is
configured against one Pulse URL and one token, and it fetches remote config
from that one Pulse server. For side-by-side evaluation, use a separate test
host or VM, a cloned lab machine, or a separate isolated agent installation.

### Can I keep Pulse v5 stable while I test Pulse v6?

Yes. That is the recommended RC posture.

### What happens to a Pulse v5 Pro or Lifetime license?

Pulse v6 can migrate valid Pulse v5 Pro or Lifetime licensing into the v6
activation model. If the auto-exchange does not complete, retry from the v6
license panel using the existing key.

### Will old bookmarks and familiar v5 pages still work?

Not necessarily. v6 reorganizes the product around Dashboard, Infrastructure,
Workloads, Storage, and Recovery. Old platform-era aliases and runbook links
should be updated to canonical v6 routes.

## Recommended Evaluation Path

1. Back up the current system and keep direct console access available.
2. Upgrade the Pulse server in place on a staging or non-critical environment.
3. Verify basic runtime health before changing agents:
   - `GET /api/version`
   - `GET /api/monitoring/scheduler/health`
   - `GET /api/resources`
4. Upgrade agents separately only if the user is explicitly testing the v5 to
   v6 agent path.
5. If the user wants side-by-side comparison, use a separate lab install or
   isolated agent deployment rather than dual-pointing one live agent.

## Rollback Posture

- The governed rollback target for the first v6 RC is `v5.1.27`.
- If rollback is needed, pin the environment back to `v5.1.27` rather than
  using an unpinned `latest` flow.
- Support should treat rollback as a normal controlled outcome for RC
  evaluation, not as user failure.
- If rollback is required because of broken upgrade, duplicate agent identity,
  licensing regression, or major monitoring breakage, collect the failure
  details before the environment is discarded.

## Ask For These Details

When a user reports a v6 RC problem, ask for:

- current version and prior version
- install type: systemd/LXC, Docker, Kubernetes, or other
- whether the issue happened during server upgrade, agent upgrade, or first use
- license tier
- whether this is a clean lab install or an upgraded v5 install
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
- rollback failure or inability to return to the previous stable state
- data-loss, destructive behavior, or security-sensitive regressions

## Public Demo Guidance

If the public demo site continues to serve Pulse v5 while Pulse v6 is still in
RC:

- keep Pulse v5 as the default public demo while v5 remains the stable line
- expose Pulse v6 only as an explicit `Preview` or `RC` switch, not as the
  default path
- keep the v6 demo clearly labeled as mock-data preview behavior
- show release-notes and feedback paths for the v6 preview
- avoid presenting the v6 demo as if it were the current stable product

That posture lets users explore v6 without confusing the stable story or
creating the impression that the v5 demo silently changed underneath them.

## Canonical References

- `docs/UPGRADE_v6.md`
- `docs/releases/RELEASE_NOTES_v6.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
- `docs/PULSE_PRO.md`
