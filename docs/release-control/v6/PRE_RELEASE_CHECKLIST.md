# Pulse v6 Pre-Release Checklist

Use this as the final gate before cutting a Pulse v6 pre-release.

## Execution Notes
- Run the commands from the repository root unless a step says otherwise.
- Treat every failed command or failed scenario as a release blocker until it is explained or fixed.
- If the mobile app is part of the release, run the mobile checks from `/Volumes/Development/pulse/repos/pulse-mobile`.
- Record pass/fail notes inline in this file or in the release ticket as each section is completed.
- Use `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` for the trust-critical hosted, relay, mobile, entitlement, org-scope, and API-token checks that must be manually confirmed before release.
- Follow `RELEASE_PROMOTION_POLICY.md` for channel routing, RC soak, stable promotion, and rollback expectations.

## Current Status
- Automated command-driven checks completed on 2026-03-06 are marked `[x]` below.
- Manual scenarios and staging-like end-to-end flows are still the main remaining release gate.
- Mobile is in scope for the release and now has targeted readiness coverage in `pulse-mobile`.
- High-risk release confidence now lives in `docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` and should be cleared alongside this checklist.

## Promotion Policy
- [ ] Record the previous stable tag and exact rollback pin command before publishing a new RC or stable release.
- [ ] For the GA/stable candidate, confirm the release pipeline has already been exercised on a real RC tag, not only linted or YAML-parsed.
- [ ] For stable promotion, confirm the candidate commit has already shipped on `rc`.
- [ ] For stable promotion, confirm the RC soak window is at least 72 hours or document the hotfix exception explicitly.
- [ ] For stable promotion, confirm paid production tenants are not being moved onto an unvalidated build.
- [ ] For GA/stable promotion, confirm `V5_MAINTENANCE_SUPPORT_POLICY.md` is still the intended policy and write down the exact v6 GA date and exact v5 end-of-support date that will ship with the announcement.

## Scope
- [x] Confirm whether there is a separate mobile app codebase.
- [x] If yes, do not call the whole product pre-release ready until that app is audited.
- [ ] If no, treat this checklist as the full product gate.

## Worktree
- [x] Run `git status --porcelain`.
- [x] Confirm there are no tracked modified or staged files.
- [x] Ignore only the known local untracked artifacts if they are expected:
  - [x] `.gocache/`
  - [x] `.gotmp/`
  - [x] `pulse-host-agent`
  - [x] `unused_param_test.o`

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
git status --porcelain
```

## Canonical v6 Contract
- [ ] Verify the key canonical v6 hardening commits are present in the release target:
  - [x] `26a88bf66512ec5cafaf6e597a5ba736d5ff279b` `internal/api/resources`
  - [x] `c5eb7057a0b2a7e8dfbcd44470185a72d083feff` `internal/ai/tools`
  - [x] `3ab9d6c13` `internal/servicediscovery`
  - [x] `f89e4a56bcad9901edd6afc3212172845605d6e7` `internal/api/state_provider` / `router_helpers`
  - [x] `7115143825647f5bfade7469fe20a324d3a76dbe` `internal/monitoring/monitor.go`
- [ ] Verify the Patrol and frontend canonicalization commits required for the release target are included.
- [x] Run the key backend/API/AI/frontend test slices that protect canonical v6 behavior.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
git merge-base --is-ancestor 26a88bf66512ec5cafaf6e597a5ba736d5ff279b HEAD
git merge-base --is-ancestor c5eb7057a0b2a7e8dfbcd44470185a72d083feff HEAD
git merge-base --is-ancestor 3ab9d6c13 HEAD
git merge-base --is-ancestor f89e4a56bcad9901edd6afc3212172845605d6e7 HEAD
git merge-base --is-ancestor 7115143825647f5bfade7469fe20a324d3a76dbe HEAD
go test ./internal/api -run 'Test(FrontendResourceType|ApplyFrontendTypes|ComputeFrontendByType|ParseResourceTypesNodeAlias|UnsupportedResourceTypeFilterTokensRejectsLegacyAliases|ResourceListIncludesKubernetesPods|ResourceListFiltersCanonicalKubernetesNamespace|BuildDiscoveryTargetKubernetesPrefersAgentID|ResourceListRejectsLegacyKubernetesTypeAlias|ResourceListReturnsCanonicalKubernetesMetricsTargets|ResourceListProxmoxNodeReturnsFrontendNodeType|ResourceGetProxmoxNodeReturnsFrontendNodeType|ResourceListRejectsLegacyHostTypeFilter|ResourceListMergesLinkedHost|ResourceListUsesUnifiedSeedProvider|ResourceListDoesNotMergeOneSidedLinkedHost|ResourceGetResource|ResourceLinkMergesResources|ResourceReportMergeCreatesExclusions)$'
go test ./internal/ai/tools
go test ./internal/servicediscovery
go test ./internal/monitoring -run 'TestMonitor(GetUnifiedReadStateOrSnapshot|UnifiedResourceSnapshot)'
cd /Volumes/Development/pulse/repos/pulse/frontend-modern
npx vitest run src/utils/__tests__/frontendResourceTypeBoundaries.test.ts
```

