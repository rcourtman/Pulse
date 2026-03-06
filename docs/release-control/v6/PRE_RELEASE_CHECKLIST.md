# Pulse v6 Pre-Release Checklist

Use this as the final gate before cutting a Pulse v6 pre-release.

## Scope
- [ ] Confirm whether there is a separate mobile app codebase.
- [ ] If yes, do not call the whole product pre-release ready until that app is audited.
- [ ] If no, treat this checklist as the full product gate.

## Worktree
- [ ] Run `git status --porcelain`.
- [ ] Confirm there are no tracked modified or staged files.
- [ ] Ignore only the known local untracked artifacts if they are expected:
  - [ ] `.gocache/`
  - [ ] `.gotmp/`
  - [ ] `pulse-host-agent`
  - [ ] `unused_param_test.o`

## Canonical v6 Contract
- [ ] Verify the key canonical v6 hardening commits are present in the release target:
  - [ ] `26a88bf66512ec5cafaf6e597a5ba736d5ff279b` `internal/api/resources`
  - [ ] `c5eb7057a0b2a7e8dfbcd44470185a72d083feff` `internal/ai/tools`
  - [ ] `3ab9d6c13` `internal/servicediscovery`
  - [ ] `f89e4a56bcad9901edd6afc3212172845605d6e7` `internal/api/state_provider` / `router_helpers`
  - [ ] `7115143825647f5bfade7469fe20a324d3a76dbe` `internal/monitoring/monitor.go`
- [ ] Verify the Patrol and frontend canonicalization commits required for the release target are included.
- [ ] Run the key backend/API/AI/frontend test slices that protect canonical v6 behavior.

## Hosted / Cloud
- [ ] Run hosted signup and billing lifecycle tests.
- [ ] Verify hosted paid checkout fails closed when org linkage is missing.
- [ ] Verify replay succeeds once the linked org exists.
- [ ] Verify the normal hosted provisioning path still succeeds.
- [ ] Confirm commit `8b3a5d30246a86005d363b405126b0e9bfdda8d3` is present.

## Relay
- [ ] Run Relay tests end to end.
- [ ] Verify fresh register works.
- [ ] Verify reconnect works.
- [ ] Verify stale cached session resume recovers by fresh registration.
- [ ] Verify abrupt disconnect and inflight drain behavior still work.
- [ ] Confirm commit `ee78cb33dd35b891d84e61bfce2098b4548bbb56` is present.

## Multi-Tenant / MSP
- [ ] Run multi-tenant tests.
- [ ] Verify unknown non-default orgs fail closed.
- [ ] Verify explicitly provisioned orgs initialize correctly.
- [ ] Verify there is no default-org fallback for non-default tenant requests.
- [ ] Confirm commit `f090a77ae` is present.

## Frontend Smoke
- [ ] Verify dashboard/workloads.
- [ ] Verify infrastructure/discovery.
- [ ] Verify alerts/investigate.
- [ ] Verify reporting/settings.
- [ ] Verify metrics history / charts.
- [ ] Confirm canonical routes and page state still behave correctly.

## AI / Tools Smoke
- [ ] Verify `pulse_query` works.
- [ ] Verify Kubernetes tools work through unified read-state.
- [ ] Verify PMG tools still work.
- [ ] Verify Patrol scoped runs still behave correctly.

## Release-Facing Scenarios
- [ ] Agent registration / install journey.
- [ ] `/api/resources` filtering and resource detail retrieval.
- [ ] Relay pairing flow.
- [ ] Hosted signup / org creation / billing webhook replay.
- [ ] Multi-tenant org switching / tenant-bound API calls.
- [ ] AI investigate / execute / query on canonical resource types.

## Final Verification
- [ ] Run the final targeted Go test slices required for release confidence.
- [ ] Run the relevant frontend tests/build checks for canonicalized surfaces.
- [ ] Document any known unrelated failures explicitly before release.
- [ ] Confirm there are no new high-severity regressions in hosted, relay, tenant isolation, or canonical v6 boundaries.

## Release Decision
Mark Pulse v6 pre-release ready only if all of the following are true:
- [ ] The smoke checks above pass.
- [ ] There is no separate unaudited mobile app.
- [ ] There is no tracked dirty state.
- [ ] No new high-severity regressions appear.
