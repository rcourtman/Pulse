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
   one signed candidate for the exact pushed SHA. It emits a one-day candidate
   artifact plus a small manifest containing version, tag, source SHA,
   filenames, sizes, and SHA-256 values.
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
   rehearsal. Normal release publication does not require a separate dry run
   because `create-release.yml` contains the exact-SHA candidate and test gates
   before publication.

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

## Verification

Pending one pushed `Release Dry Run` for the exact implementation SHA. The run
must build and validate the signed candidate, upload both candidate artifacts,
pass release checks, complete no-mutation demo verification, and leave the
published `v6.0.5` release and demo runtime unchanged.

## Current Verdict

Blocked until the pushed no-public-release rehearsal succeeds and its measured
job timings are recorded here.
