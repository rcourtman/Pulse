# Hotfix 5.1.3 Start Here

Last updated: 2026-02-07  
Branch: `pulse/hotfix-5.1.3`  
Base: `v5.1.2` (`c949e9c9`)

## Why This Exists
`5.1.3` is a stabilization release.  
Goal: restore trust and reliability quickly without mixing in large architectural changes.

This branch is intentionally isolated from the forward/unified-resource work.

## Guardrails (Non-Negotiable)
- Do not merge any unified-resource/navigation overhaul work into this branch.
- Keep fixes minimal, targeted, and low-risk.
- Every fix must have either:
  - a reproducer and a test, or
  - a reproducer and explicit manual validation evidence.
- Do not send customer follow-ups until behavior is verified locally or in known-good diagnostics.

## Known Customer Context (Cosmin)
Recent thread context (Feb 6-7, 2026):
- License appeared valid but Pro areas were locked (reported on 5.1.2).
- Docker/Swarm alert behavior looked incorrect to customer.
- Customer explicitly challenged prior explanation ("services are up, why 0.0 of 0?").
- Prior thread included accidental/incorrect outbound messages; trust is currently fragile.

Implication for 5.1.3:
- Prioritize correctness and confidence over breadth.
- Release should avoid speculative claims and include clear, verified behavior notes.

## Priority Scope

## P0 (Must Ship in 5.1.3)
1. Proxmox data freshness / false offline / stale state reliability  
Issues: `#1094`, `#1204`, `#1192`, `#1199`

2. Alerting loop reliability and stale-evaluator behavior  
Issues: `#1096`, `#1179`, `#1159`, `#1043`

3. Swarm service alert correctness (false warning patterns)  
Related customer complaint + issues: `#1202` (metrics gap), alert symptoms seen in support thread

4. License gate hardening against key/config mismatch regressions  
Not a clean open issue for this exact latest incident, but high business impact from support thread.
At minimum:
- add startup/assertion logging around active license verification key fingerprint
- add test/guard so wrong-key build cannot silently pass CI/release path

## P1 (Ship If Low Risk, Else Defer)
1. Host URL edit discoverability/regression  
Issue: `#1197`

2. Release notes "View details" broken link  
Issue: `#1195`

3. Rootless Docker detection  
Issue: `#1200`

4. Backup attribution correctness (duplicate VMID edge cases)  
Issue: `#1177`

5. VM disk totalBytes inflation edge cases  
Issue: `#1158`

## P2 (Explicitly Defer Unless Free/Fast)
- Mobile rendering regressions (`#1196`)
- Reporting engine initialization (`#1186`)
- Broader enhancement requests (for example partition exclusion)

## Start Checklist (Do This First)
1. Confirm branch and base:
   - `git status`
   - `git log --oneline -n 3`
   - `git describe --tags --exact-match` should be `v5.1.2` at branch start

2. Create a tracking checklist issue or local checklist from this doc.

3. Reproduce P0 items one by one with minimal fixtures/diagnostics.

4. Define acceptance criteria before coding each fix.

5. Implement smallest safe patch per item, with tests where possible.

## Suggested Execution Order
1. Proxmox stale/offline reliability (`#1094` cluster)  
Reason: highest customer pain + long-lived issue + high comment volume.

2. Alerting deadlock/stale evaluations (`#1096` cluster)  
Reason: can cause monitoring trust collapse across features.

3. Swarm alert correctness and messaging  
Reason: directly tied to active customer thread and confusion.

4. License verification hardening  
Reason: low frequency, high severity business impact.

5. Quick P1 regressions (`#1195`, `#1197`) if near-zero risk.

## Engineering Standards for This Hotfix
- One logical fix per commit.
- Commit message format:
  - `fix(<area>): <what changed> (#issue)`
- Add/adjust tests close to fix location.
- Prefer surgical changes over refactors.
- Keep public behavior notes precise (no guessing).

## Verification Matrix (Release Gate)
All items below must pass before tagging `v5.1.3`.

1. Backend tests:
   - `make test`

2. Frontend lint/build sanity:
   - `make lint-frontend`
   - `make frontend`

3. Full build:
   - `make build`

4. Manual smoke checks:
   - Proxmox: nodes remain fresh/online over extended run window
   - Alerts: no freeze/stale evaluator after offline->online transitions
   - Swarm: no false warning spam for healthy services
   - License: valid Pro key unlocks Pro features consistently after restart/update
   - "View details" link works (if patched)
   - Host URL editing path is clear and functional (if patched)

5. Support bundle check:
   - Confirm diagnostics/export contains enough evidence for future triage.

## Release Steps (End State)
1. Update release notes with only confirmed fixes.
2. Bump version to `5.1.3` where applicable.
3. Tag and publish release from this branch.
4. Post-release:
   - comment on fixed issues with exact version and validation notes
   - close only issues that are truly verified
5. Back-merge/cherry-pick hotfix commits into forward branch:
   - `pulse/unified-resource-pre-hotfix-2026-02-07` (or newer forward branch)

## Definition of Done
- `v5.1.3` shipped from hotfix branch.
- P0 reliability regressions fixed and validated.
- Release notes are factual and test-backed.
- Hotfix commits are propagated back to forward branch.
- Customer follow-up (including Cosmin) can be sent with confidence and concrete fixes.

## Notes for Customer Comms (When Ready)
- Lead with verified outcomes, not hypotheses.
- For each reported symptom:
  - what was wrong
  - what changed in `5.1.3`
  - what the customer should expect now
  - what to send if still reproducible (diagnostics bundle path)
