# RC-to-GA Promotion Readiness Blocked Record

- Date: `2026-03-13`
- Gate: `rc-to-ga-promotion-readiness`
- Result: `blocked`

## Blocking Facts

1. No Pulse v6 RC has shipped yet.
2. The repository contains accidental prerelease git tag history (`v6.0.0-rc.1`),
   but those tags were never published and do not count as shipped RC lineage.
3. GitHub still validates `Release Dry Run` workflow dispatch inputs against the
   default branch `main`, whose current workflow contract does not
   accept the governed stable rehearsal metadata envelope (`promoted_from_tag`,
   `rollback_version`, `ga_date`, and `v5_eos_date`).
4. The governed release profile in `docs/release-control/control_plane.json`
   currently declares both `prerelease_branch` and `stable_branch` as
   `pulse/v6-release`.
5. The active control-plane target is still `v6-rc-stabilization`, not
   `v6-ga-promotion`.
6. The active local `pulse/v6-release` branch currently reports `VERSION=6.0.0-rc.1`, so the
   working line is still prerelease and there is not yet a governed local stable
   `6.0.0` candidate.
7. There is still no governed `RC-to-GA Rehearsal Record` proving a successful
   non-publish `Release Dry Run` for the eventual stable `6.0.0` candidate.
8. `docs/releases/RELEASE_NOTES_v6.md` and
   `docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md` now carry the
   currently proposed exact dates for the eventual GA notice:
   - `v6` GA date: `2026-03-24`
   - `v5` end-of-support date: `2026-06-22`
9. There is still no governed `Release Dry Run` artifact or rehearsal record
   exercising stable inputs for:
   - `version=6.0.0`
   - no governed `promoted_from_tag` exists yet because no RC has shipped
   - the artifact-owned candidate stable tag for that rehearsal
   - the artifact-owned promotion channel for that rehearsal
   - the artifact-owned promoted RC tag for that rehearsal
   - the artifact-owned rollback target for that stable candidate
   - `ga_date=2026-03-24`
   - an explicit `rollback_version`
   - the exact derived rollback command that artifact will publish
   - `v5_eos_date=2026-06-22`

## Why The Gate Cannot Be Cleared Yet

The blocker is no longer missing governance text. The remaining problem is that
the control plane still treats v6 as an RC-stabilization line, the working
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
3. Land the canonical `.github/workflows/release-dry-run.yml` workflow-dispatch input
   contract on the default branch `main` so GitHub accepts the
   governed stable rehearsal metadata envelope when dispatching from `pulse/v6-release`.
4. Ship the first real RC through the governed prerelease release path and record
   its exact published prerelease tag plus rollback target and exact derived
   rollback command.
5. Run `Release Dry Run` from `pulse/v6-release` using that published RC as
   `promoted_from_tag` with:
   - `version=6.0.0`
   - an artifact-owned candidate stable tag matching that rehearsal
   - an artifact-owned promotion channel matching that rehearsal
   - an artifact-owned promoted RC tag matching that rehearsal
   - an artifact-owned rollback target for the stable candidate
   - the exact planned GA and v5 end-of-support dates for the publish notice
   - `ga_date=2026-03-24`
   - an explicit stable `rollback_version`
   - the exact derived rollback command that artifact will publish
   - `v5_eos_date=2026-06-22`
6. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.
7. Materialize the final rehearsal record from that artifact without
   hand-repairing any missing candidate tag, promoted RC tag, rollback
   target, rollback command, or GA/EOS metadata.
8. Change the gate from `blocked` only if the rehearsal passes and the rollout
   inputs remain explicit.
