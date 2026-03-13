# RC-to-GA Promotion Readiness Blocked Record

- Date: `2026-03-13`
- Gate: `rc-to-ga-promotion-readiness`
- Result: `blocked`

## Blocking Facts

1. The only shipped Pulse v6 RC tag is `v6.0.0-rc.1`.
2. That RC tag resolves to commit `7ac2fd682cb2facc48950371ea4fce4c885c4ea1`.
3. The governed release profile in `docs/release-control/control_plane.json`
   currently declares both `prerelease_branch` and `stable_branch` as
   `pulse/v6`.
4. The active `pulse/v6` branch still reports `VERSION=6.0.0-rc.1`, so there
   is not yet a committed `6.0.0` GA candidate to rehearse.
5. The local `pulse/v6` branch is ahead of `origin/pulse/v6`, so the current
   release-governance state and recent gate records are not yet present on the
   remote branch where GitHub Actions runs.
6. The latest successful `Release Dry Run` on `pulse/v6` was
   `https://github.com/rcourtman/Pulse/actions/runs/22289311916` on
   `2026-02-23`, before the current GA-promotion governance state existed.
7. There is no current GitHub `Release Dry Run` exercising stable inputs for
   `version=6.0.0`, `promoted_from_tag=v6.0.0-rc.1`, an explicit
   `rollback_version`, and the exact `v5_eos_date`.

## Why The Gate Cannot Be Cleared Yet

The stable/GA rehearsal for `6.0.0` cannot be exercised honestly until the
exact GA candidate exists on the governed stable branch and is available to the
GitHub workflow runner. The branch policy is no longer the blocker; the real
blocker is that there is still no pushed `6.0.0` candidate commit plus a
matching `Release Dry Run` run for that candidate.

## Required Unblock Steps

1. Commit the exact v6 GA candidate on `pulse/v6`, including `VERSION=6.0.0`.
2. Push that governed release branch state to `origin/pulse/v6`.
3. Run `Release Dry Run` from `pulse/v6` with:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.1`
   - an explicit stable `rollback_version`
   - the exact published `v5_eos_date`
4. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.
5. Materialize the final rehearsal record from that artifact.
6. Change the gate from `blocked` only if the rehearsal passes and the rollout
   inputs remain explicit.
