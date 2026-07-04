# Prerelease-to-GA Rehearsal Record

- Rehearsal date: 2026-07-04
- Result: pass
- GitHub Actions run URL: https://github.com/rcourtman/Pulse/actions/runs/28702063918
- Source branch: pulse/v6-release
- Source commit: 63bbbb1508d9f102cc2679c76825af879d2cb56b
- Version under rehearsal: 6.0.0
- Candidate stable tag: v6.0.0
- Promotion channel: stable
- Promoted prerelease tag: v6.0.0-rc.7
- Current rollback target: v5.1.35
- Exact rollback or reinstall command: `./scripts/install.sh --version v5.1.35`
- Prerelease soak hours at rehearsal time: 149
- Exact GA date to publish: 2026-07-04
- Exact v5 end-of-support date to publish: 2026-10-02
- Dry-run artifact source: `/var/folders/vg/9hdntqw90fn2662q1nsqrmh80000gn/T/rc-to-ga-rehearsal.pecfkbfe/rc-to-ga-rehearsal-summary.md`
- Hotfix exception: false
- Workflow operator note: Governed release rehearsal for 6.0.0 on 2026-07-04

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

- Workflow run: https://github.com/rcourtman/Pulse/actions/runs/28702063918
- Branch: pulse/v6-release
- Version: 6.0.0
- Candidate stable tag: v6.0.0
- Promotion channel: stable
- Promoted prerelease tag: v6.0.0-rc.7
- Rollback target: v5.1.35
- Rollback command: `./scripts/install.sh --version v5.1.35`
- Prerelease soak hours at rehearsal time: 149
- Planned GA date: 2026-07-04
- Planned v5 end-of-support date: 2026-10-02
- Hotfix exception: false
- Operator note: Governed release rehearsal for 6.0.0 on 2026-07-04

## Result

This run exercised the non-publish release path and validated the current promotion contract on the selected branch.
Record this run URL in the release ticket when clearing `rc-to-ga-promotion-readiness`.

## Governed Record

Materialize the dated rehearsal record from this exact run with:
`python3 scripts/release_control/record_rc_to_ga_rehearsal.py --run-id 28702063918`

If you do not pass `--output`, the recorder writes to `docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-rehearsal-<record-date>.md`.
```
