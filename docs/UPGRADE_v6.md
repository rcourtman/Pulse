# Upgrade to Pulse v6

This guide covers practical upgrade steps for existing Pulse installs moving to v6.

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

If a trial is enabled and started, Pulse writes local billing-state (no phone-home) and entitlements reflect the trial lifecycle.

#### v5 License Migration

Pulse v6 uses the activation/grant model for active licensing, but it can migrate valid Pulse v5 Pro and Lifetime JWT-style licenses.

- If you upgrade an existing v5 instance and Pulse finds a persisted v5 license with no v6 activation state yet, v6 will try to auto-exchange it on startup.
- If auto-exchange cannot complete, your old key is left in place and the instance will prompt you to retry activation manually.
- In the v6 license panel, you can paste either:
  - a Pulse v6 activation key, or
  - a valid Pulse v5 Pro/Lifetime license key, which Pulse will try to exchange automatically
- If the exchange service cannot complete the migration, retrieve the migrated activation key from your Pulse account or contact support.

Practical recommendation:

- Before upgrading, keep console access and your original purchase email/license record available in case the exchange service is unavailable and you need the account fallback path.

### Multi-Tenant (Opt-In)

Multi-tenant mode is opt-in and additionally license-gated:

- Enablement flag: `PULSE_MULTI_TENANT_ENABLED=true`
- Capability gate: `multi_tenant`

See any multi-tenant operational docs under `docs/architecture/` if you plan to run this mode.
