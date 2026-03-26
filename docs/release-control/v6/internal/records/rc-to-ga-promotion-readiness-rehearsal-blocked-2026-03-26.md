# Prerelease-to-GA Rehearsal Blocked Record

- Date: `2026-03-26`
- Gate: `rc-to-ga-promotion-readiness`
- Workflow run: `https://github.com/rcourtman/Pulse/actions/runs/23594550794`
- Branch: `pulse/v6-release`
- Commit: `4855b85e826759635cd12bb7b6cc9fc08038ca07`
- Result: `blocked`

## Live Rehearsal Result

1. Dispatched the real GitHub Actions `Release Dry Run` workflow against
   `pulse/v6-release` with governed stable-promotion inputs:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.1`
   - `rollback_version=5.1.14`
   - `ga_date=2026-04-15`
   - `v5_eos_date=2026-10-31`
2. GitHub accepted the dispatch on the selected remote ref. This is not the
   older default-branch workflow-input rejection failure mode.
3. The run passed `Validate release ref`, `Checkout repository`, and `Resolve required release branch`.
4. The run failed at `Resolve rehearsal metadata` with the exact workflow error:
   - `VERSION file (6.0.0-rc.1) does not match rehearsal version (6.0.0).`
5. The machine-generated `rc-to-ga-rehearsal-summary` artifact failed closed and
   did not mint a promotion metadata envelope.

## Why The Gate Remains Blocked

This live external rehearsal proves the remaining blocker is now the actual
candidate state, not GitHub dispatch mechanics or remote branch drift. The
selected remote ref is current, but the branch is still governed as
`VERSION=6.0.0-rc.1`, so a stable `6.0.0` rehearsal cannot yet succeed.

## Required Unblock Steps

1. Ship a real governed v6 prerelease so `promoted_from_tag` is no longer only an accidental local tag history reference.
2. Move the governed stable candidate on `pulse/v6-release` to `VERSION=6.0.0` only when that promotion is truly intended.
3. Re-run `Release Dry Run` against `pulse/v6-release` with the real published prerelease tag and final GA/EOS dates.
4. Capture a passing `rc-to-ga-rehearsal-summary` artifact that contains the canonical promotion metadata envelope instead of the current fail-closed summary.

## Failing Artifact

```md
# Prerelease-to-GA Rehearsal Summary

- Workflow run: https://github.com/rcourtman/Pulse/actions/runs/23594550794
- Branch: pulse/v6-release
- Result: failure

## Result

This run did not produce a valid promotion metadata envelope.
Do not use this artifact to clear `rc-to-ga-promotion-readiness`.
Fix the failed rehearsal metadata or branch-state preconditions and rerun the workflow.
```
