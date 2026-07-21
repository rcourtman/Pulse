# SignPath Windows Authenticode Integration Packet - 2026-07-21

## Purpose

This packet covers the external step repository code cannot complete: approve
and configure Pulse's SignPath Foundation open-source project, then prove one
exact-SHA stable candidate without publishing it. It covers only public
community agent executables from `rcourtman/Pulse`, never private Pulse Pro,
Relay, Enterprise, or service binaries.

## SignPath Project Contract

1. Approve the existing Pulse Monitoring Ltd open-source application.
2. Authorize the SignPath GitHub App for `rcourtman/Pulse`.
3. Bind the project to `https://github.com/rcourtman/Pulse` through the GitHub
   trusted build system.
4. Require an authorised Pulse approver and governed `main` origin.
5. Configure a required `version` parameter and this exact ZIP-root artifact:

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
