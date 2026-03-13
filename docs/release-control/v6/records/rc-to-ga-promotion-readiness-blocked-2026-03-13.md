# RC-to-GA Promotion Readiness Blocked Record

- Date: `2026-03-13`
- Gate: `rc-to-ga-promotion-readiness`
- Result: `blocked`

## Blocking Facts

1. The only shipped Pulse v6 RC tag is `v6.0.0-rc.1`.
2. That RC tag resolves to commit `7ac2fd682cb2facc48950371ea4fce4c885c4ea1`.
3. `origin/main` is still on the v5 line and reports `VERSION=5.1.23`.
4. `origin/pulse/v6` reports `VERSION=6.0.0-rc.1`.
5. `v6.0.0-rc.1` is not an ancestor of `origin/main`.
6. The release workflow contract requires stable `6.0.0` promotion to run from
   `main`, not from `pulse/v6`.
7. The recent `Release Dry Run` runs on `main` are from November 2025 and do
   not cover the current v6 GA candidate.

## Why The Gate Cannot Be Cleared Yet

The stable/GA rehearsal for `6.0.0` cannot be exercised honestly until the
candidate release line exists on `main`. Running the stable workflow against
today's `main` would validate the wrong product line, and running it from
`pulse/v6` would violate the release workflow contract.

## Required Unblock Steps

1. Put the exact v6 GA candidate onto `main`.
2. Run `Release Dry Run` from `main` with:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.1`
   - `rollback_version=v5.1.23`
   - the exact published `v5_eos_date`
3. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.
4. Materialize the final rehearsal record from that artifact.
5. Change the gate from `blocked` only if the rehearsal passes and the rollout
   inputs remain explicit.
