# Prerelease-to-GA Promotion Readiness Blocked Record

- Date: `2026-04-04`
- Gate: `rc-to-ga-promotion-readiness`
- Result: `blocked`

## Blocking Facts

1. The latest shipped Pulse v6 prerelease tag is `v6.0.0-rc.2`.
2. That shipped prerelease tag resolves to commit `7b0bdebdff8127228708e84eb836cfd3d7214c08`.
3. The governed release profile in `docs/release-control/control_plane.json`
   currently declares both `prerelease_branch` and `stable_branch` as
   `pulse/v6-release`.
4. The active control-plane target is `v6-ga-promotion`, so stable or GA
   promotion is now the governed objective for Pulse v6.
5. The active local `pulse/v6-release` branch currently reports `VERSION=6.0.0`, so a
   local GA candidate exists on the governed stable line.
6. There is still no governed `Prerelease-to-GA Rehearsal Record` proving a successful
   non-publish `Release Dry Run` for the current `6.0.0` candidate.
7. `docs/releases/RELEASE_NOTES_v6.md` and
   `docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md` now carry the
   currently proposed exact dates for the eventual GA notice:
   - `v6` GA date: `2026-04-20`
   - `v5` end-of-support date: `2026-07-19`
8. There is still no governed `Release Dry Run` artifact or rehearsal record
   exercising stable inputs for:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.2`
   - the artifact-owned candidate stable tag for that rehearsal
   - the artifact-owned promotion channel for that rehearsal
   - the artifact-owned promoted prerelease tag for that rehearsal
   - the artifact-owned rollback target for that stable candidate
   - `ga_date=2026-04-20`
   - an explicit `rollback_version`
   - the exact derived rollback command that artifact will publish
   - `v5_eos_date=2026-07-19`

## Why The Gate Cannot Be Cleared Yet

The blocker is no longer missing governance text. The remaining problem is that
there is still no exercised `Release Dry Run` record proving the exact `6.0.0`
candidate is ready for GA-style promotion. Until that rehearsal exists, stable
users would still be the first real cohort for the final promotion path.

## Required Unblock Steps

1. Push the governed `pulse/v6-release` branch state, including the current
   `VERSION=6.0.0` candidate and release-control records, to `origin/pulse/v6-release`.
2. Run `Release Dry Run` from `pulse/v6-release` with:
   - `version=6.0.0`
   - `promoted_from_tag=v6.0.0-rc.2`
   - an artifact-owned candidate stable tag matching that rehearsal
   - an artifact-owned promotion channel matching that rehearsal
   - an artifact-owned promoted prerelease tag matching that rehearsal
   - an artifact-owned rollback target for the stable candidate
   - the exact planned GA and v5 end-of-support dates for the publish notice
   - `ga_date=2026-04-20`
   - an explicit stable `rollback_version`
   - the exact derived rollback command that artifact will publish
   - `v5_eos_date=2026-07-19`
3. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.
4. Materialize the final rehearsal record from that artifact without
   hand-repairing any missing candidate tag, promoted prerelease tag, rollback
   target, rollback command, or GA/EOS metadata.
5. Change the gate from `blocked` only if the rehearsal passes and the rollout
   inputs remain explicit.
