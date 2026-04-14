# Pulse v6.0.0-rc.1 changelog evidence

_Scope note: this appendix maps the public v6 changelog to shipped code in `pulse-5.1.x@v5.1.27` and `pulse@v6.0.0-rc.1`. It does not treat later work on `pulse/v6-release` as shipped._

_Line references below were taken from the reviewed tag snapshots for those two refs on 2026-04-14._

## 1. Top-level product layout changed from Proxmox-first to surface-first

- **Claim:** Pulse v6 replaces the v5 Proxmox-first layout with `Dashboard`, `Infrastructure`, `Workloads`, `Storage`, and `Recovery`.
- **Why it is true:** The v5 router sends `/` to `/proxmox/overview` and exposes Proxmox-first primary routes. The v6 router sends `/` to `/dashboard` and the v6 layout exposes the new top-level surfaces.
- **Key files:**
  - `pulse-5.1.x@v5.1.27 frontend-modern/src/App.tsx:967-984`
  - `pulse-5.1.x@v5.1.27 frontend-modern/src/App.tsx:1217-1221`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/App.tsx:416-435`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/AppLayout.tsx:284-383`
- **Surface:** User-visible, operator-visible.
- **Consequence:** Saved routes, screenshots, runbooks, and operator habits need re-testing.
- **Shipped-tag confidence:** High.

## 2. Recovery became a first-class surface

- **Claim:** V6 promotes recovery into its own primary surface instead of keeping the v5 backup-first page and route model as the main public shape.
- **Why it is true:** V5 registers `/api/backups*` routes and exposes backup-specific payloads. V6 exposes `/recovery` in the UI and registers dedicated `/api/recovery/*` monitoring routes. The live websocket/state contract also stops treating `backups` as a canonical payload field.
- **Key files:**
  - `pulse-5.1.x@v5.1.27 internal/api/router.go:377-381,6484-6512`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/routing/resourceLinks.ts:44-45`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_monitoring.go:26-29`
  - `pulse@v6.0.0-rc.1 internal/api/router_integration_test.go:1650-1652,1758-1760`
- **Surface:** User-visible, operator-visible, API-visible.
- **Consequence:** Backup and recovery workflows should be re-tested as a migration area, especially where custom tooling assumed the old backup routes or payload shape.
- **Shipped-tag confidence:** High.

## 3. Infrastructure setup is split into host install and platform connections

- **Claim:** V6 separates `Install on a host` from `Platform connections`, and API-backed systems use platform connections instead of the host-install path.
- **Why it is true:** The shipped settings and setup wizard explicitly split these paths and describe which platforms belong in each one.
- **Key files:**
  - `pulse@v6.0.0-rc.1 frontend-modern/src/components/Settings/infrastructureWorkspaceModel.ts:9-25`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/components/Settings/platformConnectionsModel.ts:18-33`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/components/SetupWizard/SetupCompletionPanel.tsx:47-72`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/components/Settings/InfrastructureInstallerSection.tsx:140-162`
- **Surface:** User-visible, operator-visible.
- **Consequence:** First-run testing and onboarding automation should be re-tested against the v6 ownership split.
- **Shipped-tag confidence:** High.

## 4. Cluster agent deployment is part of the shipped RC workflow

- **Claim:** V6 adds structured cluster deployment flows beyond v5's simpler manual install-command model.
- **Why it is true:** The shipped router and handlers include candidate discovery, preflight, job, event, cancel, and retry routes for agent deployment.
- **Key files:**
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_registration.go:640-682`
  - `pulse@v6.0.0-rc.1 internal/api/deploy_handlers.go:97-99,197-199,973-975`
- **Surface:** Operator-visible.
- **Consequence:** Cluster operators should test rollout and recovery paths directly instead of assuming only the single-node install flow matters.
- **Shipped-tag confidence:** High.

## 5. The live-state contract changed from v5 per-type arrays to v6 unified resources

