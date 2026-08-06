# SignPath Windows Authenticode Integration Packet - 2026-07-21

## Purpose

This packet covers the external step repository code cannot complete: approve
and configure Pulse's SignPath Foundation open-source project, then prove one
exact-SHA stable candidate without publishing it. It covers only public
community agent executables from `rcourtman/Pulse`, never private Pulse Pro,
Relay, Enterprise, or service binaries.

## Current Status (2026-08-06)

The SignPath Foundation application has been approved. The following setup is
complete:

- the interactive organization invitation was accepted for `Pulse [OSS]`;
- project `Pulse` is connected to the public `rcourtman/Pulse` repository;
- the official SignPath GitHub App is installed only for that repository;
- the `CI builds` user is active, its notification address is confirmed, and
  its API token is stored as the repository secret `SIGNPATH_API_TOKEN`;
- all required `SIGNPATH_*` repository variables are configured;
- the GitHub trusted build system and governed `main` origin are active;
- `release-signing` requires one approval from the project owner; and
- artifact configuration `initial` requires `version` and permits only the
  three ZIP-root executables in the contract below.

Production signing is not ready. `Release certificate 2026` remains
`CSR PENDING`, which leaves the `release-signing` policy invalid until SignPath
Foundation's certificate authority issues and installs the certificate. The
remaining proof sequence is:

1. The isolated, non-publishing `test-signing` integration proof is complete.
2. Wait for the production release certificate to become active.
3. Run the exact-SHA, non-publishing `release-signing` proof described below.

Test-signed files use an untrusted certificate. Do not publish them, include
them in a production candidate, or mark production Windows signing ready on the
basis of a test request.

## Test-Signing Integration Proof (2026-08-06)

The canonical non-production integration proof is GitHub Actions run
[`31114440080`](https://github.com/rcourtman/Pulse/actions/runs/31114440080),
executed from `main` at
`0dd9c92cb84548c4f602f9e647150d2ce6de4f14` for version `6.2.0-rc.8`.
The manually dispatched workflow hard-coded `test-signing`, submitted only the
three contract executables through artifact configuration `initial`, verified
the exact returned file set and Authenticode signatures, and retained no signed
output. Repository policy retains the JSON proof artifact for seven days and
the unsigned input artifact for one day. The test-signed executables were
neither uploaded as a GitHub artifact nor published or assembled into a release
candidate. This record preserves the accepted evidence fields after the
ephemeral artifacts expire.

SignPath request
[`88badc79-817f-4a8d-80e9-5ccdbe6d69a1`](https://app.signpath.io/Web/1ecb3261-e389-49c2-a071-e03ff177cc5d/SigningRequests/88badc79-817f-4a8d-80e9-5ccdbe6d69a1)
used the self-signed test certificate
`CN=Test certificate for 'Pulse [OSS]'`, thumbprint
`D24B66B8F6C5F59362DE85C0965F1E4303ADE344`. The accepted signed-file hashes
are:

- `pulse-agent-windows-386.exe`:
  `b69ab0ed070e64e459edfe9150f2ad1b9a23bdb22b601bbf7ce480d2ed1dd6eb`
- `pulse-agent-windows-amd64.exe`:
  `75d08c411ccd3a98fa406674836e67e824a12a0dac7437fe1fa7ad552791ed6c`
- `pulse-agent-windows-arm64.exe`:
  `03ac32e86bc5456ccc23cfd85f3c7df2c23eafe84c74ac77f1e2fb0caa230d8d`

The four diagnostic runs `31110733308`, `31111803643`, `31112661094`, and
`31113777871` were cancelled or failed without publication while isolating the
Windows user-certificate-store hangs. The replacement uses bounded,
non-interactive `certutil` operations against the documented ephemeral machine
stores and bounds every SignTool verification to 90 seconds.

## SignPath Project Contract

The approved project must continue to satisfy this contract:

1. Keep the SignPath GitHub App restricted to `rcourtman/Pulse`.
2. Keep the project bound to `https://github.com/rcourtman/Pulse` through the
   GitHub trusted build system.
3. Require an authorised Pulse approver and governed `main` origin.
4. Keep the required `version` parameter and this exact ZIP-root artifact:

```xml
<artifact-configuration xmlns="http://signpath.io/artifact-configuration/v1">
  <parameters><parameter name="version" required="true" /></parameters>
  <zip-file>
    <pe-file path="pulse-agent-windows-amd64.exe"><authenticode-sign /></pe-file>
    <pe-file path="pulse-agent-windows-arm64.exe"><authenticode-sign /></pe-file>
    <pe-file path="pulse-agent-windows-386.exe"><authenticode-sign /></pe-file>
  </zip-file>
</artifact-configuration>
```

Preserve those three root filenames. Do not use a wildcard that could sign
unrelated executables.

## GitHub Repository Configuration

Add repository secret `SIGNPATH_API_TOKEN`, restricted to this project and
policy. Add these repository variables with the exact SignPath values:

- `SIGNPATH_ORGANIZATION_ID`
- `SIGNPATH_PROJECT_SLUG`
- `SIGNPATH_SIGNING_POLICY_SLUG`
- `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG`
- `SIGNPATH_EXPECTED_CERTIFICATE_SUBJECT`

Never store the token as a variable or commit a credential value. The workflow
reports every missing name before allocating the Windows runner.

## Non-Publishing Proof Run

Prepare the intended stable version on `main` through the governed packet
process, but do not dispatch `create-release.yml`. From the exact remote `main`
SHA, dispatch `Release Dry Run` with the stable version and all required
promotion/mobile metadata, then approve its SignPath request.

Accept the proof only when:

- SignPath origin identifies the expected repo, workflow, branch, and SHA;
- amd64, arm64, and 386 Authenticode verification passes;
- `windows-signing-evidence-<sha>-<version>` records that SHA, request URL,
signer identity, and three SHA-256 values;
- the candidate-manifest artifact contains `release-candidate.json` and
  `windows-signing-evidence.json`;
- release preflight, no-mutation stable-demo verification, and Definitive
  Dry-Run Verdict pass; and
- no tag, release, Docker tag, Helm chart, demo deployment, or private Pro
  promotion is created or changed.

Record the run URL, SignPath request URL, source SHA, version, and evidence
artifact before changing `single-build-release-promotion-path` to passed.

## Failure Handling

Missing configuration, rejection/timeouts, unexpected filenames, invalid
signatures, or signer mismatch remain failed stable proof. `legacy-pfx` is an
explicitly approved break-glass backend only; normal stable callers hard-code
`signpath`.
