# Prerelease-to-GA Promotion Readiness Blocked Record

- Date: `2026-03-26`
- Gate: `rc-to-ga-promotion-readiness`
- Result: `blocked`

## Blocking Facts

1. No Pulse v6 prerelease has shipped yet.
2. The repository contains accidental prerelease git tag history (`v6.0.0-rc.1`),
   but those tags were never published and do not count as shipped prerelease lineage.
3. The governed release profile in `docs/release-control/control_plane.json`
   currently declares both `prerelease_branch` and `stable_branch` as
   `pulse/v6-release`.
4. The active control-plane target is still `v6-rc-stabilization`, not
   `v6-ga-promotion`.
5. The active local `pulse/v6-release` branch currently reports `VERSION=6.0.0-rc.1`, so the
   working line is still prerelease and there is not yet a governed local stable
   `6.0.0` candidate.
6. There is still no governed `Prerelease-to-GA Rehearsal Record` proving a successful
   non-publish `Release Dry Run` for the eventual stable `6.0.0` candidate.
7. `docs/releases/RELEASE_NOTES_v6.md` and
   `docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md` still leave the
   GA announcement dates as placeholders because no real prerelease lineage or GA-ready
   rehearsal has locked them yet:
   - `v6` GA date placeholder: `[v6-ga-date]`
   - `v5` end-of-support placeholder: `[v5-eos-date]`
8. There is still no governed `Release Dry Run` artifact or rehearsal record
   exercising stable inputs for:
   - `version=6.0.0`
   - no governed `promoted_from_tag` exists yet because no prerelease has shipped
   - the artifact-owned candidate stable tag for that rehearsal
   - the artifact-owned promotion channel for that rehearsal
   - the artifact-owned promoted prerelease tag for that rehearsal
   - the artifact-owned rollback target for that stable candidate
   - no exact `ga_date` is locked yet because the GA notice is still pending
   - no exact `v5_eos_date` is locked yet because the GA notice is still pending
   - an explicit `rollback_version`
   - the exact derived rollback command that artifact will publish

## Why The Gate Cannot Be Cleared Yet

The blocker is no longer missing governance text. The remaining problem is that
the control plane still treats v6 as the prerelease-stabilization line, the working
version is still prerelease (`6.0.0-rc.1`), and there is still no exercised
`Release Dry Run` record proving the eventual stable `6.0.0`
candidate is ready for GA-style promotion. Until that rehearsal exists, stable
users would still be the first real cohort for the final promotion path.

## Required Unblock Steps

1. Promote the active target from `v6-rc-stabilization` to
   `v6-ga-promotion` only when that change is actually intended.
2. Push the governed `pulse/v6-release` branch state that is intended to become the
   stable `6.0.0` candidate, including the eventual `VERSION=6.0.0`
   change and release-control records, to `origin/pulse/v6-release`.
3. Ship the first real prerelease through the governed prerelease release path and record
   its exact published prerelease tag plus rollback target and exact derived
   rollback command.
4. Run `Release Dry Run` from `pulse/v6-release` using that published prerelease as
   `promoted_from_tag` with:
   - `version=6.0.0`
   - an artifact-owned candidate stable tag matching that rehearsal
   - an artifact-owned promotion channel matching that rehearsal
   - an artifact-owned promoted prerelease tag matching that rehearsal
   - an artifact-owned rollback target for the stable candidate
   - the exact planned GA and v5 end-of-support dates for the publish notice
   - an explicit `ga_date` chosen for that rehearsal
   - an explicit stable `rollback_version`
   - the exact derived rollback command that artifact will publish
   - an explicit `v5_eos_date` chosen for that rehearsal
5. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.
6. Materialize the final rehearsal record from that artifact without
   hand-repairing any missing candidate tag, promoted prerelease tag, rollback
   target, rollback command, or GA/EOS metadata.
7. Change the gate from `blocked` only if the rehearsal passes and the rollout
   inputs remain explicit.
