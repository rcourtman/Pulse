# Prerelease-to-GA Rehearsal Record

- Rehearsal date: 2026-08-22
- Result: pass
- GitHub Actions run URL: https://github.com/rcourtman/Pulse/actions/runs/32566890805
- Source branch: main
- Source commit: ca311323e96392efe18038707884283cd59aa71e
- Version under rehearsal: 6.3.0
- Candidate stable tag: v6.3.0
- Promotion channel: stable
- Promoted prerelease tag: v6.3.0-rc.6
- Current rollback target: v6.2.1
- Exact rollback or reinstall command: `./scripts/install.sh --version v6.2.1`
- Prerelease soak hours at rehearsal time: 15
- Exact GA date to publish: 2026-08-22
- Exact v5 end-of-support date to publish: 2026-10-02
- Dry-run artifact source: `/tmp/pulse-ga-record.ouxzO8/rc-to-ga-rehearsal-summary.md`
- Hotfix exception: true
- Hotfix reason: Release owner approved the v6.3.0 cutoff after clean privacy-safe production telemetry from rc.5 and rc.6 and accepted the shortened rc.6 soak and bounded post-RC changes.
- Windows Authenticode required: false
- Unsigned Windows exception: true
- Unsigned Windows reason: Windows Authenticode signing is not yet available; the release owner accepts unsigned Windows Unified Agent artifacts for v6.3.0 with public disclosure and unchanged exact-SHA integrity controls.
- Workflow operator note: GA dry run after joining every exact-SHA compilation task; candidate ca311323e96392efe18038707884283cd59aa71e.
- Additional note: The v6.3.0 stable dry-run summary omitted legacy first-GA date fields; the dated record supplies the 2026-08-22 stable publication date and retains the already-published 2026-10-02 v5 end-of-support date.

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

- Workflow run: https://github.com/rcourtman/Pulse/actions/runs/32566890805
- Branch: main
- Version: 6.3.0
- Candidate stable tag: v6.3.0
- Promotion channel: stable
- Promoted prerelease tag: v6.3.0-rc.6
- Rollback target: v6.2.1
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Prerelease soak hours at rehearsal time: 15
- Planned GA date: 2026-08-22
- Planned v5 end-of-support date: 2026-10-02
- Hotfix exception: true
- Hotfix reason: Release owner approved the v6.3.0 cutoff after clean privacy-safe production telemetry from rc.5 and rc.6 and accepted the shortened rc.6 soak and bounded post-RC changes.
- Windows Authenticode required: false
- Unsigned Windows exception: true
- Unsigned Windows reason: Windows Authenticode signing is not yet available; the release owner accepts unsigned Windows Unified Agent artifacts for v6.3.0 with public disclosure and unchanged exact-SHA integrity controls.
- Operator note: GA dry run after joining every exact-SHA compilation task; candidate ca311323e96392efe18038707884283cd59aa71e.

## Result

This run exercised the non-publish release path and validated the current promotion contract on the selected branch.
Record this run URL in the release ticket when clearing `rc-to-ga-promotion-readiness`.

## Governed Record

Materialize the dated rehearsal record from this exact run with:
`python3 scripts/release_control/record_rc_to_ga_rehearsal.py --run-id 32566890805`

If you do not pass `--output`, the recorder writes to `docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-rehearsal-<record-date>.md`.
```
