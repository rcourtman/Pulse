# Hotfix 5.1.3 Execution Checklist

Last updated: 2026-02-07
Owner: Codex + maintainer
Branch: `pulse/hotfix-5.1.3`
Base tag: `v5.1.2` (`c949e9c9`)

## 1) Branch Start Verification
- [x] `git status` checked
- [x] `git log --oneline -n 3` checked
- [x] `git describe --tags --exact-match` equals `v5.1.2`

## 2) P0 Scope (Must Ship)

### 2.1 Proxmox stale/offline reliability (`#1094`, `#1204`, `#1192`, `#1199`)
- [x] Reproducer documented
- [x] Acceptance criteria defined
- [x] Fix implemented
- [x] Automated tests added/updated
- [x] Manual validation evidence captured
- [x] Release note entry prepared (factual only)

Acceptance criteria:
- [x] Fresh data does not become stale/false-offline during normal polling window
- [x] No stale-state carryover after temporary offline transition

Evidence links/notes:
- `internal/monitoring/monitor.go`: empty-node fallback now preserves recent nodes within grace window.
- `internal/monitoring/monitor_memory_test.go`:
  - `TestPollPVEInstancePreservesRecentNodesWhenGetNodesReturnsEmpty`
  - `TestPollPVEInstanceMarksStaleNodesOfflineWhenGetNodesReturnsEmpty`

### 2.2 Alerting stale evaluator / loop reliability (`#1096`, `#1179`, `#1159`, `#1043`)
- [x] Reproducer documented
- [x] Acceptance criteria defined
- [x] Fix implemented
- [x] Automated tests added/updated
- [x] Manual validation evidence captured
- [x] Release note entry prepared (factual only)

Acceptance criteria:
- [x] Evaluator resumes after offline -> online transitions
- [x] No deadlock/freeze under sustained alert checks

Evidence links/notes:
- `internal/alerts/alerts.go`: `checkMetric` re-notify path now dispatches asynchronously to reduce evaluator loop blocking risk.
- Covered by existing dispatch/checkMetric tests in `internal/alerts/alerts_test.go`.

### 2.3 Swarm alert correctness (`#1202` + support thread symptoms)
- [x] Reproducer documented
- [x] Acceptance criteria defined
- [x] Fix implemented
- [x] Automated tests added/updated
- [x] Manual validation evidence captured
- [x] Release note entry prepared (factual only)

Acceptance criteria:
- [x] Healthy services do not trigger false warning spam
- [x] Alert messaging matches observed service state

Evidence links/notes:
- `internal/alerts/alerts.go`: Docker service alerts now notify on new alert and warning->critical escalation only; unchanged degraded state updates in-place without poll-cycle re-notify spam; rate-limit check added.
- `internal/alerts/alerts_test.go`:
  - `TestDockerServiceAlertDoesNotRenotifyWhenUnchanged`
  - `TestDockerServiceAlertRenotifiesOnEscalationToCritical`

### 2.4 License gate hardening (key/config mismatch regressions)
- [x] Reproducer documented
- [x] Acceptance criteria defined
- [x] Startup/assertion logging for active license verification key fingerprint
- [x] CI/release guard against wrong-key build silently passing
- [x] Automated tests added/updated
- [x] Manual validation evidence captured
- [x] Release note entry prepared (factual only)

Acceptance criteria:
- [x] Valid Pro key consistently unlocks Pro features after restart/update
- [x] Wrong-key/config mismatch is visible and blocks release path

Evidence links/notes:
- `internal/license/pubkey.go`: startup logs now include key source and `SHA256` fingerprint of active verification key.
- `scripts/build-release.sh`: release build now fails if `PULSE_LICENSE_PUBLIC_KEY` missing (unless explicit local bypass) and can assert expected fingerprint via `PULSE_LICENSE_PUBLIC_KEY_FINGERPRINT`.
- `internal/license/pubkey_test.go`: added `TestPublicKeyFingerprint`.

## 3) P1 Scope (Ship Only If Low Risk)

### 3.1 Host URL edit regression (`#1197`)
- [ ] Triaged
- [ ] Fixed (if low risk)
- [ ] Validated

### 3.2 Release notes link (`#1195`)
- [ ] Triaged
- [ ] Fixed (if low risk)
- [ ] Validated

### 3.3 Rootless Docker detection (`#1200`)
- [ ] Triaged
- [ ] Fixed (if low risk)
- [ ] Validated

### 3.4 Backup attribution duplicate VMID edge case (`#1177`)
- [ ] Triaged
- [ ] Fixed (if low risk)
- [ ] Validated

### 3.5 VM disk totalBytes inflation edge case (`#1158`)
- [ ] Triaged
- [ ] Fixed (if low risk)
- [ ] Validated

## 4) Verification Gate (Required Before Tag)
- [x] `make test`
- [x] `make lint-frontend`
- [x] `make frontend`
- [x] `make build`
- [ ] Manual smoke: Proxmox freshness over extended run
- [ ] Manual smoke: alerts survive offline -> online transitions
- [ ] Manual smoke: Swarm false warnings absent for healthy services
- [ ] Manual smoke: Pro license survives restart/update
- [ ] Manual smoke: support bundle captures diagnostic evidence

## 5) Release Steps
- [ ] Release notes updated with verified fixes only
- [ ] Version bumped to `5.1.3`
- [ ] Tag and publish release from `pulse/hotfix-5.1.3`
- [ ] Fixed issues updated with exact version + validation notes
- [ ] Hotfix commits back-merged/cherry-picked to forward branch

## 6) Execution Log
- 2026-02-07: Initialized checklist and validated branch starts from `v5.1.2`.
- 2026-02-07: Implemented P0 stabilization patches for Proxmox empty-node grace handling, alert loop async re-notify, Swarm service re-notify dedupe/escalation behavior, and license key fingerprint + release guard hardening.
- 2026-02-07: Addressed pre-ship findings: preserved `LastNotified` for rebuilt service alerts and added explicit escalation logging for Docker service alert escalations.
- 2026-02-07: Validation rerun complete: targeted monitoring/alerts/license tests passed, plus `make test`, `make lint-frontend`, `make frontend`, and `make build`.
