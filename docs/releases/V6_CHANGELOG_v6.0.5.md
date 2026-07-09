# Pulse v6.0.5

_This changelog describes the stable `v6.0.5` patch release compared with
stable `v6.0.4`._

## Added

- MSP reports now support scheduled delivery with weekly or monthly cadence,
  explicit resource or tag scoping, and PDF or CSV output by email or to
  disk, and the MSP provider portal now shows a per-workspace alert rollup.
- Physical disk temperature thresholds are now configurable per disk type,
  with explicit per-disk overrides beating per-type defaults.
- OIDC provider settings now support editing requested scopes, and SSO
  provider restriction fields can be cleared once set.
- Kubernetes shared toolbar filters are now URL-backed so saved views keep
  exclusions, Docker container filters support `-term` search exclusions and
  persistent saved views, and saved views appear in the FilterBar mobile
  expanded body.

## Fixed

- Paid runtime activation now writes and reuses a durable installation fingerprint
  so repeated activation on the same Pro install reuses the existing
  installation slot.
- License grants now include the billing `current_period_end` alongside the
  short renewable grant lease, allowing client status to show the real paid
  period without weakening enforcement.
- Server installer execution now rejects unsafe piped invocation and keeps
  pinned `--version vX.Y.Z` update flows aligned.
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
- PBS backup discovery now derives its memory bounds from real snapshot data,
  fixing an rc.3 regression where groups beyond the bound were synthesized
  without verification, size, file, or per-snapshot time data.
- Physical disk SMART and Proxmox records now merge through canonical disk
  identity so NVMe, size, SMART, and temperature metadata survive enrichment.
- Proxmox install/update flows now preserve generated token state and expose
  smoke diagnostics for setup validation.
- Legacy OIDC SSO discovery now persists and advertises configured providers,
  with CSP nonce handling for embedded frontend assets.
- Legacy 3-segment OIDC SSO login and callback paths are served again for
  installs configured before the route consolidation.
- Pulse Mobile pairing now omits unsafe non-HTTPS Pulse web handoff URLs from
  QR/deep-link payloads while keeping relay pairing usable and returning an
  `instance_url_not_https` diagnostic for settings copy.
- OIDC and SAML browser sessions now expose a separate display label for app
  chrome while authorization remains bound to the stable provider-scoped
  principal.
- Proxmox host setup now prefers route-aware host URLs.
- Hosts now join the discovery fingerprint model so they auto-discover.
- Agent auth failures and staleness are now surfaced instead of a silent 401
  retry loop, with recovery guidance pointing at the Pulse UI.
- The Pro binary now blocks in-app self-update to prevent a silent downgrade
  to the community build.
- Registry rebuilds no longer emit no-op relationship-change records, keeping
  resource history growth bounded on busy deployments.
- Mock-mode churn is paced to homelab rates so demo deployments stop
  generating phantom change events.
- Docker Swarm workload dedupe now picks cluster-scoped winners
  deterministically.
- Relay pairing help now points at the Pulse Mobile install page.
- v5 migration recovery states are clarified for interrupted migrations.
- Docker, Helm, installer, and release-helper metadata now track the active
  stable patch version.

## Release Metadata

- Version: `v6.0.5`
- Rollback target: `v6.0.4`
- Promotion path: stable patch hotfix from `main`
