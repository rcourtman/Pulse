# Pulse v6 Prerelease Runbook

This runbook captures the branch-specific operational path that was used while
`main` continued to serve v5 releases during the v6 prerelease period.

Canonical customer-channel and promotion rules now live in
`docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`.
Current release-branch authority lives in
`docs/release-control/control_plane.json`.
If this historical runbook and the release-control policy disagree, the
release-control policy wins.

## Branch Model (Current)

- `main`: v5 stable (current public/stable line)
- `pulse/v6-release`: active v6 prerelease and stable release line until an explicit
  post-GA branch cutover is governed

Do not move `main` to v6 during prerelease.

## Enforced Workflow Policy

Release workflows now enforce branch/tag lineage rules:

- `Pulse Release Pipeline` (`create-release.yml`)
  - Resolves stable versus prerelease branch requirements from
    `docs/release-control/control_plane.json`.
  - For the current v6 profile, both stable and prerelease releases dispatch
    from `pulse/v6-release`.
- `publish-docker.yml`, `promote-floating-tags.yml`, `publish-helm-chart.yml`,
  and `update-demo-server.yml`
  - Validate the release tag commit is reachable from the governed branch for
    that version instead of assuming `main`.
- `update-demo-server.yml`
  - Routes stable tags to the stable demo environment.
  - Prerelease public demo deployment is retired after v6 GA; prerelease tags
    must not create or update a second public v6 demo surface by default.
  - The selected tag must still be reachable from the governed branch for that
    version.

This prevents accidental cross-line releases from non-governed branches even if
the stable branch changes later.

## Important Scope Note

The Pulse release workflow in this repo (`.github/workflows/create-release.yml`) builds from the checked-out `Pulse` ref and runs `./scripts/build-release.sh`, which builds `./cmd/pulse`.

It does not automatically check out or build `pulse-enterprise`.

That means public `pulse-v...` release archives are OSS runtime artifacts. They must not be
described as including Pulse Pro runtime features unless a separate Pro package has been built
from `pulse-enterprise` against the same Pulse ref and version.

Paid-user GA is blocked until the Pro release artifacts are built and wired into the paid
install/upgrade path. The current Pro packaging path lives in `pulse-enterprise`:

- `.github/workflows/build-pro-release.yml`
- `scripts/build-pro-release.sh`

The paid-user promise is only satisfied when paid customers are directed to `pulse-pro-v...`
artifacts, or to an explicitly verified paid container image, rather than the public OSS
`pulse-v...` archives.

## Versioning Rules

- v5 stable examples: `5.1.14`
- v6 prerelease examples: `6.0.0-rc.1`, `6.0.0-rc.2`, `6.0.0-rc.3`, `6.0.0-rc.4`, `6.0.0-rc.5`, `6.0.0-rc.6`, `6.0.0-rc.7`
- v6 GA example: `6.0.0`

The workflow auto-marks `-rc.N`/`-alpha.N`/`-beta.N` as prerelease.

## Preconditions for Each RC

1. `pulse/v6-release` is pushed and green in CI.
2. `VERSION` file in `pulse/v6-release` exactly matches the release input version.
3. The current RC release packet is prepared and internally linked:
   - release notes
   - changelog
   - operator support pack
   - `docs/RELEASE_NOTES.md` points at the current in-repo draft packet
4. `PULSE_LICENSE_PUBLIC_KEY` secret is present in GitHub Actions.
5. For any build after `v6.0.0-rc.2`, operators know the update signer changed.
   Hosts pinned to the historical `rc.2` trust root must not assume unattended
   continuity into later prerelease or GA artifacts; use a manual reinstall or
   other explicit trust-migration path before testing those newer packets.
6. For paid-user GA, run the `pulse-enterprise` Pro release workflow against the
   same Pulse ref/version, verify `pulse-pro-v...` archives exist, verify
   `bin/pulse --version` identifies `Pulse Pro`, and confirm the paid
   install/upgrade docs point paid customers to the Pro artifacts or verified paid
   container image.

## RC Release Steps

1. Update version on `pulse/v6-release`:

```bash
export RC_VERSION="6.0.0-rc.7"

git checkout pulse/v6-release
git pull --ff-only
printf '%s\n' "$RC_VERSION" > VERSION
git add VERSION
git commit -m "chore(release): bump version to ${RC_VERSION}"
git push origin pulse/v6-release
```

2. Optional preflight dry run:
   - Run workflow: `Release Dry Run`
   - Ref: `pulse/v6-release`
   - Inputs:
     - `version`: `RC_VERSION`
     - optional `note`

3. Dispatch through the canonical file-backed helper:

   ```bash
   RELEASE_NOTES_FILE="docs/releases/RELEASE_NOTES_v${RC_VERSION}.md"
   ./scripts/trigger-release.sh "$RC_VERSION" "$RELEASE_NOTES_FILE"
   ```

   - Keep the current release-notes, changelog, and operator-support packet in
     sync. Do not update only one of them and treat the packet as ready.
   - Do not paste multiline release notes into the GitHub workflow form as the
     normal operator path. The helper sends the file through JSON transport so
     line breaks are preserved.
   - The workflow independently rejects flattened Markdown before it creates a
     tag or draft, then compares GitHub's stored body byte-for-byte with the
     rendered file before asset upload.