## Hosted / Cloud
- [x] Run hosted signup and billing lifecycle tests.
- [x] Verify hosted paid checkout fails closed when org linkage is missing.
- [x] Verify replay succeeds once the linked org exists.
- [ ] Verify the normal hosted provisioning path still succeeds.
- [x] Confirm commit `8b3a5d30246a86005d363b405126b0e9bfdda8d3` is present.
- [ ] Clear gate `hosted-signup-billing-replay` in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
git merge-base --is-ancestor 8b3a5d30246a86005d363b405126b0e9bfdda8d3 HEAD
go test ./internal/api -run 'TestStripeWebhook_'
go test ./internal/cloudcp/... -count=1
go test ./internal/hosted/... -count=1
```

Manual scenario:
- Complete a hosted signup or replay a recorded checkout flow.
- Verify webhook handling does not return success when org linkage is unresolved.
- Verify a replay succeeds once the org exists.

## Relay
- [x] Run Relay tests end to end.
- [ ] Verify fresh register works.
- [ ] Verify reconnect works.
- [x] Verify stale cached session resume recovers by fresh registration.
- [ ] Verify abrupt disconnect and inflight drain behavior still work.
- [x] Confirm commit `ee78cb33dd35b891d84e61bfce2098b4548bbb56` is present.
- [ ] Clear gate `relay-registration-reconnect-drain` in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
git merge-base --is-ancestor ee78cb33dd35b891d84e61bfce2098b4548bbb56 HEAD
go test ./internal/relay -count=1 -timeout=120s
```

Manual scenario:
- Connect a Relay client normally.
- Force a stale cached session or server-side session eviction.
- Verify the client retries with fresh registration instead of hanging in resume/backoff loops.

## Multi-Tenant / MSP
- [x] Run multi-tenant tests.
- [x] Verify unknown non-default orgs fail closed.
- [x] Verify explicitly provisioned orgs initialize correctly.
- [x] Verify there is no default-org fallback for non-default tenant requests.
- [x] Confirm commit `f090a77ae` is present.
- [ ] Clear gate `organization-user-scope-and-rbac` in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
git merge-base --is-ancestor f090a77ae HEAD
go test ./internal/monitoring -run 'TestMultiTenantMonitor'
go test ./internal/api -run 'TestMultiTenantStateProvider_|TestSetMultiTenantMonitor_WiresHandlers|TestResourceHandlers_NonDefaultOrg|TestMultiTenant_ConcurrentAPIStress'
```

Manual scenario:
- Use a non-default org header/cookie/token that does not correspond to a provisioned tenant.
- Verify monitor/API access fails closed.
- Provision the tenant explicitly and verify access succeeds afterwards.

## Frontend Smoke
- [ ] Verify dashboard/workloads.
- [ ] Verify infrastructure/discovery.
- [ ] Verify alerts/investigate.
- [ ] Verify reporting/settings.
- [ ] Verify metrics history / charts.
- [x] Confirm canonical routes and page state still behave correctly.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse/frontend-modern
npx vitest run src/routing/__tests__/resourceLinks.test.ts src/routing/__tests__/navigation.test.ts src/utils/__tests__/frontendResourceTypeBoundaries.test.ts src/api/__tests__/chartsApi.test.ts src/hooks/__tests__/useDashboardTrends.test.ts
```

