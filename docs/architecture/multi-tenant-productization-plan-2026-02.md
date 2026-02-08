# Multi-Tenant Productization Plan (Detailed Execution Spec)

Status: Draft
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/multi-tenant-productization-progress-2026-02.md`

## Product Intent

Pulse is single-tenant first by default.
Multi-tenant is opt-in. If not enabled, users must not see multi-tenant UI, routes, copy, or behavior leaks.

## Non-Negotiable Contracts

1. Single-tenant invisibility contract:
- No org switcher.
- No organization settings routes.
- No org sharing/billing/member management UI.
- No multi-tenant wording in general UX.
- Multi-tenant API endpoints return feature-disabled behavior when tenant mode is off.

2. Server trust boundary contract:
- Client headers are advisory only.
- Server validates caller membership/role/org binding on every org-scoped operation.

3. Isolation contract:
- No cross-org reads.
- No cross-org writes.
- No cross-org websocket events.
- No cross-org push/notification leakage.
- No cache/background-job cross-contamination.

4. Rollback contract:
- A single kill switch can disable multi-tenant behavior safely.
- Rollback instructions are documented and tested.

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: delegated coding agent.
- Reviewer: orchestrator.

A packet can be marked DONE only when:
- all packet tests pass,
- all required evidence is provided,
- reviewer gate checklist passes,
- verdict is `APPROVED`.
- every corresponding checklist item in the progress tracker is checked.

## Required Review Output (for every packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit 0
2. `<command>` -> exit 0

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Global Validation Baseline

Run after every packet unless explicitly waived:

1. `go build ./...`
2. `go test ./internal/api/... -v`
3. `go test ./internal/ai/... -v` (for packets touching AI/push/contracts)
4. `go test ./internal/monitoring/... -v` (for packets touching websocket/monitor flows)
5. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
6. `npm --prefix frontend-modern exec -- vitest run`

Note:
- If a full suite is too heavy for a packet, run targeted tests during packet implementation and run full baseline at milestone boundaries (M2, M4, Final).

## Execution Packets

### Packet 00: Surface Inventory and Risk Register

Objective:
- Build a complete inventory of multi-tenant surfaces and map each to a guard strategy.

Scope:
- `internal/api/`
- `internal/ai/`
- `internal/monitoring/`
- `frontend-modern/src/components/`
- `frontend-modern/src/routing/`
- `frontend-modern/src/utils/apiClient.ts`
- `frontend-modern/src/stores/`

Implementation checklist:
1. Enumerate all org-related endpoints and their authz model.
2. Enumerate all frontend entry points that expose org state.
3. Enumerate websocket and notification emission paths with org context.
4. Build risk register with severity and remediation owner.

Tests:
1. `go test ./internal/api/... -run TestOrgHandlers -v`
2. `go test ./internal/api/... -run TestContract_ -v`

Evidence:
- Inventory table committed to docs.
- Risk register with high/medium/low severity.

Exit criteria:
- 100% of known surfaces have an explicit guard strategy.

### Packet 01: Central Feature Gating and Invisibility Backbone

Objective:
- Create one canonical gate path so single-tenant mode hides all multi-tenant surfaces.

Scope:
- `frontend-modern/src/utils/featureFlags.ts`
- `frontend-modern/src/App.tsx`
- `frontend-modern/src/routing/`
- `frontend-modern/src/components/Settings/`
- `internal/api/router.go`

Implementation checklist:
1. Ensure a single frontend helper determines multi-tenant visibility.
2. Ensure top-level nav, routes, and settings sections all use the same helper.
3. Ensure backend route registration or handlers enforce feature-disabled responses.
4. Remove duplicate/legacy gating logic in scattered components.

Tests:
1. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/platformTabs.test.ts`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`
3. `go test ./internal/api/... -run TestOrgHandlersMultiTenantGate -v`

Evidence:
- Before/after screenshots or route snapshots in single-tenant mode.
- File-level mapping of where old gates were removed.

Exit criteria:
- In single-tenant mode, no multi-tenant UI or route entry points remain.

### Packet 02: Single-Tenant UX Parity Sweep

Objective:
- Guarantee default users see a clean single-tenant product with no incidental org artifacts.

Scope:
- `frontend-modern/src/components/`
- `frontend-modern/src/routing/`
- `frontend-modern/src/features/`

Implementation checklist:
1. Remove stray multi-tenant labels/tooltips/copy outside tenant areas.
2. Verify loading/empty/error states do not reference orgs unless feature is enabled.
3. Fix dead links and route aliases caused by hidden tenant sections.

Tests:
1. `npm --prefix frontend-modern exec -- vitest run src/routing/__tests__/resourceLinks.test.ts`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Settings/__tests__/settingsRouting.test.ts`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Evidence:
- Single-tenant UX checklist with pass/fail for each page family.

Exit criteria:
- No multi-tenant text/components are visible with feature disabled.

### Packet 03: API Authorization and Org Binding Hardening

Objective:
- Enforce strict server-side org binding and role checks across all org-sensitive handlers.

Scope:
- `internal/api/org_handlers.go`
- `internal/api/router.go`
- `internal/config/multi_tenant.go`
- related auth helpers in `internal/api/`

Implementation checklist:
1. Validate org membership before every org read/write operation.
2. Validate role for org-admin/owner-only operations.
3. Ensure token-based access cannot bypass write restrictions.
4. Ensure invalid org IDs and malformed payloads fail safely.

