# Pulse v6.0.0-rc.1 Notes

`v6.0.0-rc.1` is a low-key Pulse v6 release candidate for users who want to test early and report issues before the stable `v6.0.0` release.

This note is intentionally brief. I will publish the fuller Pulse v6 release notes with the final GA release.

## Before You Try It

- I do not recommend upgrading a production Pulse v5 installation yet.
- Pulse v5 remains the current stable line during the v6 RC period.
- If you want to test v6, use a staging instance, lab environment, or separate non-production install first.
- Keep console access and a current backup available before upgrading.

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

### What about Pulse Pro licensing?

If you already have a valid Pulse v5 Pro or Lifetime license, Pulse v6 can migrate it into the v6 activation model.

If the automatic exchange does not complete, retry from the v6 license panel. You can enter either:

- a Pulse v6 activation key
- a valid Pulse v5 Pro or Lifetime key for migration

### Will my old bookmarks and familiar pages still work?

Not necessarily. v6 reorganizes the product around Dashboard, Infrastructure, Workloads, Storage, and Recovery, and legacy page aliases have been removed.

If you rely on old bookmarks or runbooks, expect to update them.

## What Feedback Is Most Useful

- v5 to v6 upgrade friction
- first-session onboarding and navigation
- unified-agent update experience after server upgrade
- Pulse Pro activation or v5 license migration
- regressions, broken flows, or anything that feels unreliable

## More Detail

If you want the operator-facing migration and upgrade details, use these docs:

- `docs/UPGRADE_v6.md`
- `docs/PULSE_PRO.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
