# Upgrade to Pulse v6

This guide covers practical upgrade steps for existing Pulse installs moving to v6.

## Before You Upgrade

- Create an encrypted config backup: **Settings → System → Backups → Create Backup**
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

Legacy pages redirect into the unified navigation as compatibility aliases.

- Reference: `docs/MIGRATION_UNIFIED_NAV.md`
- To disable all legacy route redirects (and surface stale bookmarks immediately), set:
  - `PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS=true` or
  - `disableLegacyRouteRedirects: true` in `system.json`
- Optional migration aid: enable the "Classic platform shortcuts" bar (Settings → System → General).
- Optional preference: switch to **Classic** navigation style (Settings → System → General). This is stored per browser.

### API Changes

Unified Resources is now the canonical model and endpoint family:

- Canonical: `/api/resources`
- Deprecated alias (temporary): `/api/v2/resources`

If you have scripts/integrations calling `/api/v2/resources`, migrate them to `/api/resources`.

### License, Trial, and Entitlements

Pulse v6 feature gating is driven by the entitlements endpoint:

- `GET /api/license/entitlements`

If a trial is enabled and started, Pulse writes local billing-state (no phone-home) and entitlements reflect the trial lifecycle.

### Multi-Tenant (Opt-In)

Multi-tenant mode is opt-in and additionally license-gated:

- Enablement flag: `PULSE_MULTI_TENANT_ENABLED=true`
- Capability gate: `multi_tenant`

See any multi-tenant operational docs under `docs/architecture/` if you plan to run this mode.
