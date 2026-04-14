# Upgrade to Pulse v6

This guide covers practical upgrade steps for existing Pulse installs moving to v6.

For launch-time rollout answers and canned support responses, see:

- `docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md`

## Before You Upgrade

- Create an encrypted config backup: **Settings → System → Recovery → Create Backup** (older versions labeled this **Backups**)
- Confirm you can access the host/container console (for rollback and bootstrap token retrieval)
- If you have any external integrations or scripts: review the **API Changes** section below

## Upgrade Paths

### systemd and Proxmox LXC installs

Preferred path:

- **Settings → System → Updates**

If you prefer CLI, use the official installer for the target version:

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | \
  sudo bash -s -- --version vX.Y.Z
```

This installer updates the **Pulse server**. Agent updates use the `/install.sh` command generated in **Settings → Unified Agents → Installation commands**.

### Docker

```bash
docker pull rcourtman/pulse:latest
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
**Settings → Unified Agents → Installation commands** when you want to move
agents to v6.

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

Yes. That is the recommended RC posture. Keep v5 as your stable line and test
v6 first on a staging or non-critical install with a rollback path available.

## Migration Notes (v6)

### Unified Navigation (Bookmarks and Deep Links)

Legacy page aliases have been removed. Use canonical unified routes only.

- Reference: `docs/MIGRATION_UNIFIED_NAV.md`
- Optional migration aid: enable the "Classic platform shortcuts" bar (Settings → System → General).
- Optional preference: switch to **Classic** navigation style (Settings → System → General). This is stored per browser.

### API Changes

Unified Resources is now the canonical model and endpoint family:

- Canonical: `/api/resources`

### License, Trial, and Entitlements

Pulse v6 feature gating is driven by the entitlements endpoint:

- `GET /api/license/entitlements`

For self-hosted v6, Pulse now sells monitored coverage by monitored system rather than by installed agent. Community includes 5 monitored systems, Relay includes 8, Pro includes 15, and Pro+ includes 50. Relay also raises history to 14 days, while Pro and Pro+ raise it to 90 days.

For self-hosted v6, `POST /api/license/trial/start` initiates hosted signup by returning `409 trial_signup_required` with a hosted `action_url` rather than minting a local trial directly. Duplicate start attempts stay on the hosted-signup retry burst until it is exhausted, then return canonical `429 trial_rate_limited` plus `Retry-After` backoff metadata. Pulse only reflects trial lifecycle entitlements after the hosted control plane returns a signed activation token to `/auth/trial-activate`.

If you are upgrading an existing free instance that already exceeds the new Community cap, Pulse should not hard-break monitoring on rollout day. During grace, existing monitoring continues and only newly added counted systems are blocked until you remove systems or upgrade.

#### v5 License Migration

Pulse v6 uses the activation/grant model for active licensing, but it can migrate valid Pulse v5 Pro and Lifetime JWT-style licenses.

- If you upgrade an existing v5 instance and Pulse finds a persisted v5 license with no v6 activation state yet, v6 will try to auto-exchange it on startup.
- If auto-exchange cannot complete, your old key is left in place and the instance will prompt you to retry activation manually.
- In the v6 license panel, you can paste either:
  - a Pulse v6 activation key, or
  - a valid Pulse v5 Pro/Lifetime license key, which Pulse will try to exchange automatically
- If the exchange service cannot complete the migration, retry from the v6 license panel or use the self-serve retrieval flow to fetch the current v6 activation key. Email is only a backup copy of that key.
- Existing active recurring v5 customers keep their grandfathered recurring price and uncapped monitored-system and guest capacity until cancellation. If they cancel and later return, current v6 pricing and limits apply.

Practical recommendation:

- Before upgrading, keep console access available so you can retry activation from the v6 license panel if the exchange service is temporarily unavailable.

### Multi-Tenant (Opt-In)

Multi-tenant mode is opt-in and additionally license-gated:

- Enablement flag: `PULSE_MULTI_TENANT_ENABLED=true`
- Capability gate: `multi_tenant`

See any multi-tenant operational docs under `docs/architecture/` if you plan to run this mode.
