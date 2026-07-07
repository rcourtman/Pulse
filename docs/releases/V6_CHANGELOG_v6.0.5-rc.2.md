# Pulse v6.0.5-rc.2

_This changelog describes the `v6.0.5-rc.2` release candidate compared with
stable `v6.0.4`._

## Fixed

- Patrol readiness checks now detect Gemini tool-call capability from Gemini
  candidate parts as well as top-level tool-call lists.
- Remembered-login state now persists the saved username when the checkbox is
  enabled during submit.
- Proxmox SMART temperature collection now retries direct SATA/SAT disks with an
  explicit SAT probe when smartctl auto-detection returns health but no
  temperature.
- Legacy agent update token recovery now handles v5-to-v6 update paths where a
  running agent can provide connection details even when stored state is absent.
- Temperature displays now resolve configured alert thresholds across node,
  workload, Docker, standalone agent, and drawer surfaces.
- PBS backup polling now keeps a bounded read-state working set during backup
  discovery.
- Physical disk SMART and Proxmox records now merge through canonical disk
  identity so NVMe, size, SMART, and temperature metadata survive enrichment.
- Proxmox install/update flows now preserve generated token state and expose
  smoke diagnostics for setup validation.
- Legacy OIDC SSO discovery now persists and advertises configured providers,
  with CSP nonce handling for embedded frontend assets.
- Proxmox host setup now prefers route-aware host URLs.
- Docker, Helm, installer, and release-helper metadata now track the active
  release candidate version.

## Release Metadata

- Version: `v6.0.5-rc.2`
- Rollback target: `v6.0.4`
- Promotion path: release candidate from `main`
