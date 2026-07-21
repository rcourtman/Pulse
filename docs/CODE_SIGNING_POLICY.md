# Code Signing Policy

Pulse publishes release artifacts from the public
[`rcourtman/Pulse`](https://github.com/rcourtman/Pulse) repository. This policy
applies only to the open-source community artifacts built from that repository.
Private Pulse Pro, Relay, Enterprise, and service infrastructure are outside the
scope of the SignPath Foundation application and must not be submitted to the
community signing project.

## Signing service

Pulse is applying to the SignPath Foundation open-source programme. Once the
application is approved, Windows community release artifacts will use free code
signing provided by [SignPath.io](https://signpath.io/), with the certificate
issued by the [SignPath Foundation](https://signpath.org/).

Until approval and production integration are complete, release notes must say
when a Windows artifact is not Authenticode-signed. Detached checksums and Pulse
release signatures remain mandatory and are not a substitute for Authenticode.

The canonical CI integration uses SignPath's GitHub trusted-build-system
action. GitHub Actions uploads the three unsigned Windows agent executables as
one immutable workflow artifact, submits it to SignPath, waits for approval and
completion, downloads the signed result, and verifies every file before
candidate assembly. A non-secret evidence artifact records the SignPath request
URL, source SHA, signer identity, and signed-file SHA-256 values.

The repository-secret PFX path is an explicitly selected break-glass fallback.
Normal stable publication and stable dry runs select `signpath` directly.

## Build and release controls

- Release artifacts are built by GitHub Actions from an exact commit on the
  protected `main` branch.
- The release workflow records artifact digests and promotes the same immutable
  candidate without rebuilding it.
- Only binaries built from the public repository's source and build scripts may
  be submitted to the SignPath Foundation project.
- Third-party or private binaries must never be signed with the community
  project certificate.
- Every signing request requires approval by an authorised project approver.
- Release checksums and detached signatures are published alongside artifacts
  and verified independently after publication.

## Project roles

- **Committers and reviewers:** repository collaborators listed by GitHub for
  [`rcourtman/Pulse`](https://github.com/rcourtman/Pulse).
- **Approvers:** the repository owner,
  [`rcourtman`](https://github.com/rcourtman), and any future maintainer granted
  the SignPath Approver role by the repository owner.

All project members with repository or signing access must use multi-factor
authentication. Signing access must be removed promptly when a maintainer no
longer needs it.

## User privacy and system changes

Pulse's data handling and opt-out controls are documented in the
[Privacy Policy](PRIVACY.md). Installer behavior, service creation, privileges,
and uninstallation are documented in the [Installation Guide](INSTALL.md) and
[Agent Security](AGENT_SECURITY.md).

Security concerns involving a signed artifact should be reported using the
private process in the repository's [Security Policy](../SECURITY.md).
