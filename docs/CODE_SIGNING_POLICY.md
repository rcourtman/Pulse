# Code Signing Policy

Pulse publishes release artifacts from the public
[`rcourtman/Pulse`](https://github.com/rcourtman/Pulse) repository. This policy
applies only to the open-source community artifacts built from that repository.
Private Pulse Pro, Relay, Enterprise, and service infrastructure are outside the
scope of the SignPath Foundation application and must not be submitted to the
community signing project.

## Signing service

Pulse was accepted into the SignPath Foundation open-source programme on
2026-08-06.

> Free code signing provided by [SignPath.io](https://signpath.io/), certificate
> by [SignPath Foundation](https://signpath.org/).

The SignPath organization `Pulse [OSS]` and project `Pulse` are
connected only to the public repository through SignPath's GitHub App and
trusted build system. The production release certificate is still awaiting
issuance (`CSR PENDING`), so the `release-signing` policy is not yet available
for production releases.

Until the production certificate is active and the exact-commit,
non-publishing proof run has passed, release notes must say when a Windows
artifact is not Authenticode-signed. Detached checksums and Pulse release
signatures remain mandatory and are not a substitute for Authenticode. The
`test-signing` policy may be used to validate the integration, but its test
certificate is untrusted and its output must never be published as a
production release. The manual
[`SignPath Test Signing Proof`](../.github/workflows/signpath-test-signing.yml)
workflow is the only test-signing entrypoint: it is restricted to `main`,
hard-codes the test policy, verifies the exact returned file set, and uploads
only a non-production JSON evidence record after verification. It never uploads
the test-signed binaries as a GitHub artifact or assembles a release candidate.

The canonical CI integration uses SignPath's GitHub trusted-build-system
action, split into two phases because production signing requests require
manual approval in the SignPath UI. The Windows build job uploads the three
unsigned agent executables as one immutable workflow artifact, submits it to
SignPath without waiting, and records the signing request id. A separate
collection job absorbs the approval latency: it waits for the recorded request
to complete, downloads the signed result by request id, and verifies every
file before candidate assembly. If approval outlasts the collection job's
polling window, that job fails with re-run guidance, and re-running it
collects the same recorded request — no rebuild, no second submission, no
second approval. A non-secret evidence artifact records the SignPath request
URL, source SHA, signer identity, and signed-file SHA-256 values.

The artifact configuration accepts exactly these ZIP-root files and no others:

- `pulse-agent-windows-amd64.exe`
- `pulse-agent-windows-arm64.exe`
- `pulse-agent-windows-386.exe`

The repository-secret PFX path is an explicitly selected break-glass fallback.
Normal stable publication and stable dry runs select `signpath` directly.

## Build and release controls

- Release artifacts are built by GitHub Actions from an exact commit on the
  `main` branch.
- The release workflow records artifact digests and promotes the same immutable
  candidate without rebuilding it.
- Only binaries built from the public repository's source and build scripts may
  be submitted to the SignPath Foundation project.
- Third-party or private binaries must never be signed with the community
  project certificate.
- Every production signing request requires approval by an authorised project
  approver.
- Test-signed output must never enter a release candidate or publication path.
- Production signing must fail closed while the release certificate or signing
  policy is invalid.
- Release checksums and detached signatures are published alongside artifacts
  and verified independently after publication.
- The exact-version OCI Helm chart is published only by the hosted
  `publish-helm-chart.yml` workflow. Its SHA-256 manifest digest and GitHub
  build-provenance attestation must bind to the release source commit before
  the digest enters `release-activation.json`. Activation recovery repeats
  that verification, and Helm Pages refuses to advertise a chart whose OCI
  tag, signer workflow, source commit, or digest has drifted from the immutable
  activation packet.
- Release activation requires GitHub CLI 2.97.0 or newer, which includes the
  literal signer-identity matcher fix. The shared
  `scripts/require-safe-gh-attestation.sh` guard enforces this floor. The
  published checksum manifest must carry build provenance from the exact
  `create-release.yml` workflow and release source commit; repository-level
  provenance is not sufficient.
- Every new release is assembled and validated as a draft. Its activation
  marker is uploaded and digest-checked before publication; GitHub must then
  report the published release as immutable, protecting its tag and complete
  asset set from replacement.
- Customer-facing image aliases, Helm indexes, paid-runtime pointers, and demo
  environments are not promoted until `gh release verify <tag> --repo
  rcourtman/Pulse` validates GitHub's signed release attestation and `gh release
  verify-asset <tag> <downloaded-asset> --repo rcourtman/Pulse` binds the
  downloaded activation marker to that attestation. The activation verifier
  also binds the release's downloaded `checksums.txt` to that immutable packet
  and verifies its exact workflow and source provenance. Operators can use the
  same commands to verify the packet and any downloaded release asset
  independently.

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
