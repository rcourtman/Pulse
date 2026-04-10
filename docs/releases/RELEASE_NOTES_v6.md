# Pulse v6.0.0-rc.1 Release Candidate Notes

`v6.0.0-rc.1` is the first public release candidate for Pulse v6.

This build is intended for evaluation and early deployment testing before the stable `v6.0.0` release. Pulse v5 remains the current stable line. If you rely on Pulse in production today, start with a staging or non-critical environment first and keep a rollback path available.

## Before You Try It

- Do not make your first v6 test on a production Pulse v5 installation.
- Prefer a lab, staging instance, or separate non-critical install for your first RC pass.
- Keep current backups and direct console access available before upgrading.
- Treat this RC as real release-candidate software: suitable for evaluation, but not yet the default recommendation for broad production rollout.

## What This RC Is For

- validating the v5 to v6 upgrade path
- checking first-session navigation and onboarding
- testing unified-agent continuity after the server upgrade
- exercising Pulse Pro activation and v5 Pro or Lifetime migration
- surfacing regressions, broken flows, and rough edges before GA

## Upgrade FAQ

### Do I need to uninstall Pulse v5 first?

No. Test Pulse v6 as an upgrade, not as a tear-down-and-rebuild exercise.

### Does upgrading the Pulse server to v6 automatically update my unified agents?

No. The server upgrade and unified-agent upgrade are separate.

If you install Pulse v6, your existing agents do not all switch to v6 automatically just because the server changed.

### If I want to test the full v6 agent path, what should I do?

After upgrading the server, update existing agents separately using the command generated from:

`Settings -> Unified Agents -> Installation commands`

That is the supported v5-to-v6 crossover path for agent testing.

### Do I need to uninstall existing v5 agents before updating them?

No. Existing v5 unified agents should be upgraded in place when testing them against a v6 server.

### Will an upgraded v5 agent keep the same identity in v6?

Yes. The supported v5-to-v6 agent path is intended to preserve one canonical agent identity rather than duplicating the machine during upgrade.

### Can one installed Pulse Unified Agent report to both a Pulse v5 instance and a Pulse v6 instance at the same time?

Not as a supported in-place setup. A running Unified Agent installation is configured against one Pulse URL and one token. If you need side-by-side evaluation, use a separate test host or VM, a cloned lab machine, or a separate isolated agent installation instead of trying to point one running agent service at two Pulse servers.

### What about Pulse Pro licensing?

If you already have a valid Pulse v5 Pro or Lifetime license, Pulse v6 can migrate it into the v6 activation model.

If the automatic exchange does not complete, retry from the v6 license panel. You can enter either:

- a Pulse v6 activation key
- a valid Pulse v5 Pro or Lifetime key for migration

### Will my old bookmarks and familiar pages still work?

Not necessarily. v6 reorganizes the product around Dashboard, Infrastructure, Workloads, Storage, and Recovery, and legacy page aliases have been removed.

If you rely on old bookmarks or runbooks, expect to update them.

## Feedback

Use the `Pulse v6 pre-release feedback` issue template for bugs, regressions, upgrade failures, performance issues, or actionable UX friction:

- `https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml`

When you report something, include:

- Pulse version
- install path
- installation type
- OS or environment
- license tier
- what you expected
- what happened instead
- sanitized logs, screenshots, or diagnostics if they help

## More Detail

If you want the operator-facing migration and upgrade details, use these docs:

- `docs/UPGRADE_v6.md`
- `docs/PULSE_PRO.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
