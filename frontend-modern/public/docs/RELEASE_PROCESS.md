# Releases and update channels

Use **Stable** for production and **Preview** when you want to help test upcoming
changes. Preview releases state what is ready to test, what remains uncertain,
and how to return to the previous stable version.

Pulse's earlier releases did not consistently give testers enough time between
candidates. From October 2026, releases run on a fixed train: a release candidate is
cut on a regular schedule, tested as published, and promoted to stable unchanged.
These are the commitments for releases going forward, not a claim that earlier
releases met them.

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

Releases follow a fixed schedule rather than individual judgment.

1. **Cut a candidate from `main`.** Fourteen days after the last stable release,
   the next release candidate is cut from `main`, provided `main` has changed since
   that release. An important fix that should not wait for the schedule can bring
   that cut forward, or replace a candidate that is still soaking with a fresh one
   that starts its own soak. A candidate carries everything merged to `main` at the
   cut. Nothing is selected for it or added to it afterwards.
2. **Qualify and publish it.** Required checks include targeted tests, installation
   smoke checks, release-pipeline validation, and recorded rollback instructions.
   RCs also run the integration checks required for stable releases. Release notes
   explain the changes, known issues, what testers should exercise, and the rollback
   target. A tag or successful build alone is not a published release.
3. **Soak it on Preview.** The published candidate soaks for **24 hours** on the
   Preview channel.
4. **Promote it unchanged.** After a clean soak, the same candidate is promoted to
   stable. Release metadata may change, but product content cannot.

## Cadence and stable promotion

- Each release is the next minor version: after `6.5.0` comes `6.6.0`. Patch
  releases are not scheduled.
- Only an open issue labelled `release-blocker` stops a promotion. That label marks
  something that worked in the current stable release but is broken in the
  candidate. Each such issue is closed once its fix is on `main`. When every one is
  closed, a fresh candidate is cut from `main` and starts its own 24-hour soak.
- Successive candidates for the same version are published at least **24 hours**
  apart. Time spent on a beta does not count towards an RC's soak.
- A defect that the current stable release already has is fixed on `main` and ships
  in the next release.

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
