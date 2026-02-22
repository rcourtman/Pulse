# Pulse v6 Prerelease Runbook

This runbook is the operational path for shipping v6 safely while `main` continues to serve v5 releases.

## Branch Model (Current)

- `main`: v5 stable (current public/stable line)
- `pulse/v6`: v6 development and prereleases

Do not move `main` to v6 during prerelease.

## Important Scope Note

The Pulse release workflow in this repo (`.github/workflows/create-release.yml`) builds from the checked-out `Pulse` ref and runs `./scripts/build-release.sh`, which builds `./cmd/pulse`.

It does not automatically check out or build `pulse-enterprise`.

## Versioning Rules

- v5 stable examples: `5.1.14`
- v6 prerelease examples: `6.0.0-rc.1`, `6.0.0-rc.2`
- v6 GA example: `6.0.0`

The workflow auto-marks `-rc.N`/`-alpha.N`/`-beta.N` as prerelease.

## Preconditions for Each RC

1. `pulse/v6` is pushed and green in CI.
2. `VERSION` file in `pulse/v6` exactly matches the release input version.
3. Release notes are prepared.
4. `PULSE_LICENSE_PUBLIC_KEY` secret is present in GitHub Actions.

## RC Release Steps

1. Update version on `pulse/v6`:

```bash
git checkout pulse/v6
git pull --ff-only
echo "6.0.0-rc.1" > VERSION
git add VERSION
git commit -m "chore(release): bump version to 6.0.0-rc.1"
git push origin pulse/v6
```

2. Optional preflight dry run:
   - Run workflow: `Release Dry Run`
   - Ref: `pulse/v6`
   - Input: optional note

3. Create draft prerelease:
   - Run workflow: `Pulse Release Pipeline`
   - Ref: `pulse/v6`
   - Inputs:
     - `version`: `6.0.0-rc.1`
     - `release_notes`: markdown text
     - `draft_only`: `true`

4. Validate draft outputs:
   - Confirm assets exist and checksums match.
   - Confirm GitHub release is marked prerelease.
   - Smoke install on a test host/container.

5. Publish prerelease:
   - Re-run `Pulse Release Pipeline` on `pulse/v6`
   - Same `version` and notes
   - `draft_only`: `false`

6. Canary rollout:
   - Upgrade a small user subset first.
   - Collect regressions, fix on `pulse/v6`, then cut `rc.2`/`rc.3` as needed.

## Keep v5 Stable During v6 RC

- Continue v5 patch releases from `main` as normal.
- Do not merge `pulse/v6` into `main` during prerelease.
- Keep v5 and v6 changelogs/release notes separate.

## GA Cutover (Only After RC Confidence)

Do this only when v6 is proven stable in production-like usage.

1. Create v5 long-tail branch from current `main`:

```bash
git checkout main
git pull --ff-only
git checkout -b pulse/v5-maintenance
git push -u origin pulse/v5-maintenance
```

2. Promote v6 to `main`:

```bash
git checkout main
git merge --ff-only pulse/v6
git push origin main
```

3. Release `6.0.0` from `main` using `Pulse Release Pipeline`.

4. Continue critical v5 fixes from `pulse/v5-maintenance` only.

## Rollback Strategy

If an RC is bad:

1. Do not promote to GA.
2. Keep fixing on `pulse/v6`.
3. Cut next RC.
4. Keep v5 users on `main` stable releases.

If GA has a severe regression:

1. Patch quickly on `main` (v6.0.1), or
2. Advise affected users to hold at prior stable while fix ships.
3. Continue v5 emergency fixes from `pulse/v5-maintenance` if needed.

## Minimal Per-Release Checklist

1. Version file matches workflow input.
2. CI green on release ref.
3. Draft release validated.
4. Checksums and assets verified.
5. Canary cohort upgraded successfully.
6. Rollback note prepared before publish.
