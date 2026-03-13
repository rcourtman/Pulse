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
4. The active local `pulse/v6` branch now reports `VERSION=6.0.0`, so a local
   GA candidate exists.
5. The local `pulse/v6` branch is ahead of `origin/pulse/v6` by `1447`
   commits, so the current governed release-control state and the current
   `6.0.0` GA candidate are not yet present on the remote branch where GitHub
   Actions runs.
6. `docs/releases/RELEASE_NOTES_v6.md` and
   `docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md` already carry the
   exact governed dates:
   - `v6` GA date: `2026-03-13`
   - `v5` end-of-support date: `2026-06-11`
7. There is still no governed `Release Dry Run` artifact or rehearsal record
   exercising stable inputs for:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.1`
   - an explicit `rollback_version`
   - `v5_eos_date=2026-06-11`

## Why The Gate Cannot Be Cleared Yet

The GA candidate and the required v5 support-policy dates now exist locally, so
the blocker is no longer missing governance text or a missing `6.0.0` version
bump. The real blocker is operational: the exact `6.0.0` candidate and current
release-governance state have not been pushed to `origin/pulse/v6`, and there
is still no matching `Release Dry Run` rehearsal artifact for that candidate.

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
