# Upgrade to Pulse v6

This guide covers practical upgrade steps for existing Pulse installs moving to v6.

For current stable release notes and rollout references, see:

- `docs/releases/RELEASE_NOTES_v6.md`
- `docs/releases/V6_CHANGELOG.md`

## Before You Upgrade

- Create an encrypted config backup: **Settings → System → Recovery → Create Backup** (older versions labeled this **Backups**)
- Confirm you can access the host/container console (for rollback and bootstrap token retrieval)
- If you have any external integrations or scripts: review the **API Changes** section below

## Upgrade Paths

### systemd and Proxmox LXC installs

Preferred path:

- **Settings → System → Updates**

If you prefer CLI, use the installed update helper for the target version:

```bash
sudo /bin/update --version vX.Y.Z
```

`/bin/update` is installed by the supported systemd and Proxmox LXC server installer. If your host does not have it yet, follow the signed server-installer flow in [INSTALL.md](INSTALL.md). Agent updates still use the `/install.sh` command generated in **Settings → Infrastructure → Install on a host**.

Operator note for builds after `v6.0.0-rc.2`: the historical Pulse update
signer was not recovered. Hosts pinned to the `rc.2` trust root should not
assume unattended continuity into newer prerelease or GA artifacts; plan a
manual reinstall or other explicit trust migration before testing those builds.

### Docker

```bash
docker pull rcourtman/pulse:vX.Y.Z
docker compose up -d
```

### Kubernetes (Helm)

```bash
helm repo update
helm upgrade pulse pulse/pulse -n pulse
```

## Post-Upgrade Checklist

- Confirm version: `GET /api/version`
- Confirm scheduler health: `GET /api/monitoring/scheduler/health`
- Confirm unified resources API is responding: `GET /api/resources`
- Confirm nodes are polling and no breakers are stuck open
- Confirm notifications still send (send a test)
- Confirm agents are connected (if used)

## v5 to v6 Operator FAQ

### Do I need to uninstall Pulse v5 first?

No. Upgrade the existing Pulse server installation in place.

### Do I need to uninstall my existing Pulse Unified Agents first?

No. Use the unified installer to upgrade existing agent deployments in place.

### Does upgrading the Pulse server to v6 automatically upgrade my agents?

No. The server upgrade and the Unified Agent upgrade are separate operations.
After the server is on v6, use the generated install or upgrade command from
**Settings → Infrastructure → Install on a host** when you want to move agents
to v6.

### Will an upgraded v5 agent keep the same identity in v6?

Yes. The v5-to-v6 agent path is expected to preserve one canonical agent
identity rather than creating a duplicate record during the upgrade.

### Do I need new agent tokens just because the server moved to v6?

No. Existing installed agents are expected to continue through the v6
compatibility boundary for legacy persisted agent scopes.

### Can one installed Pulse Unified Agent report to both a Pulse v5 instance and a Pulse v6 instance at the same time?

Not as a supported in-place setup. A running Unified Agent installation is
configured against one Pulse URL and one token, and it fetches remote config
from that one Pulse server. If you need side-by-side evaluation, use a
separate test host or VM, a cloned lab machine, or a separate isolated agent
installation instead of trying to point one running agent service at two Pulse
servers.

### Can I keep Pulse v5 stable while I test Pulse v6?

Pulse v5 entered maintenance-only support on `2026-04-20` and remains eligible
only for critical maintenance fixes until `2026-07-19`.

If you want extra caution, use a staging or otherwise controlled upgrade first
and keep a rollback path available, but v6 is now the supported stable line.

## Migration Notes (v6)

### Unified Navigation (Bookmarks and Deep Links)

Legacy page aliases have been removed. Use canonical unified routes only.

- Reference: `docs/MIGRATION_UNIFIED_NAV.md`
- Optional migration aid: enable the "Classic platform shortcuts" bar (Settings → System → General).
- Optional preference: switch to **Classic** navigation style (Settings → System → General). This is stored per browser.

### API Changes

Unified Resources is now the canonical model and endpoint family:

- Canonical: `/api/resources`

### License and Entitlements

Pulse v6 feature gating is driven by the entitlements endpoint:

- `GET /api/license/entitlements`

For self-hosted v6, Pulse no longer sells monitored-system volume. Core
monitoring stays available across Community, Relay, and Pro, while Relay and
Pro sell convenience, history, AI operations, and advanced administration.
Relay raises history to 14 days, while Pro raises it to 90 days.

Self-hosted v6 does not expose a general in-app trial, trial-return callback,
or hosted AI quickstart path. Ordinary upgraded self-hosted installs should use
activation, recovery, or BYOK/local AI setup instead; any exceptional
support-issued entitlement is reflected through hosted entitlement state rather
than a local in-app trial acquisition flow.

#### v5 License Migration

Pulse v6 uses the activation/grant model for active licensing, but it can migrate valid Pulse v5 paid JWT-style licenses, including legacy Pro and Lifetime licenses.

- If you upgrade an existing v5 instance and Pulse finds a persisted v5 license with no v6 activation state yet, v6 will try to auto-exchange it on startup.
- If auto-exchange cannot complete, your old key is left in place and the instance will prompt you to retry activation manually.
- In the v6 license panel, you can paste either:
  - a Pulse v6 activation key, or
  - a valid Pulse v5 paid license key, which Pulse will try to exchange automatically into the v6 activation model
- If the exchange service cannot complete the migration, retry from the v6 license panel or use the self-serve retrieval flow to fetch the current v6 activation key. Email is only a backup copy of that key.
- The exchanged v6 entitlement depends on the original cohort. Lifetime,
  active pre-cutover recurring Pro, and other migrated legacy paid installs do
  not all land on the same commercial continuity posture.
- Legacy recurring Pulse Pro subscriptions already active before the public v6 pricing cutover keep their grandfathered recurring price and uncapped monitored-system and guest capacity until cancellation. If they cancel and later return, current v6 pricing applies.

#### Paid Upgrade Truth Table

When an existing paid user asks what changes for them specifically, use this rule set:

- Legacy recurring Pulse Pro subscriptions from v5 or earlier that were already active before the public v6 pricing cutover keep their current recurring price and uncapped monitored-system plus guest capacity while the subscription remains continuously active.
- Existing lifetime customers remain permanently valid and uncapped.
- Legacy paid v5 licenses migrated into v6 outside the recurring grandfathered path can still exchange into the v6 activation model without repurchasing. Migration records can preserve the original cohort for support and audit, but self-hosted monitoring volume is no longer the paid gate.
- Former recurring customers who already canceled, or who cancel and later return, do not resume the old grandfathered pricing or uncapped capacity automatically; they re-enter on current public v6 pricing.
- New self-hosted v6 purchases use the current Community / Relay / Pro plan model with unlimited core monitoring.

If a self-hosted v6 install sees a new monitored-system cap after moving to v6, treat that as a regression, not as expected upgrade behavior.

Practical recommendation:

- Before upgrading, keep console access available so you can retry activation from the v6 license panel if the exchange service is temporarily unavailable.

### Multi-Tenant (Opt-In)

Multi-tenant mode is opt-in and additionally license-gated:

- Enablement flag: `PULSE_MULTI_TENANT_ENABLED=true`
- Capability gate: `multi_tenant`

See any multi-tenant operational docs under `docs/architecture/` if you plan to run this mode.
