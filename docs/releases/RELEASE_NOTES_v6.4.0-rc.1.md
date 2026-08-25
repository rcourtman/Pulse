# Pulse v6.4.0-rc.1 Release Notes

`v6.4.0-rc.1` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.3.1` and is the first candidate on the `v6.4.0` line. It
focuses on responsive large-estate operation, consistent infrastructure
tables and detail drawers, monitoring correctness, and a more isolated release
build path.

## Highlights

- Large estates render, scroll, search, and update incrementally across workloads, infrastructure, storage, and platform pages.
- Infrastructure tables and drawers share consistent desktop, mobile, touch, sorting, density, and navigation behavior.
- Monitoring and agent re-enrollment fixes preserve configured polling, offline policy, command intent, and resource visibility.

## Added

- A weekly scheduled dependency-vulnerability scan now covers the Go, frontend,
  integration, and GitHub Actions dependency surfaces.
- Shared windowed platform-list primitives extend bounded rendering to Docker,
  Kubernetes, Proxmox, TrueNAS, VMware, standalone agent, availability, alert,
  and storage views.
- Canonical object-drawer headers, attention sections, technical-detail
  disclosures, sortable-table indicators, and touch-capability helpers provide
  one reusable presentation contract across infrastructure surfaces.

## Improved

- Workload, Proxmox, infrastructure, and storage views use incremental
  navigation, virtualized rows, stable scroll ownership, and adaptive preview
  thresholds to keep 50-node estates responsive.
- Platform search and row visibility share the same predicates, including
  Proxmox nodes and their visible guests. Navigation tabs remain stable across
  WebSocket updates instead of remounting with live state.
- Route-level code splitting remains effective on cold start: the frontend no
  longer modulepreloads every lazy route chunk, while integrity coverage stays
  enforced for dynamically loaded assets.
- Proxmox backup views use canonical routes, compact healthy-state shields, and
  recovery pagination derived from the normalized query limit.
- Mobile and touch layouts keep native page scrolling and gestures, avoid hover
  tooltips, use consistent disclosure affordances, and retain reachable table
  actions at narrow widths.
- Public and private release payloads compile once on isolated,
  credential-free trusted workers, then cross an immutable artifact-identity
  boundary before hosted signing, qualification, and publication.

## Fixed

- Fixed polling intervals are honored when adaptive scheduling is disabled.
- Re-enrollment clears host-removal blocks from every owning store so a valid
  returning agent is not held in a partially removed state.
- Connection alerts can no longer bypass the configured offline-alert policy.
- Agent reinstall and hosted enrollment preserve command-policy intent instead
  of silently dropping the command-execution posture.
- Discovery resolves known equivalent forked host identities when matching
  resources, preventing an identity spelling difference from hiding current
  discovery results.
- Large workload and platform tables no longer leave blank virtualized regions,
  lose expanded-row scrolling, or move touch gestures away from the page.
- Notification configuration no longer returns the stored Apprise API key to
  the browser after it has been saved.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 26 release
  gates passed at the candidate cutoff.
- The single-build release workflow must pass its self-contained frontend,
  backend, mobile-decision, immutable-candidate, container, Helm, installer,
  public/private staging, and activation checks before publication.
- The release decision is `no-mobile-impact`: no Pulse Mobile API, Relay,
  pairing, push, authentication, approval, or onboarding contract changed from
  `v6.3.1`, and no companion upload or public mobile-store rollout is part of
  this candidate.
- The changes since `v6.3.1` do not require a Pulse Mobile client change and
  preserve the existing mobile, Relay, onboarding, and mobile-facing API
  contracts.
- Windows Unified Agent binaries in this prerelease retain exact-SHA, checksum,
  and detached-signature verification but are not Authenticode-signed. Stable
  `v6.4.0` still requires the normal SignPath Authenticode lane unless a new
  explicit version-bound owner decision is recorded.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.4.0-rc.1` only when you are
comfortable testing an RC. Existing configurations remain valid and no manual
data migration is required.

The rollback target is `v6.3.1`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.3.1
```

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
