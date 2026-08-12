# Pulse v6.2.2-rc.1 Release Notes

`v6.2.2-rc.1` is a release candidate for the next Pulse v6 patch. It follows
stable `v6.2.1` and packages the accumulated security, monitoring-scale,
operator-control, agent, and update reliability work completed since that cut.

## Highlights

- Large Proxmox clusters retain backup, replication, storage, and guest work
  when a poll cycle reaches its budget.
- Resources gain canonical monitoring pause and resume controls, inherited
  backup thresholds, and restored LXC filesystem data.
- Configuration transfer, proxy-role authorization, and audit signatures now
  enforce stronger fail-closed boundaries.

## Added

- Canonical resource monitoring policy and operator-state controls across the
  resource drawer, alert surfaces, API, Assistant, and MCP adapter.
- Optional Relay CONNECT notification preferences for server-enforced mobile
  push delivery. Legacy mobile CONNECT frames remain compatible.
- Patrol findings in alert email notifications.
- A categorized post-update changelog using the published release body.

## Improved

- Metrics writes are bounded so slow disks cannot stall an entire monitoring
  cycle, while shared metrics directories reject unsafe ownership.
- Agent stale-state timing follows the adaptive planned poll interval rather
  than a fixed cutoff.
- Offline agents remain removable after a restart, Docker alerts clear when a
  unified host is removed, and normal WebSocket shutdowns no longer read as
  failures.
- API tokens can be renamed, first-run authentication persists correctly for
  systemd installs, and global settings remain visible to non-admin viewers
  without granting mutation authority.
- Proxmox agent installation can opt into insecure TLS explicitly, PBS agent
  profiles avoid irrelevant PVE-linkage warnings, and Docker image update
  checks tolerate registries that omit digest headers.
- The release triggers now run portable exact-SHA qualification on a configured
  external amd64 worker before allocating the hosted release workflow.

## Fixed

- Tenant-scoped service discovery executes under the selected organization.
- Backup, replication, storage, and guest polling no longer lose work on large
  Proxmox clusters.
- Per-guest backup and snapshot alert toggles correctly inherit their global
  threshold defaults.
- LXC mount points and filesystem usage reported by Pulse Agent reach the
  cluster resource API again.
- Patrol attention rows no longer receive false freshness from evidence-timing
  noise, and retained alert-delivery notices explain why they remain visible.
- Docker image update checks now route Pulse images through the product updater
  and handle registries without `Docker-Content-Digest` response headers.

## Security

- Configuration transfer endpoints require a dedicated authorization envelope,
  bind the operation to the authenticated principal and organization, expire
  promptly, and reject replay or cross-scope use.
- Proxy authentication no longer treats every authenticated user as an
  administrator when `PROXY_AUTH_ROLE_HEADER` is set without
  `PROXY_AUTH_ADMIN_ROLE`. Set the latter to the IdP administrator group name,
  matching case exactly; the `admin` fallback will not match names such as
  `Admins` or `authentik Admins`.
- Audit signatures use an unambiguous framed payload, while verification still
  accepts historical records written with the legacy encoding.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.2-rc.1` only when you are
comfortable testing a release candidate. The rollback target is `v6.2.1`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.2.1
```

Before upgrading a proxy-auth deployment that sets
`PROXY_AUTH_ROLE_HEADER`, configure `PROXY_AUTH_ADMIN_ROLE` explicitly. See
`docs/PROXY_AUTH.md` for the trusted-proxy boundary and examples.

Pulse Mobile `1.0.0` iOS build `12` and Android versionCode `9` remain
compatible. The new CONNECT preference object is optional, and legacy frames
retain the existing default push behavior. No companion build upload or public
mobile-store rollout is part of this candidate.

Windows Unified Agent binaries in this prerelease retain exact-SHA, checksum,
and detached-signature verification but are not Authenticode-signed, so Windows
may display an Unknown Publisher warning. Stable `v6.2.2` still requires the
normal SignPath Authenticode lane unless a separate version-bound owner decision
is recorded.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