Tests:
1. `go test ./internal/api/... -run TestOrgHandlersCRUDLifecycle -v`
2. `go test ./internal/api/... -run TestOrgHandlersMemberCannotManageOrg -v`
3. `go test ./internal/api/... -run TestOrgHandlersTokenListAllowedButWriteForbidden -v`
4. `go test ./internal/api/... -run TestOrgHandlersCrossOrgIsolation -v`
5. `go test ./internal/api/... -run TestOrgHandlersShareIsolationAcrossOrganizations -v`

Evidence:
- Table mapping each endpoint to role requirements and tests.

Exit criteria:
- Cross-org and unauthorized actions are denied consistently.

### Packet 04: Header Spoofing and Context Propagation Hardening

Objective:
- Ensure `X-Org-ID` or equivalent client context cannot escalate access.

Scope:
- `frontend-modern/src/utils/apiClient.ts`
- `internal/api/`
- middleware/auth context files used by org handlers

Implementation checklist:
1. Treat client org header as requested context, not trusted identity.
2. Resolve effective org from authenticated membership on server.
3. Deny mismatched org context with deterministic error semantics.
4. Ensure logs/metrics capture spoof attempts.

Tests:
1. Add/extend spoofing negative-path tests in `internal/api/...`.
2. `go test ./internal/api/... -run "Spoof|CrossOrg|OrgHandlers" -v`

Evidence:
- Explicit test cases showing spoof attempts denied.

Exit criteria:
- Header spoofing cannot bypass org access controls.

### Packet 05: Realtime Isolation (Websocket + Push + AI Events)

Objective:
- Ensure all event streams are strictly org-scoped.

Scope:
- `internal/monitoring/`
- `internal/api/ai_handler.go`
- `internal/ai/`
- push/relay notification emitters and serializers

Implementation checklist:
1. Trace event emission sources and attach org scope enforcement.
2. Validate subscription/session filtering by org membership.
3. Ensure no duplicate or malformed completion/error events for mobile contracts.
4. Ensure approval/action identifiers remain correct and org-safe.

Tests:
1. `go test ./internal/api/... -run TestContract_ -v`
2. `go test ./internal/ai/... -run "Push|Approval|Stream|Contract" -v`
3. `go test ./internal/monitoring/... -v`

Evidence:
- Event matrix: event type, scope key, filter point, test coverage.

Exit criteria:
- No cross-org realtime leakage in tests.

### Packet 06: Workflow Polish (Admin and Member)

Objective:
- Make organization workflows production-grade and consistent.

Scope:
- `frontend-modern/src/components/OrgSwitcher.tsx`
- org settings/org management/billing/sharing frontend files
- `frontend-modern/src/api/orgs.ts`

Implementation checklist:
1. Validate org switcher behavior for single-org and multi-org users.
2. Validate org overview, membership, role updates, ownership transfer UX.
3. Validate sharing flows and permission-denied UX copy.
4. Ensure all error states are actionable and consistent.

Tests:
1. Add/extend component tests for org switcher + org settings flows.
2. `npm --prefix frontend-modern exec -- vitest run src/components/**/__tests__/*.test.ts`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Evidence:
- Workflow checklist for admin/member personas with pass/fail.

Exit criteria:
- All primary admin/member workflows complete without workaround paths.

### Packet 07: Operational Hardening (Rollout, Kill Switch, Telemetry)

Objective:
- Make multi-tenant rollout safe and observable.

Scope:
- feature flag config and docs
- server metrics/logging for org access denials and switch failures
- rollback runbook docs

Implementation checklist:
1. Define staged rollout strategy and owner checkpoints.
2. Add metrics for org-switch failures and denied access by endpoint.
3. Add alerts for suspicious cross-org denial spikes.
4. Validate kill switch behavior in test/dev environments.

Tests:
1. `go test ./internal/api/... -v`
2. targeted tests for flag-disabled behavior and fallback states

Evidence:
- Runbook with rollback steps and expected post-rollback state.
- Metric/alert inventory.

Exit criteria:
- Kill switch and rollback validated with evidence.

### Packet 08: Final Certification

Objective:
- Prove multi-tenant is fully polished without regressing single-tenant users.

Scope:
- Full stack (backend + frontend + contracts + realtime)

Implementation checklist:
1. Execute full test baseline.
2. Execute single-tenant invisibility checklist.
3. Execute multi-tenant happy/negative path checklist.
4. Produce final release-readiness report.

Tests:
1. `go build ./...`
2. `go test ./...`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
4. `npm --prefix frontend-modern exec -- vitest run`

Evidence:
- Final report with command outputs, gate status, known risks, and follow-ups.

Exit criteria:
- Reviewer marks final verdict `APPROVED`.

## Milestones

M1 complete when Packets 00-02 are approved.
M2 complete when Packets 03-05 are approved.
M3 complete when Packets 06-07 are approved.
M4 complete when Packet 08 is approved.

## Definition of Done

This initiative is complete only when all of the following are true:

1. Single-tenant mode has zero visible multi-tenant surface area.
2. Server denies cross-org access across API, websocket, push, and background flows.
3. Admin/member org workflows are complete and polished.
4. Kill switch and rollback are validated with documented evidence.
5. Full certification suite is green and review verdict is `APPROVED`.

## Delegation Packet Template

Use this exact structure when delegating each packet:

```markdown
IMPLEMENTATION PACKET: <Packet ID and Name>

Objective:
- <objective>

Allowed scope:
- <paths>

Required implementation:
1. <task>
2. <task>

Required tests:
1. `<command>`
2. `<command>`

Required evidence in response:
1. Files changed + reason per file
2. Command outputs + explicit exit codes
3. Risks and rollback

Approval gates:
- P0: no timeout/empty/truncated output accepted
- P1: packet-specific security/isolation/contract checks pass
- P2: docs and tracker updated after passing gates only
```
