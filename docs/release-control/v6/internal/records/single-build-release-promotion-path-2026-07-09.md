# Single-Build Release Promotion Path - 2026-07-09

## Scope

Make the normal RC, stable, and patch release path one unattended workflow
whose public promotion and definitive verdict complete materially faster than
the v6.0.5 path without weakening release gates.

## v6.0.5 Timing Baseline

- The successful public release run `29022145812` started at
  `2026-07-09T13:38:55Z` and did not finish its awaited private-runtime work
  until `2026-07-09T15:45:54Z`.
- Pre-publication checks consumed approximately 36 minutes because integration
  tests waited for the 20-minute backend suite.
- `create_release` then consumed another 21 minutes. Its signed release build
  alone took 18 minutes and 44 seconds even though the same SHA had already
  completed release checks.
- Post-publication asset validation consumed 22 minutes and 29 seconds because
  it downloaded the complete 213-asset, multi-gigabyte release packet.
- Private Pro build and promotion consumed 46 minutes and 35 seconds, but did
  not start until release-asset validation finished.
- Stable patch operation also required a separate 37-minute exact-SHA dry run,
  making the safe operator path additive rather than a single release run.

## Canonical Correction

1. `.github/workflows/build-release-candidate.yml` builds and locally validates
   one signed candidate for the exact pushed SHA. Required publication runs
   first produce Developer ID/notarized macOS agents and Authenticode-signed
   Windows agents in parallel, then assemble those exact bytes into the
   candidate. It emits a one-day candidate artifact plus a small manifest
   containing version, tag, source SHA, filenames, sizes, and SHA-256 values.
2. Candidate construction runs in parallel with frontend, backend, Docker,
   Helm, and integration checks. Frontend lint/build runs once and provides one
   verified bundle to backend and integration jobs. Backend and integration no
   longer serialize on each other.
3. `create_release` downloads and verifies the immutable candidate instead of
   rebuilding release assets. Tag, draft, and publication mutations remain
   downstream of every required check.
4. Standard post-upload validation compares the candidate manifest with
   GitHub's server-side release-asset SHA-256 digests and sizes. The legacy
   full-download validator remains available for manual repair or release-edit
   validation when no same-run manifest exists.
5. Docker publication, candidate-backed release-asset validation, and private
   Pro publication start independently after release creation. Floating tags
   still require both Docker and asset validation; install smoke, Helm, stable
   demo, paid-runtime promotion, and the definitive verdict retain their
   applicable safety dependencies.
6. `Release Dry Run` calls the same candidate builder for a no-public-release
   rehearsal, including the native platform signing lanes. Normal release
   publication does not require a separate dry run because `create-release.yml`
   contains the exact-SHA candidate and test gates before publication.
7. Release tarball validation extracts all required entries from each archive
   in one pass. The previous per-entry list/type/content checks repeatedly
   decompressed multi-gigabyte archives and consumed more than 15 minutes.
8. Native signing configuration now has one cheap fail-fast job that reports
   every missing repository secret before macOS or Windows runners are
   allocated.

## Timing Contract

Using the v6.0.5 timings as the baseline:

- the public release boundary should normally be reached within 35 minutes of
  dispatch rather than after a separate rehearsal plus approximately 58
  minutes of checks and rebuilding;
- the definitive cross-product verdict should normally complete within 80
  minutes, with the private Pro build as the expected critical path rather than
  serial release-asset downloads; and
- operator attention remains limited to preparing the release packet and one
  dispatch. Runner queueing or external registry degradation may extend wall
  time, but no standard job may reintroduce duplicate release builds, complete
  packet downloads, or avoidable serial dependencies.

## Remote Rehearsal Findings

- Run `29051381272` proved that the initial event-name condition incorrectly
  skipped the candidate job. The condition now keys off the non-empty manual
  `version` input, with a regression contract.
- Run `29051903335` built the exact-SHA signed candidate for 19 minutes and 9
  seconds. It then exposed two independent late failures: an unmanaged service
  discovery backfill raced temporary-store cleanup in the backend suite, and
  candidate validation exceeded the original 35-minute job ceiling. Backfill
  shutdown is now cancellable and joined; the focused test passes 50 repeats,
  10 complete package repeats, and five race-enabled repeats. The candidate
  ceiling is now 60 minutes.
- The same run showed the validator still decompressing each large tarball for
  every required entry. The replacement validated 15 real archive entries in
  one extraction pass in approximately one second locally.
