# Pulse v6.2.2-rc.1

_This changelog describes the `v6.2.2-rc.1` release candidate compared with
stable `v6.2.1`._

## Added

- Canonical resource monitoring policy with pause and resume controls across
  runtime, API, frontend, Assistant, and MCP surfaces.
- Optional Relay CONNECT notification preferences for server-enforced mobile
  push delivery while preserving legacy frame compatibility.
- Patrol findings in alert email notifications.
- Categorized post-update changelog presentation from the published release
  body.
- An optional exact-SHA external amd64 preflight ahead of hosted release
  workflow dispatch.

## Changed

- Large Proxmox poll cycles preserve deferred backup, replication, storage, and
  guest work within explicit cycle budgets.
- Synchronous metrics writes use a bounded queue, shared metrics directories
  enforce safe ownership, and connection staleness follows adaptive cadence.
- Per-guest backup and snapshot alert toggles inherit global thresholds.
- Existing API tokens can be renamed; global settings project to non-admin
  viewers without mutation access.
- Normal WebSocket shutdowns are informational, offline agents remain removable
  after restart, and removed unified hosts clear their Docker alerts.
- Proxmox agent install commands support an explicit insecure-TLS choice and PBS
  profiles omit irrelevant PVE-linkage guidance.

## Fixed

- Tenant-scoped service discovery executes in the requested organization.
- Agent-reported LXC filesystem usage and configured mount points reach the
  normal cluster resource path.
- Docker image update checks route Pulse images through product update logic and
  tolerate registries without digest response headers.
- First-run systemd installs persist authentication state correctly.
- Patrol attention freshness ignores evidence-timing noise, and retained alert
  delivery notices explain their state.

## Security

- Configuration export and import require dedicated, expiring, principal- and
  organization-bound transfer authorization and reject replay.
- Proxy-auth role gating fails closed when a role header is configured without
  an explicit administrator role; matching is exact and case-sensitive.
- Audit signatures use unambiguous field framing and retain verification
  compatibility for existing historical records.

## Release Metadata

- Version: `v6.2.2-rc.1`
- Previous stable: `v6.2.1`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease without moving stable or latest pointers
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.2.2` restores mandatory SignPath signing
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile `1.0.0`
  iOS build `12` and Android versionCode `9` remain compatible because the new
  CONNECT preferences are optional, and no companion upload or public store
  rollout is required
