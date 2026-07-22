# Pulse v6.1.0

_This changelog describes stable `v6.1.0` compared with stable `v6.0.5`._

## Added

- Typed Pulse Intelligence detection, investigation, proposal, review,
  execution, receipt, and verification contracts.
- A dedicated Actions inbox and canonical Operational Trust attention
  workbench for protection, availability, alert, and governed-action work.
- Governed Docker and Podman restart, Docker update, Proxmox guest lifecycle,
  host update, Debian and Ubuntu package maintenance, and storage-pressure
  cleanup paths.
- Report-only Unified Agent observer destinations with destination-scoped
  identity, token, retry, health, command-authority, and transport policy.
- Model-led Patrol qualification and subscription-backed Claude transport with
  bounded native-tool execution.
- Routed Agent Doctor workflow, `--report-ip` host identity, SAS transport, and
  SCSI SMART attribute support.
- Exact-SHA, checksum, detached-signature, manifest, and published-digest
  verification for Windows Unified Agent artifacts.

## Changed

- Platform and connected-system pages now emphasize monitor-first attention and
  direct operator tasks.
- Patrol, Assistant, Actions, alerting, and monitored resources share canonical
  attention and action lifecycle state.
- Physical addresses lead virtual bridges in host identity, ESXi hosts group
  under vCenter, and integration-observed machines stay distinct from real
  Unified Agents.
- Metrics reads proceed concurrently with writes, and retained audit data is
  reclaimed incrementally.
- Post-update What's New content uses the shared dialog and keeps the existing
  release-gating and dismissal contract.

## Fixed

- Agent install tokens now carry requested command permissions and support
  first-registration command-channel binding.
- Native updates self-test replacements, reject edition downgrades, preserve
  rollback, and keep manual update commands visible until an update applies.
- TrueNAS response handling, ZFS member identity, SAS/SCSI disk health, and
  Proxmox health vocabulary now reflect authoritative storage data.
- Patrol route scope, slow-provider status, finding reconciliation, proposal
  evidence, notification delivery, and action-dispatch recovery are consistent.
- Relay-mobile tokens can access the Patrol attention routes required by the
  existing Pulse Mobile candidate, with compatibility enforced by the
  canonical core/mobile contract.
- In-app update selection, new-version reload, release-note parsing, Docker
  recovery, OIDC/session continuity, and installer recovery include the fixes
  accumulated during the release-candidate line.
- DOM sanitization includes the fix for `GHSA-c2j3-45gr-mqc4`.

## Release Metadata

- Version: `v6.1.0`
- Previous stable: `v6.0.5`
- Promoted prerelease lineage: `v6.1.0-rc.4`
- Rollback target: `v6.0.5`
- Rollback command: `./scripts/install.sh --version v6.0.5`
- Promotion path: owner-approved exact-SHA stable cutoff from `main`, using the
  one-version soak exception and the single-build release-candidate workflow
- Windows signing decision: one-release owner exception; the `v6.1.0` Windows
  Unified Agent binaries are not Authenticode-signed and may show an Unknown
  Publisher warning, while checksums and detached `.sig`/`.sshsig` signatures
  remain required. Later stable releases restore the Authenticode requirement.
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile `1.0.0`
  iOS build `11` and Android versionCode `9` require no companion upload, and
  no public store rollout is part of this server release