- Run `29055137616` exercised the actual native-signing graph. macOS failed in
  53 seconds because `APPLE_DEVELOPER_ID_CERTIFICATE_P12_BASE64` was empty;
  Windows failed in 38 seconds because
  `WINDOWS_CODE_SIGNING_CERTIFICATE_PFX_BASE64` was empty. Repository secret
  inventory confirmed that none of the required native-signing names exists.
  Independently, the complete frontend, backend, binary, container, and
  integration preflight passed in 39 minutes and 29 seconds. The chained
  no-mutation demo check then passed its Tailscale, SSH, runtime, frontend,
  public-health, and browser checks in 1 minute and 8 seconds.
- Run `29055794064` exercised the fail-fast configuration gate on exact SHA
  `a3ef1226b7dde7bc661d787fcaddf7256d9fb6b2`. It reported all eight absent
  repository secret names in approximately one second and skipped both native
  runner jobs and candidate construction.
- The published `v6.0.5` release remains non-draft and non-prerelease with its
  original 213 assets and latest asset timestamp `2026-07-09T14:36:37Z`.
  `https://demo.pulserelay.pro/api/version` remains stable `6.0.5`, and
  `/api/health` remains healthy for monitor, scheduler, and websocket.

## External Owner Action

Add these GitHub Actions repository secrets for `rcourtman/Pulse`, using the
matching Apple Developer and Windows code-signing credentials:

- `APPLE_DEVELOPER_ID_CERTIFICATE_P12_BASE64`
- `APPLE_DEVELOPER_ID_CERTIFICATE_PASSWORD`
- `APPLE_DEVELOPER_ID_APPLICATION_IDENTITY`
- `APPLE_NOTARY_KEY_P8_BASE64`
- `APPLE_NOTARY_KEY_ID`
- `APPLE_NOTARY_ISSUER_ID`
- `WINDOWS_CODE_SIGNING_CERTIFICATE_PFX_BASE64`
- `WINDOWS_CODE_SIGNING_CERTIFICATE_PASSWORD`

After all eight names are configured, rerun `Release Dry Run` for the exact
current `main` SHA. It must pass native signing, build and validate the
candidate, upload both candidate artifacts, pass release checks, complete
no-mutation demo verification, and leave the published `v6.0.5` release and
demo runtime unchanged.

## 2026-07-09 Verdict

Blocked only on the absent native platform signing credentials. Repository-side
orchestration, diagnostics, timeout hardening, archive validation performance,
and late backend-flake containment are implemented and covered by contracts.

## Prerelease Windows Signing Boundary (2026-07-10)

The first `v6.0.6-rc.1` publication attempt proved Apple signing and
notarization credentials are configured, but Windows Authenticode credentials
are not. Pulse Monitoring Ltd submitted the public community project to the
SignPath Foundation open-source programme on 2026-07-10; approval and CI
integration remain externally owned and asynchronous.

RC publication may proceed while that application is pending, provided all of
the following remain true:

- macOS agent binaries are Developer ID signed and notarized;
- Windows agent binaries retain the exact-SHA candidate, checksums, detached
  release signatures, and post-publication digest verification;
- the RC release notes state explicitly that Windows binaries are not
  Authenticode-signed and may show an unknown-publisher warning; and
- stable publication continues to require successful Windows Authenticode
  signing and verification.

The reusable candidate workflow therefore separates the macOS platform-signing
requirement from the Windows Authenticode requirement. `create-release.yml`
requires Windows signing for stable versions and relaxes only that requirement
for recognized prerelease versions. `release-dry-run.yml` requires both
platforms for stable-version rehearsals and applies the same bounded Windows
exception to prerelease rehearsals. The `single-build-release-promotion-path`
release gate remains blocked until the external stable signing path is proven.

## Repository Advancement (2026-07-21)

Stable publication and stable dry-run callers now select SignPath instead of
assuming an exportable certificate PFX in GitHub. The reusable workflow uploads
the exact-SHA unsigned Windows binaries, submits their artifact id through the
pinned official SignPath v2 action, verifies all returned executables, and
records the request, run, source SHA, signer identity, and file digests beside
the immutable candidate manifest. `Release Dry Run` now has a terminal verdict
covering candidate signing and no-mutation demo verification.

The repository inventory still contains no `SIGNPATH_API_TOKEN` and no SignPath
project-coordinate variables. The latest successful prerelease dry run remains
valid only under the unsigned-Windows RC exception and does not clear stable
readiness.

## Exact Remaining External Action (2026-07-21)

Complete the SignPath setup in
`signpath-windows-authenticode-integration-packet-2026-07-21.md`, authorize its
GitHub App for `rcourtman/Pulse`, add the named secret and variables, and
approve one stable-version `Release Dry Run` request for the exact current
`main` SHA. The signed candidate, release checks, no-mutation stable-demo lane,
and Definitive Dry-Run Verdict must pass. Do not publish a stable release as
part of this proof run.

