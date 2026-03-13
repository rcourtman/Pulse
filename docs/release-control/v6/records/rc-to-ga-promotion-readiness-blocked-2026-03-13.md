# RC-to-GA Promotion Readiness Blocked Record

- Date: `2026-03-13`
- Gate: `rc-to-ga-promotion-readiness`
- Result: `blocked`

## Blocking Facts

1. The only shipped Pulse v6 RC tag is `v6.0.0-rc.1`.
2. That RC tag resolves to commit `ceb23d19a6241efbb548a1f21dccba14de5821dc`.
3. The governed release profile in `docs/release-control/control_plane.json`
   currently declares both `prerelease_branch` and `stable_branch` as
   `pulse/v6`.
4. The active control-plane target is `v6-ga-promotion`, so stable or GA
   promotion is now the governed objective for Pulse v6.
5. The active local `pulse/v6` branch currently reports `VERSION=6.0.0`, so a
   local GA candidate exists on the governed stable line.
6. There is still no governed `RC-to-GA Rehearsal Record` proving a successful
   non-publish `Release Dry Run` for the current `6.0.0` candidate.
7. `docs/releases/RELEASE_NOTES_v6.md` and
   `docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md` already carry the
   exact governed dates:
   - `v6` GA date: `2026-03-13`
   - `v5` end-of-support date: `2026-06-11`
8. There is still no governed `Release Dry Run` artifact or rehearsal record
   exercising stable inputs for:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.1`
   - an explicit `rollback_version`
   - `v5_eos_date=2026-06-11`

## Why The Gate Cannot Be Cleared Yet

The blocker is no longer missing governance text. The remaining problem is that
there is still no exercised `Release Dry Run` record proving the exact `6.0.0`
candidate is ready for GA-style promotion. Until that rehearsal exists, stable
users would still be the first real cohort for the final promotion path.

## Required Unblock Steps

1. Push the governed `pulse/v6` branch state, including the current
   `VERSION=6.0.0` candidate and release-control records, to `origin/pulse/v6`.
2. Run `Release Dry Run` from `pulse/v6` with:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.1`
   - an explicit stable `rollback_version`
   - `v5_eos_date=2026-06-11`
3. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.
4. Materialize the final rehearsal record from that artifact.
5. Change the gate from `blocked` only if the rehearsal passes and the rollout
   inputs remain explicit.
