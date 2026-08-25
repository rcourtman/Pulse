# Pulse v6.4.0-rc.1

_This changelog describes the changes since `v6.3.1` included in
`v6.4.0-rc.1`._

## Added

- A scheduled weekly dependency-vulnerability scan covers the Go, frontend,
  integration, and GitHub Actions dependency surfaces.
- Shared platform windowing, object-drawer headers, attention sections,
  technical-detail disclosures, sortable indicators, and touch-capability
  helpers establish reusable infrastructure presentation contracts.

## Changed

- Large-estate workloads, infrastructure, storage, and platform lists render,
  navigate, search, and receive live updates incrementally.
- Proxmox node search shares the workload visibility predicate, backup views
  use canonical routes, and recovery pagination follows the normalized limit.
- Docker, Kubernetes, Proxmox, TrueNAS, VMware, standalone agent,
  availability, alert, and storage tables share consistent density, disclosure,
  drawer, and narrow-viewport behavior.
- Mobile and touch layouts retain native page scrolling and gestures while
  hover-only tooltips stay disabled on touch interactions.
- Cold start preloads only the entry module's static import graph while the
  import-map integrity block continues to cover every built JavaScript asset.
- Public and private release payload compilation runs once on isolated,
  credential-free trusted workers and crosses an immutable artifact-identity
  boundary before hosted signing or publication.

## Fixed

- Fixed poll intervals remain fixed when adaptive scheduling is disabled.
- Agent re-enrollment clears host-removal blocks from every owning store.
- Connection alerts respect the configured offline-alert policy.
- Reinstall and hosted enrollment preserve agent command-policy intent.
- Discovery lookups resolve known equivalent forked host identities.
- Windowed tables no longer leave blank regions, lose expanded-row scroll
  ownership, or detach touch gestures from the page.
- The notifications API no longer returns the stored Apprise API key.

## Release Metadata

- Version: `v6.4.0-rc.1`
- Previous stable: `v6.3.1`
- Rollback target: `v6.3.1`
- Rollback command: `./scripts/install.sh --version v6.3.1`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.4.0` restores mandatory SignPath signing unless a
  new version-bound owner decision is recorded
- Mobile decision: `no-mobile-impact`; changes since `v6.3.1` preserve the
  existing mobile, Relay, onboarding, and mobile-facing API contracts, so no
  companion upload or public store rollout is required