## v6.1.0 Owner-Bounded Stable Rehearsal (2026-07-22)

The release owner ended the moving release-candidate loop, waived the remaining
v6.1.0 prerelease soak, and approved one stable-release exception for unavailable
Windows Authenticode signing. The exception is limited by the resolver and
workflow contracts to stable `v6.1.0`, requires a non-empty owner reason, and
requires the public release notes to state that Windows binaries are not
Authenticode-signed and may show an unknown-publisher warning. macOS signing and
notarization remain mandatory, as do exact-SHA identity, checksums, detached
signatures, the immutable candidate manifest, and post-publication digest
validation. Stable releases after `v6.1.0` automatically restore the Windows
Authenticode requirement.

Release Dry Run `29936239561` exercised the exception on exact source SHA
`0b5763764cea3e1d93aacfe2b84ddfe278083409` without creating a tag or modifying
the public release:

- `Preflight Release Checks (No Publish)` passed in 40 minutes 50 seconds,
  including the frontend, backend, integration, Docker, Helm, and mobile gates.
- `Sign and Notarize macOS Agent` passed in 2 minutes 48 seconds.
- The version-bound Windows Authenticode job was intentionally skipped, and
  `Build and Validate Release Candidate` passed in 21 minutes 18 seconds using
  the retained unsigned-Windows verification controls.
- The no-mutation current-stable demo check passed, followed by the terminal
  `Definitive Dry-Run Verdict`.

The complete successful rehearsal is recorded at
`https://github.com/rcourtman/Pulse/actions/runs/29936239561`. This clears the
previous external-rehearsal prerequisite under the recorded v6.1.0-only owner
exception; the matching public promotion must still finish its own definitive
verdict before the release operation is considered complete.

## Release Pipeline Defect Reopen (2026-07-22)

The first public-promotion dispatch, run `29939510627`, preserved the mutation
boundary and created no tag, draft, or release. Its integration job failed
deterministically before publication because `create-release.yml` still named
`tests/03-multi-tenant.spec.ts` after that whole file had moved into the
Playwright quarantine on 2026-07-17. Playwright correctly returned `No tests
found`; rerunning the same SHA could not change that result. The release owner
cutoff explicitly permits a release-pipeline defect to reopen v6.1.0.

The corrected release job targets the current stable organization-sharing UI
spec instead, while retaining the real container build, bootstrap-token check,
update-route smoke, backend suite, and all other self-contained publication
gates. The installer workflow contract now rejects any attempt to point the
release job back at the quarantined multi-tenant file. A fresh exact-SHA dry run
is required before the corrected public promotion is dispatched.

## v6.1.0 Stable Promotion Verdict (2026-07-22)

Corrected Release Dry Run `29941836354` passed on exact source SHA
`e1f33c1bad6831ea00a2824b39d259fb8a071508`. Its preflight, macOS signing and
notarization, immutable candidate, no-mutation stable-demo verification, and
terminal `Definitive Dry-Run Verdict` all succeeded. The Windows Authenticode
job was skipped only under the recorded stable-v6.1.0 exception; the retained
Windows exact-SHA, manifest, checksum, detached-signature, and digest controls
passed.

Canonical public release run `29945066337` then promoted that same candidate
without rebuilding it. The corrected integration gate, candidate validation,
release creation, 213-asset validation, Docker and Helm publication, install
smoke, stable demo, floating-tag update, private Pro runtime publication, and
terminal `Definitive Release Verdict` all succeeded. The downstream private
workflows also completed successfully:

- Pro build: `https://github.com/rcourtman/pulse-enterprise/actions/runs/29946936202`
- paid-runtime promotion: `https://github.com/rcourtman/pulse-pro/actions/runs/29949536073`

The public non-prerelease release is
`https://github.com/rcourtman/Pulse/releases/tag/v6.1.0`. Its annotated
`v6.1.0` tag peels exactly to
`e1f33c1bad6831ea00a2824b39d259fb8a071508`, and the release target identifies
the same commit. The complete asset set includes checksums and detached
`.sig`/`.sshsig` signatures. The public notes disclose that the Windows
binaries are not Authenticode-signed and may show `Unknown Publisher`, and
retain the `v6.0.5` rollback command. macOS artifacts are signed and notarized.

The owner-bounded v6.1.0 exception is therefore exercised and closed. Stable
versions after v6.1.0 automatically restore the Windows Authenticode
requirement. The `single-build-release-promotion-path` gate verdict is passed.
