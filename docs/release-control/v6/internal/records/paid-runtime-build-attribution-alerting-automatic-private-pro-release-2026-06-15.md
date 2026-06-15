# Paid Runtime Automatic Private Pro Release Gate

Date: 2026-06-15
Owner: paid-runtime-build-attribution-alerting
Evidence tier: test-proof

## Trigger

A paid customer reported that the private Pulse Pro v6 download links stopped at
`6.0.0-rc.4` even though public v6 RCs had advanced past RC4.

## Finding

The live `pulse-license` private Pro release manifest still pointed at
`6.0.0-rc.4` with prefix `v6.0.0-rc.4-pro-20260507`. Public releases
`v6.0.0-rc.5` and `v6.0.0-rc.6` existed, but `rcourtman/pulse-enterprise`
had no later `Build Pro Release` workflow-dispatch run after the corrected RC4
customer-facing Pro publish on 2026-05-07.

The previous policy and checklist required a generated proof packet plus
`scripts/promote_paid_runtime_release_packet.sh`, but that path was still a
manual post-release operation. Public RC publication could therefore advance
without automatically building or promoting the matching private Pro runtime.

## Decision

For every non-draft v6 public release, the public release workflow owns the
private Pro runtime publication handoff:

1. After public asset validation succeeds, dispatch `rcourtman/pulse-enterprise`
   `Build Pro Release` against the exact public tag and version.
2. Require `upload_actions_artifact=false`, `upload_to_r2=true`, and
   `publish_docker_image=true`.
3. Derive the private R2 prefix from the public release workflow run.
4. Wait for the private Pro R2/Docker publication workflow to succeed.
5. Dispatch `rcourtman/pulse-pro` `Promote Paid Runtime Release` with the same
   version and R2 prefix.
6. Wait for the live paid-download broker promotion to succeed.

A failed private build or failed live promotion fails the public release
workflow. Private Pro RC/GA advancement must not depend on an operator noticing
a checklist item after the public RC has shipped.

## Implementation

- `repos/pulse/.github/workflows/create-release.yml` now has a
  `publish_private_pro_runtime` job gated on non-draft v6 releases after
  `validate_release_assets`.
- The job dispatches `rcourtman/pulse-enterprise` `Build Pro Release`, waits for
  completion, dispatches `rcourtman/pulse-pro` `Promote Paid Runtime Release`,
  and waits for completion without `continue-on-error`.
- `repos/pulse-pro/.github/workflows/promote-paid-runtime-release.yml` downloads
  the signed R2 proof packet, verifies its version, then runs
  `scripts/promote_paid_runtime_release_packet.sh --release-dir <proof-packet-dir> --execute-live`.
- Release policy, deployment-installability ownership docs, Pro operations docs,
  the Pro upgrade runbook, and the Pro launch checklist now describe the
  automatic path.
- The paid-runtime distribution validator now requires the promotion workflow
  and rejects non-blocking promotion drift.
- The legacy license email now repeats the private Pulse Pro runtime handoff for
  v6 paid features, including the Linux/Proxmox LXC archive guard.

## Proof

- `go test ./scripts/installtests -run 'TestCreateReleasePublishesPrivateProRuntime|TestInstallShSmokeWorkflowPresent|TestPublishHelmChartReachableViaWorkflowCall' -count=1`
- `python3 scripts/validate_paid_runtime_distribution.py`
- `python3 -m unittest scripts.tests.test_validate_paid_runtime_distribution`
- `go test . -run 'Test.*LicenseEmail|TestV6LicenseEmailIncludesPrivateDownloadPage' -count=1`
- YAML parse checks for `.github/workflows/create-release.yml` and
  `.github/workflows/promote-paid-runtime-release.yml`
- `git diff --check` in `repos/pulse` and `repos/pulse-pro`

## Residual

This record fixes the future release process and prevents another silent private
Pro runtime lag. It does not itself publish a new private Pro artifact for the
already-shipped public RC6 line; that is a separate live release operation.
