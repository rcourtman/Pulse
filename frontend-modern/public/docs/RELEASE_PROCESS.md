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

### Daily betas

Between stable releases, a beta is cut from `main` every **24 hours** when
there are changes since the previous beta. Each carries everything on `main`
at its cut, not just the fixes named in an issue reply. No beta is cut within
a day of the release candidate being due. A beta has no soak and is never
promoted to stable; a failed beta does not hold the stable release.

You do not need to install every beta. A fix merged after a beta was cut is
not in that beta, even if it is now on `main`. Use the containing version
named in the issue reply and its published release notes, rather than assuming
that the newest preview fixes your problem. Stable users receive the change
only when a stable release containing it ships.

## Software releases and Helm charts

The releases list contains both Pulse software and Helm charts. Software tags
use `vX.Y.Z` for stable releases, with a prerelease suffix for previews. Tags
starting with `helm-chart-` contain chart packaging for Kubernetes; they are not
Pulse server archives and cannot update a systemd or Proxmox LXC server.

List order does not establish the latest software release: a chart can be
published after its corresponding server release. Choose the published software
tag and its server assets for a server update, or use Pulse's **Settings → System
→ Updates** selector. If a third-party helper chooses a chart tag, do not work
around the failure by disabling signature verification or replacing the server
with a chart asset. Follow the existing installation's
[signed update route and helper ownership checks](AUTO_UPDATE.md#proxmoxve-lxc-manual).

## How releases are prepared

Releases follow a fixed schedule rather than individual judgment.

1. **Cut a candidate from `main`.** Stable publication is targeted **seven days
   after the last stable release**, provided `main` has changed since that release.
   The next release candidate becomes eligible **one day earlier**, reserving the
   full 24-hour soak. Preparation and qualification can move publication later.
   An important fix that should not wait for the schedule can bring that cut
   forward, or replace a candidate that is still soaking with a fresh one
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

An important fix can bring the next candidate cut forward. It still carries
everything on `main` at the cut and goes through the same required checks and
24-hour soak; it is not a selected patch release or a testing exemption.

## Taking part in Preview testing

Start with the [published release notes](https://github.com/rcourtman/Pulse/releases).
Keep a consistent [full-state backup](MIGRATION.md#full-state-recovery) and the
documented rollback version available before an update. Follow the
[update preparation and recovery limits](DEPLOYMENT_MODELS.md#updates-by-model)
for your existing deployment; a backup of Pulse is not a backup of its monitored
workloads. You do not need to install every preview.

Retest a relevant correction through the next ordinary, safe use of the
affected view or workflow. Do not repeat a guest freeze, backup failure,
outage, uncertain update or queued notification just to confirm a fix. A
successful Test or a source test is not confirmation of the original job;
unavailable observations remain unknown. Preserve the existing state and
evidence when recovery is uncertain.

When [reporting a problem](https://github.com/rcourtman/Pulse/issues/new/choose),
include the exact version already observed, deployment method, expected and
observed behaviour, and original sequence; unknown details are valid. Use
[locally reviewed, redacted evidence](TROUBLESHOOTING.md#collect-diagnostics-safely),
not a new diagnostic run merely to fill a report. Confirmation of ordinary
recovery is useful too; if the problem persists, reply on the original issue
rather than filing it again.