Manual scenario:
- Open workloads, infrastructure/discovery, alerts, reporting, and metrics screens.
- Verify workload URLs use canonical types.
- Verify Kubernetes/pod pages and investigation paths still behave correctly.

## AI / Tools Smoke
- [x] Verify `pulse_query` works.
- [x] Verify Kubernetes tools work through unified read-state.
- [x] Verify PMG tools still work.
- [x] Verify Patrol scoped runs still behave correctly.
- [ ] Clear gate `paid-feature-entitlement-gating` in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
go test ./internal/ai/tools -count=1
go test ./internal/ai -count=1
```

Manual scenario:
- Run representative `pulse_query`, Kubernetes, PMG, and Patrol flows.
- Verify scoped Patrol runs only see and mutate in-scope resources.

## Release-Facing Scenarios
- [ ] Agent registration / install journey.
- [ ] `/api/resources` filtering and resource detail retrieval.
- [ ] Relay pairing flow.
- [ ] Hosted signup / org creation / billing webhook replay.
- [ ] Multi-tenant org switching / tenant-bound API calls.
- [ ] AI investigate / execute / query on canonical resource types.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse/tests/integration
npx tsc --noEmit
```

Manual scenario:
- Walk the user-visible flows end to end in a staging-like environment.
- Confirm the canonical resource model holds up in real UI/API usage, not only in unit tests.

## Final Verification
- [ ] Run the final targeted Go test slices required for release confidence.
- [ ] Run the relevant frontend tests/build checks for canonicalized surfaces.
- [ ] Document any known unrelated failures explicitly before release.
- [ ] Confirm there are no new high-severity regressions in hosted, relay, tenant isolation, or canonical v6 boundaries.

Run:

```bash
cd /Volumes/Development/pulse/repos/pulse
go test ./internal/api/... -count=1
go test ./internal/relay/... -count=1
go test ./internal/cloudcp/... -count=1
go test ./internal/hosted/... -count=1
go test ./internal/monitoring/... -count=1
go test ./internal/ai/... -count=1
cd /Volumes/Development/pulse/repos/pulse/frontend-modern
npx vitest run
```

If mobile is in scope, also run:

```bash
cd /Volumes/Development/pulse/repos/pulse-mobile
git status --porcelain
npm test -- --runTestsByPath src/utils/__tests__/secureStorage.test.ts src/stores/__tests__/instanceStore.test.ts src/stores/__tests__/authStore.test.ts
npm test -- --runTestsByPath src/relay/__tests__/client.test.ts src/relay/__tests__/client-hardening.test.ts
npm test -- --runTestsByPath src/stores/__tests__/approvalStore.test.ts
```

Mobile automated checks completed:
- [x] Secure storage / auth / instance persistence
- [x] Relay reconnect and hardening tests
- [x] Approval store state consistency
- [ ] Clear gate `mobile-relay-auth-approvals` in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md`.

## High-Risk Trust Gates
- [ ] Clear gate `hosted-signup-billing-replay`.
- [ ] Clear gate `paid-feature-entitlement-gating`.
- [ ] Clear gate `rc-to-ga-promotion-readiness`.
- [ ] Clear gate `relay-registration-reconnect-drain`.
- [ ] Clear gate `mobile-relay-auth-approvals`.
- [ ] Clear gate `organization-user-scope-and-rbac`.
- [ ] Clear gate `api-token-scope-and-assignment`.

## Release Decision
Mark Pulse v6 pre-release ready only if all of the following are true:
- [ ] The smoke checks above pass.
- [x] There is no separate unaudited mobile app.
- [ ] There is no tracked dirty state.
- [ ] No new high-severity regressions appear.

## Stable Promotion Decision
Mark a Pulse v6 build stable-promotion ready only if all of the following are true:
- [ ] The candidate commit has already been exercised on `rc`.
- [ ] The `Promotion Policy` section above is complete.
- [ ] Applicable items in `HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md` are cleared.
- [ ] The rollback target and exact reinstall command are recorded.
- [ ] The release pipeline has already been exercised on the candidate RC path in a real run, not only statically validated.
- [ ] The v5 maintenance-only support policy, exact GA/EOS dates, and release-note notice are written and ready to publish.
