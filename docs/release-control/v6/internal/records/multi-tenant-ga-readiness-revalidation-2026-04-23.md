# Multi-Tenant GA Readiness Revalidation

- Date: `2026-04-23`
- Gates:
  - `multi-tenant-runtime-isolation-and-coherence`
  - `msp-provider-tenant-management`
  - `organization-user-scope-and-rbac`
- Evidence tier: `local-rehearsal`
- Scope: Current checkout revalidation after the multi-tenant GA audit found red proof in org ownership transfer, MSP account invite lifecycle, portal bundle sync, workspace-limit HA safety, and non-default tenant fallback drift.

## Changes Revalidated

1. Organization ownership transfer now validates the target-member prerequisite before requiring a fresh browser session, so invalid transfers return the canonical `owner_transfer_requires_member` contract instead of a reauth response.
2. MSP account invite lifecycle proof now matches the canonical pending-invitation model for unknown account emails: `202 Accepted` with `state=pending`, not an immediate active membership.
3. Pulse Account embedded portal bundle was rebuilt from the current frontend source and the bundle sync test now passes.
4. Workspace creation now has a registry-backed transactional workspace-limit check in addition to the per-process account lock, so multiple control-plane instances share the same final limit source of truth.
5. Non-default tenant monitor/config helper paths now fail closed instead of falling back to default-org state when tenant monitor resolution is unavailable.

## Proof Commands

- `go test ./internal/api -run 'Test(.*Tenant|.*Org|.*RBAC|.*BillingState|.*WebSocketIsolation|.*ResourceHandlers_NonDefaultOrg|.*SetMultiTenantMonitor_WiresHandlers|.*MultiTenantStateProvider|.*Hosted|.*Token.*Org|.*Isolation|.*RateLimit)' -count=1`
- `go test ./internal/cloudcp ./internal/cloudcp/account ./internal/cloudcp/auth ./internal/cloudcp/handoff ./internal/cloudcp/portal ./internal/cloudcp/registry ./internal/cloudcp/stripe -run 'Test(.*Account|.*Tenant|.*Workspace|.*Portal|.*Session|.*Magic|.*Membership|.*MSP|.*Billing|.*Handoff|.*Route|.*Registry|.*Stripe|.*Grace|.*Provision|.*Lifecycle|.*Entitlement|.*Signup|.*Public)' -count=1`
- `go test ./internal/monitoring -run 'TestMultiTenantMonitor|TestMonitorBroadcastStateUsesTenantChannel|TestMonitorBroadcastStateUsesGlobalChannel|TestMonitorSetOrgID|TestMonitoredSystemUsage|TestRecoveryRollups' -count=1`
- `go test ./pkg/audit ./pkg/cloudauth ./pkg/auth ./pkg/licensing -run 'Test(.*Tenant|.*Org|.*RBAC|.*Session|.*Handoff|.*Billing|.*Plan|.*Workspace|.*Limit|.*Entitlement|.*Grant|.*Scope|.*User|.*Audit|.*Cardinality)' -count=1`
- `npm --prefix frontend-modern test -- src/components/Settings/__tests__/OrganizationSharingPanel.test.tsx src/components/Settings/__tests__/RBACPaywallPanels.test.tsx src/components/Settings/__tests__/OrganizationAccessPanel.test.tsx src/components/Settings/__tests__/OrganizationOverviewPanel.test.tsx src/utils/__tests__/organizationRolePresentation.test.ts src/utils/__tests__/organizationSettingsPresentation.test.ts src/utils/__tests__/rbacPermissions.test.ts src/utils/__tests__/rbacPresentation.test.ts`
- `npm --prefix internal/cloudcp/portal/frontend test`
- `npm --prefix internal/cloudcp/portal/frontend run typecheck`

All commands passed.

## Browser Rehearsal

- Started the local Pulse Account preview server with `npm --prefix internal/cloudcp/portal/frontend run dev`.
- Opened `http://127.0.0.1:8765/?scenario=managed&reset=1` in the in-app browser.
- Verified the managed MSP account shell rendered Workspaces, Access, Invite people, and Billing without console warnings or errors.
- Opened `http://127.0.0.1:8765/?scenario=selfhosted&reset=1&portal_handoff_id=cph_preview&feature=self_hosted_plan`.
- Verified the self-hosted plan-upgrade billing shell rendered the updated Plans-page checkout copy and plan cards without console warnings or errors.

## Outcome

- The previously red local proof surfaces are green.
- The shipped Pulse Account bundle is synchronized with the current source.
- The workspace-limit path has a database-backed final enforcement point for multi-instance control-plane deployments.
- Non-default tenant state resolution no longer has the audited default-org fallback drift in the router/config helper paths.

## Remaining Promotion Note

This record is a local GA-readiness revalidation. The older `real-external-e2e` MSP production evidence remains the external proof floor for `msp-provider-tenant-management`; a fresh production promotion rehearsal should still be run as part of the release-day publication workflow.
