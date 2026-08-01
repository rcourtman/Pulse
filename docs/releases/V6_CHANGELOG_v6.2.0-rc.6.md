# Pulse v6.2.0-rc.6

_This changelog describes the changes since `v6.2.0-rc.5`.
`v6.2.0-rc.6` remains a prerelease and rolls back to stable `v6.1.2`._

## Fixed

- Restored PVE workloads on the aggregate Proxmox page.
- Prevented the Alert thresholds page from crashing on guest filesystems with
  omitted usage values and aligned shared disk presentation with the optional
  API fields.
- Repaired half-removed installations across Unix and Windows update paths.
- Preserved Windows custom-CA and Skip TLS verification behavior through the
  generated PowerShell 5.1 installer command.
- Unified direct PBS API connections with matching agent reports without
  introducing ambiguous or VMware identity merges.
- Removed false Agent Doctor `Needs attention` states and fictitious deployed
  `v0` labels when only legacy profile-deployment history is absent.

## Release Guardrails

- All release cuts now gate publication on frontend unit tests, TypeScript
  checking, and render smoke for the primary Proxmox, Docker, Kubernetes, and
  Alert threshold surfaces.
- Release qualification executes the generated Windows TLS commands against
  self-signed and custom-CA endpoints under Windows PowerShell 5.1.
- Regression coverage now locks aggregate platform filtering, omitted disk
  fields, installer repair, connected-system identity, and profile diagnostic
  state at their owning contracts.

## Release Metadata

- Version: `v6.2.0-rc.6`
- Previous candidate: `v6.2.0-rc.5`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `existing-mobile-build-compatible`; RC6 has no new mobile
  impact, and Pulse Mobile 1.0.0 iOS build 11 and Android versionCode 9, both on
  runtime version 2, remain distributed to the existing beta cohort. No public
  store rollout is part of this candidate