- **Claim:** Existing automation that reads v5 `/api/state` or websocket payloads should expect rework in v6.
- **Why it is true:** V6 tests explicitly require the old top-level keys to be absent and require `resources` and `connectedInfrastructure` instead. The canonical backend model is now the unified resource model.
- **Key files:**
  - `pulse@v6.0.0-rc.1 tests/migration/v5_full_upgrade_test.go:266-291`
  - `pulse@v6.0.0-rc.1 internal/api/router_integration_test.go:1633-1652,1728-1760`
  - `pulse@v6.0.0-rc.1 internal/unifiedresources/types.go:11-68,106-152`
- **Surface:** Operator-visible, API-visible, architectural.
- **Consequence:** Custom dashboards, scripts, websocket consumers, and state parsers built for v5 should be treated as migration-sensitive.
- **Shipped-tag confidence:** High for shipped behavior. Medium-high for the "new in v6" framing because some unified-resource groundwork existed before the RC.

## 6. Canonical agent and resource naming changed, with legacy aliases treated as compatibility paths

- **Claim:** V6 changes the preferred naming from `host`/generic `container` toward `agent`, `system-container`, and `app-container`, and treats older host-oriented routes and aliases as compatibility paths.
- **Why it is true:** V6 frontend types and backend alias logic canonicalize the newer names, while the router exposes `/api/agents/agent/*` as canonical and wraps `/api/agents/host/*` in legacy alias handlers.
- **Key files:**
  - `pulse-5.1.x@v5.1.27 frontend-modern/src/types/resource.ts:11-32`
  - `pulse@v6.0.0-rc.1 frontend-modern/src/types/resource.ts:23-44,105-118`
  - `pulse@v6.0.0-rc.1 internal/unifiedresources/legacy_aliases.go:5-35`
  - `pulse-5.1.x@v5.1.27 internal/api/router.go:314-321`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_registration.go:56-68,114-115`
- **Surface:** Operator-visible, API-visible, architectural.
- **Consequence:** New integrations should use the v6 canonical names and routes; older assumptions should be reviewed rather than carried forward unchanged.
- **Shipped-tag confidence:** High.

## 7. V6 ships a real v5 upgrade path and a tenant-aware data layout

- **Claim:** The shipped RC includes compatibility-preserving upgrade behavior for real v5 installations, and the on-disk layout changes during upgrade.
- **Why it is true:** Migration tests cover config, encryption, sessions, metrics, audit history, and preserved settings. The config migration moves local state into `orgs/default` and leaves compatibility symlinks behind.
- **Key files:**
  - `pulse@v6.0.0-rc.1 tests/migration/v5_to_v6_test.go:157-257`
  - `pulse@v6.0.0-rc.1 tests/migration/v5_session_db_test.go:22-150,152-238,258-347,417-553`
  - `pulse@v6.0.0-rc.1 tests/migration/v5_full_upgrade_test.go:293-367,369-420`
  - `pulse@v6.0.0-rc.1 internal/config/migration.go:20-149`
  - `pulse@v6.0.0-rc.1 tests/migration/v5_to_v6_test.go:353-406`
- **Surface:** Operator-visible, architectural.
- **Consequence:** Upgrade testing should use a real v5 data copy, and any tooling that assumes the old flat filesystem layout should be re-tested.
- **Shipped-tag confidence:** High.

## 8. Licensing and activation changed from simple key status to entitlement plus monitored-system logic

- **Claim:** V6 licensing is materially different for existing v5 operators, especially around paid-license upgrade, entitlement state, monitored-system limits, and trial behavior.
- **Why it is true:** The public route surface expands beyond v5's key-status model, and the v6 licensing models and migration tests cover entitlements, billing state, monitored-system continuity, unresolved migration blocking, and downgrade-safe legacy retention.
- **Key files:**
  - `pulse-5.1.x@v5.1.27 internal/api/router.go:602-606`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_licensing.go:35-79`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_cloud.go:28-85`
  - `pulse@v6.0.0-rc.1 pkg/licensing/models.go:29-41,44-60,195-220`
  - `pulse@v6.0.0-rc.1 pkg/licensing/billing_store.go:3-40`
  - `pulse@v6.0.0-rc.1 tests/migration/v5_commercial_migration_test.go:137-199`
  - `pulse@v6.0.0-rc.1 tests/migration/v5_real_exchange_upgrade_test.go:123-170`
  - `pulse@v6.0.0-rc.1 internal/api/trial_handlers_commercial_migration_test.go:15-84`
- **Surface:** Operator-visible, architectural.
- **Consequence:** Paid-license upgrade and first-boot commercial state should be part of every serious v5-to-v6 test plan.
- **Shipped-tag confidence:** High.

## 9. Monitored-system counting and grandfathering are explicit v6 operator concerns

- **Claim:** V6 applies limits to canonical top-level monitored systems and can preserve a higher grandfathered floor after v5 upgrade.
- **Why it is true:** The limit enforcement and licensing status code operate on canonical top-level systems, and the auto-migration tests show pending and settled monitored-system continuity behavior.
- **Key files:**
  - `pulse@v6.0.0-rc.1 internal/api/monitored_system_limit_enforcement.go:121-125,430-464`
  - `pulse@v6.0.0-rc.1 internal/api/monitored_system_ledger_test.go:160-164`
  - `pulse@v6.0.0-rc.1 internal/api/licensing_handlers_auto_migrate_test.go:796-880`
  - `pulse@v6.0.0-rc.1 pkg/licensing/service.go:515-547,789-806`
- **Surface:** Operator-visible.
- **Consequence:** Operators should verify monitored-system counts and grandfathered floor status after upgrade instead of assuming the old v5 counting model still applies.
- **Shipped-tag confidence:** High.

## 10. Hosted, org, relay, and mobile onboarding surfaces are present in the shipped RC

- **Claim:** Hosted/org and relay/mobile paths are part of the shipped `v6.0.0-rc.1` surface, even if some self-hosted v5 operators defer them in first-wave testing.
- **Why it is true:** The shipped router registers hosted billing-state, org lifecycle, public signup, magic-link, cloud handoff, org CRUD/membership/share, relay settings/status, onboarding QR/deep-link/validate, and relay-mobile token creation routes. The main router also starts relay runtime and persists relay identity on first enable.
- **Key files:**
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_cloud.go:28-85`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_licensing.go:67-79`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_ai_relay.go:79-100`
  - `pulse@v6.0.0-rc.1 internal/api/router_routes_auth_security.go:126-131`
  - `pulse@v6.0.0-rc.1 internal/api/router.go:2827-3007`
