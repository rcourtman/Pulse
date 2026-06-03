# Prerelease-to-GA Rehearsal Record

- Rehearsal date: 2026-06-03
- Result: pass
- GitHub Actions run URL: https://github.com/rcourtman/Pulse/actions/runs/26900927235
- Source branch: pulse/v6-release
- Source commit: 0d0eb4bf11e51a55d834d5a9df71bc6214f0ef40
- Version under rehearsal: 6.0.0
- Candidate stable tag: v6.0.0
- Promotion channel: stable
- Promoted prerelease tag: v6.0.0-rc.6
- Current rollback target: v5.1.34
- Exact rollback or reinstall command: `./scripts/install.sh --version v5.1.34`
- Prerelease soak hours at rehearsal time: 164
- Exact GA date to publish: 2026-06-03
- Exact v5 end-of-support date to publish: 2026-09-01
- Dry-run artifact source: `/var/folders/vg/9hdntqw90fn2662q1nsqrmh80000gn/T/rc-to-ga-rehearsal.4tjrwjru/rc-to-ga-rehearsal-summary.md`
- Hotfix exception: false
- Workflow operator note: Governed release rehearsal for 6.0.0 after backend gate fix

## Verification Notes

1. Confirmed the rehearsal was generated from the GitHub `Release Dry Run` workflow.
2. Confirmed the non-publish release path was exercised end to end up to, but not including, publication.
3. Confirmed rollback target and exact rollback command are recorded explicitly for the promotion candidate.
4. Confirmed the v5 maintenance-only policy remains the governing support contract for the GA handoff.
5. Confirmed the linked artifact is the machine-generated `rc-to-ga-rehearsal-summary` for this run.

## Follow-Up

1. None.

## Dry-Run Artifact

```md
# Prerelease-to-GA Rehearsal Summary

- Workflow run: https://github.com/rcourtman/Pulse/actions/runs/26900927235
- Branch: pulse/v6-release
- Version: 6.0.0
- Candidate stable tag: v6.0.0
- Promotion channel: stable
- Promoted prerelease tag: v6.0.0-rc.6
- Rollback target: v5.1.34
- Rollback command: `./scripts/install.sh --version v5.1.34`
- Prerelease soak hours at rehearsal time: 164
- Planned GA date: 2026-06-03
- Planned v5 end-of-support date: 2026-09-01
- Hotfix exception: false
- Operator note: Governed release rehearsal for 6.0.0 after backend gate fix

## Result

This run exercised the non-publish release path and validated the current promotion contract on the selected branch.
Record this run URL in the release ticket when clearing `rc-to-ga-promotion-readiness`.

## Governed Record

Materialize the dated rehearsal record from this exact run with:
`python3 scripts/release_control/record_rc_to_ga_rehearsal.py --run-id 26900927235`

If you do not pass `--output`, the recorder writes to `docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-rehearsal-<record-date>.md`.
```
