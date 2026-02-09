# Changelog Draft - Delete After Release

## Pulse v2.0 — Release Highlights

### New Features
- **Unified Resource Model (W1)**: Single API surface for all infrastructure resources. New `/api/v2/resources` endpoints. Legacy compatibility facade retained.
- **TrueNAS Support (W2)**: First-party TrueNAS storage array integration. Setup, discovery, polling, ZFS health mapping, alerts, and AI context. Feature flag: `PULSE_ENABLE_TRUENAS`.
- **Mobile App (W3)**: Companion app with QR pairing, relay protocol, push notifications, approval workflows, and biometric lock.
- **Multi-Tenant (W4)**: Secure multi-org isolation with per-tenant RBAC, storage, monitoring, user limits, and operational kill-switch. Feature flag: `PULSE_MULTI_TENANT_ENABLED`.
- **Conversion Pipeline (W5)**: In-app conversion event tracking, dynamic upgrade reasons, trial lifecycle, and real usage in limit displays.
- **Hosted Mode (W6)**: Private beta. Tenant provisioning, billing-state API, lifecycle management, observability. Feature flag: `PULSE_HOSTED_MODE`. Posture: GO_WITH_CONDITIONS (private_beta).

### Monetization Foundation (W0)
- Entitlement primitives: capabilities, limits, meters, plan_version, subscription_state
- Canonical evaluator with legacy alias resolution
- Metering pipeline with windowed aggregation
- Subscription state machine (trial/active/grace/expired/suspended)
- CRL revocation cache with fail-open
- Frontend entitlement API endpoint

### Operator Notes
- **Feature flags**: `PULSE_ENABLE_TRUENAS`, `PULSE_MULTI_TENANT_ENABLED`, `PULSE_HOSTED_MODE` control major features
- **Kill-switches**: TrueNAS (delete connections via API), Multi-Tenant (env var + restart), Conversion (runtime API config), Hosted (env var + restart)
- **Incident severity**: All runbooks now use P1-P4 framework
- **No manual data migration required** for this release

## PVE Backup Visibility Fix (Issue #1139)

### Fixed
- **PVE Backups not showing (agent setup)**: Fixed an issue where local PVE backups weren't visible when nodes were set up via the unified agent (`--enable-proxmox`). The agent now grants the required `PVEDatastoreAdmin` permission automatically.

### Improved
- **Backup permission warnings**: The Backups page now detects and displays a warning banner when backup permission issues are detected, with instructions on how to fix them. Users no longer have to guess why backups aren't appearing.

### Action Required for Existing Users
If you set up PVE nodes via the unified agent before this release, backups may not appear. The Backups page will now show a warning banner with the fix command, but you can also run this manually on each Proxmox host:

```bash
pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin
```

Alternatively, delete the node in Pulse Settings and re-run the agent setup - it will now grant the correct permissions.

---

**Commits:**
- 316a5629 fix(agent): grant PVEDatastoreAdmin for backup visibility
- 3237a4d7 docs: clarify PVE backup permission requirements
- 1733bea1 feat(ui): show backup permission warnings on Backups page
- 896b5bfc Fix: enable backup monitoring for PVE instances via config migration (already in main)

---

## Program Closeout Tracks

### Architecture and Platform Changes

- **Alerts**: Unified resource model hardening is complete. Alerts now work consistently across all resource types (VMs, containers, nodes, hosts, Docker, Kubernetes, PBS, PMG, and storage) using canonical resource type mapping.
- **Control Plane**: API routing was decomposed from monolithic `router.go` into modular route groups (auth, monitoring, AI, org, config), improving maintainability and enabling more independent test coverage.
- **Settings**: The Settings control plane was decomposed from a monolithic component into modular panels with registry-based dispatch. Feature gating, deep-linking, and navigation behavior were extracted into testable modules.
- **Multi-Tenant**: Multi-tenant mode is now fully productized as an opt-in capability behind feature flag and license gate. Single-tenant users get zero multi-tenant UI/behavior. Tenant isolation coverage spans API, WebSocket, alerts, AI, and settings surfaces.
- **Security**: Comprehensive tenant isolation replay was completed across API, WebSocket, and monitoring layers, including cross-org access prevention validation.
- **Bug Fix**: Fixed legacy guest alert ID migration in `LoadActiveAlerts()`, ensuring old-format IDs migrate correctly on startup.
- **PBS Backup Cache**: Terminal PBS datastore errors (404, 400) now correctly clear stale cached backups instead of preserving them indefinitely. Transient errors (500, timeout) still preserve cached data.

### Operator Notes

- **Multi-tenant controls**: Multi-tenant behavior is controlled by `PULSE_MULTI_TENANT_ENABLED` and enforced by license gate (`multi_tenant`).
- **Alert migration behavior**: Legacy alert IDs migrate automatically during startup. Migration is transparent to operators.
- **Settings routing compatibility**: Deep-link URLs are preserved. Legacy routes such as `/proxmox`, `/hosts`, and related aliases redirect to canonical settings paths.
- **Migration requirements**: No manual data migration is required for the closeout tracks in this release.

### Deferred/Follow-up (Tracked in Debt Ledger)

- Storage + Backups V2 remains deferred to the next milestone.