- **Surface:** Operator-visible, architectural.
- **Consequence:** These paths are real shipped scope for the RC, but not every self-hosted v5 operator needs to treat them as first-wave regression targets.
- **Shipped-tag confidence:** High for route/runtime presence. Medium for public-priority framing.

## 11. Install and bootstrap behavior is more governed in v6

- **Claim:** V6 bootstrap artifacts and generated install commands are more structured and more controlled than the v5 install-command model.
- **Why it is true:** V6 setup artifacts include explicit metadata, setup tokens, token hints, and expiry, while generated commands use a governed root-or-sudo wrapper. Released builds resolve install assets to the exact release tag.
- **Key files:**
  - `pulse-5.1.x@v5.1.27 internal/api/router.go:373-375,1346-1352`
  - `pulse@v6.0.0-rc.1 internal/api/agent_install_command_shared.go:35-42,95-107,196-220,245-326`
  - `pulse@v6.0.0-rc.1 internal/api/config_setup_handlers.go:29-149,172-250`
  - `pulse@v6.0.0-rc.1 internal/api/contract_test.go:6883-6894,6945-6959`
- **Surface:** Operator-visible, under-the-hood.
- **Consequence:** Provisioning automation that assumed the old raw command shape or long-lived bootstrap material should be re-tested and updated where necessary.
- **Shipped-tag confidence:** High.
