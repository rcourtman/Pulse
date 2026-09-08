# Releases and update channels

Use **Stable** for production and **Preview** when you want to help test upcoming
changes. Preview releases state what is ready to test, what remains uncertain,
and how to return to the previous stable version.

Pulse's earlier releases did not consistently give testers enough time between
candidates. From September 2026, the release process uses explicit testing stages,
fixed candidates, and observation periods. The aim is to make each release label
dependable, while development continues at its own pace. These are the commitments
for releases going forward, not a claim that earlier releases met them.

## Choose your update channel

| Channel | Who it is for | What to expect |
| --- | --- | --- |
| **Stable** | Normal use, including production | A release qualified for general use after release-candidate testing. This is the default and recommended channel. |
| **Preview** | People willing to test and report problems | Opt-in prereleases. Read the version's maturity, known issues, test instructions, and rollback guidance before installing. |

Preview contains different maturity stages, rather than a separate update channel
for each stage:

- **Alpha** is incomplete or experimental and intended for tightly controlled testing.
- **Beta** is the normal public testing stage. Its scope should be useful to test,
  but known gaps and further product changes are expected.
- **Release candidate (RC)** means the build is believed capable of becoming stable
  without further product changes. It is still a prerelease.

A beta does not become stable directly. It must be followed by a qualified RC.
Choosing Preview does not make a beta production-qualified. Pulse's unattended
systemd updater remains Stable-only. See [Automatic updates](AUTO_UPDATE.md) for
deployment-specific update behaviour.

## How releases are prepared

Each release has a defined scope, testing stage, and set of required checks.
Development can continue while a candidate is being tested without changing
what testers have installed.

1. **Select a fixed candidate.** A release branch holds the selected changes while
   development continues on `main`. Unrelated newer work does not enter a candidate
   merely because it has landed elsewhere.
2. **Qualify it for its testing stage.** Required checks include targeted tests,
   installation smoke checks, release-pipeline validation, and recorded rollback
   instructions. RCs also run the integration checks required for stable releases.
3. **Publish a useful checkpoint.** Release notes explain the changes, known issues,
   what testers should exercise, and the rollback target. A tag or successful build
   alone is not a published release.
4. **Assess the results.** Review covers reported problems, test installations,
   the demo, and available preview telemetry before advancing.
   A defect that invalidates the checkpoint holds it for repair and fresh qualification.

## Cadence and stable promotion

Releases follow readiness rather than a fixed calendar. Compatible fixes are
batched into checkpoints so testers have time to use a build:

- Successive public checkpoints at the same maturity on a version line are spaced
  at least **24 hours** apart. Work and testing continue during that interval.
- A patch RC needs at least **72 hours** of clean observation before stable promotion.
  A minor RC needs at least **seven days**.
- A product change to an RC starts a fresh observation period. Time spent on beta
  does not count towards the RC period.
- Stable retains the qualified RC's product content. Release metadata may change,
  but unrelated product changes cannot be added during promotion.

Elapsed time alone is insufficient. Promotion also requires no open high or
critical candidate issues, clean operation on the project's test installation
and demo throughout the observation period, and assessment of preview failure
signals where available. Sparse telemetry or silence from testers does not prove
that every changed feature was exercised.

An urgent hotfix exception requires active customer harm and a justification
recorded in the release notes. Narrow testing may instead use an immutable reporter
test image. Such an image is a temporary diagnostic build and cannot be promoted
directly to stable.

## Taking part in Preview testing

Start with the [published release notes](https://github.com/rcourtman/Pulse/releases).
Back up your data, keep the documented rollback version available, and exercise
the checks relevant to your installation. You do not need to install every preview.

When [reporting a problem](https://github.com/rcourtman/Pulse/issues/new/choose),
include the exact version, deployment method, expected and observed behaviour,
and reproduction steps. Remove credentials and other private information from
logs and screenshots. Confirmation that a named test worked is useful too.
