# Pulse v6.0.5-rc.1

_This changelog describes the `v6.0.5-rc.1` release candidate compared with
stable `v6.0.4`._

## Fixed

- Patrol readiness checks now detect Gemini tool-call capability from Gemini
  candidate parts as well as top-level tool-call lists.
- Remembered-login state now persists the saved username when the checkbox is
  enabled during submit.
- Proxmox SMART temperature collection now retries direct SATA/SAT disks with an
  explicit SAT probe when smartctl auto-detection returns health but no
  temperature.
- Docker, Helm, installer, and release-helper metadata now track the active
  release candidate version.

## Release Metadata

- Version: `v6.0.5-rc.1`
- Rollback target: `v6.0.4`
- Promotion path: release candidate from `main`