4. Validate the integrated release result:
   - Treat `Definitive Release Verdict` as the release result.
   - Confirm assets exist and checksums match.
   - Confirm GitHub marks the release as a prerelease.
   - Confirm the rendered release body retains its standalone headings, lists,
     code fences, Installation section, and Promotion Metadata section.
   - Smoke install on a test host/container.

5. Retry an unpublished draft only through the same helper:
   - Use the same version and exact release-notes file.
   - Existing unpublished draft releases for the same tag are updated in place
     and their tag is retargeted to the current governed release-line head
     automatically. Do not delete the tag manually just to retry publication.
   - A malformed body edit is quarantined back to draft without deleting valid
     assets; correct the notes file and rerun the canonical helper.
   - Historical prerelease publications used a separate preview demo runtime
     while v5 remained the public stable demo.
   - Prerelease public demo deployment is retired after v6 GA; future
     prerelease tags must not create or update a second public v6 demo surface
     unless a new governed preview target is explicitly introduced.

6. Canary rollout:
   - Upgrade a small user subset first.
   - Collect regressions, fix on `pulse/v6-release`, then cut later RCs as needed.

## Keep v5 Stable During v6 RC

- Continue v5 patch releases from `main` as normal.
- Do not merge `pulse/v6-release` into `main` during prerelease.
- Keep v5 and v6 changelogs/release notes separate.
- Do not rewrite shipped RC notes in place. Each RC should get its own draft or
  published release-notes packet so `rc.1`, `rc.2`, and later prerelease
  support context remain historically accurate.

## GA Cutover (Only After RC Confidence)

Do this only when v6 is proven stable in production-like usage.

1. Create v5 long-tail branch from current `main`:

```bash
git checkout main
git pull --ff-only
git checkout -b pulse/v5-maintenance
git push -u origin pulse/v5-maintenance
```

2. Keep the governed v6 release line on `pulse/v6-release` for GA:

```bash
git checkout pulse/v6-release
git pull --ff-only
```

3. Release `6.0.0` from `pulse/v6-release` using `Pulse Release Pipeline`.
   Before the real GA publish, run `./scripts/trigger-release-dry-run.sh 6.0.0`
   from `pulse/v6-release`. That helper validates the default-branch workflow-dispatch
   contract before calling GitHub so stale `main` workflow inputs fail locally
   instead of returning an opaque 422 from `gh workflow run`.
   The governed `Release Dry Run` must still carry:
   - `version`: `6.0.0`
   - `promoted_from_tag`: exact prerelease tag being promoted
   - `rollback_version`: prior stable
   - `ga_date`: exact published v6 GA date
   - `v5_eos_date`: exact published v5 end-of-support date
   - optional `hotfix_exception` and `hotfix_reason`
   After the run passes, materialize the governed dated rehearsal record with
   `python3 scripts/release_control/record_rc_to_ga_rehearsal.py --run-id <run-id>`.
   If `--output` is omitted, that recorder writes to
   `docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-rehearsal-<record-date>.md`.
   Attach that record, the `rc-to-ga-rehearsal-summary` artifact, and the run URL
   to the release ticket, and confirm the artifact carries the canonical promotion
   metadata envelope for that candidate: candidate stable tag, promotion channel,
   promoted prerelease tag, rollback target, exact rollback command, planned GA date,
   and planned v5 end-of-support date.

4. Publish the exact v6 GA date and v5 end-of-support date in the GA release
   notice using
   `docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md` as the canonical
   policy.

5. Continue critical v5 fixes from `pulse/v5-maintenance` only.
   - The support window lasts 90 calendar days from v6 GA.
   - Only critical security issues, critical correctness/data-loss issues,
     installer or updater failures, licensing or billing blockers, and safe
     migration blockers are eligible during that window.
   - After that window, v5 is unsupported.
6. Treat any future move of stable v6 releases away from `pulse/v6-release` as a
   separate post-GA governance change; do not assume an automatic cutover to
   `main`.

## Rollback Strategy

If an RC is bad:

1. Do not promote to GA.
2. Keep fixing on `pulse/v6-release`.
3. Cut next RC.
4. Keep v5 users on `main` stable releases.

If GA has a severe regression:

1. Patch quickly on `pulse/v6-release` (v6.0.1), or
2. Advise affected users to hold at prior stable while fix ships.
3. Continue v5 emergency fixes from `pulse/v5-maintenance` only if the
   published maintenance-only window is still active or I explicitly announce
   an exception.

## Minimal Per-Release Checklist

1. Version file matches workflow input.
2. CI green on release ref.
3. Draft release validated.
4. Checksums and assets verified.
5. Canary cohort upgraded successfully.
6. Rollback note prepared before publish.
