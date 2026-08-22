# Pulse v6.3.0

_This changelog describes stable `v6.3.0` compared with stable `v6.2.1`._

## Added

- Durable scoped Patrol objectives, read-only observer missions, verified work
  receipts, and a dedicated Actions workspace for approvals and governed work.
- Typed Unified Agent action preflight with stable refusal classifications for
  target changes, prerequisites, policy, capability, contract, and stale-plan
  boundaries.
- Estate summaries, status facets, canonical platform search, resource
  relationships, change timelines, and real notification-delivery history.
- A supported least-privilege Unified Agent profile and destination-scoped
  report-only observer topology.

## Changed

- Patrol provider-unavailable state is explicit and stale blocked findings
  recover when a configured provider returns.
- Platform and resource presentation preserve canonical identity across
  separate estates even when hosts share the same short name.
- Per-resource severity overrides can deliberately re-enable disabled offline
  alerts without changing the global threshold.
- Subscription-backed AI turns, Docker-in-LXC discovery, large-estate reads,
  release compilation, backend admission, container staging, and paid-runtime
  convergence use bounded work and measured capacity.
- Release publication keeps exact-source, signing, immutable-manifest,
  installer, public/private artifact, Helm, activation, and convergence proof
  joined at one release commit.

## Fixed

- Release dry runs fail closed on diagnostic runner and stable-tier failures
  and retain the corresponding diagnostics.
- Native-agent fixtures remain path-portable on Windows, and pre-commit linting
  uses the Go toolchain selected by `go.mod`.
- Release qualification preserves asset-builder resource controls and refuses
  a backend shard plan that lacks measured worker headroom.
- Activation recovery, Helm repository operations, private-license checks, and
  paid-runtime verification use canonical exact-release state.

## Release Metadata

- Version: `v6.3.0`
- Previous stable: `v6.2.1`
- Promoted prerelease lineage: `v6.3.0-rc.6`
- Content cutoff base: `53ba9786c5522a6839f9cbd3d01c02402556f9eb`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: owner-approved exact-SHA stable cutoff from `main`, using the
  v6.3.0-only soak waiver and the single-build release workflow
- Soak decision: production telemetry showed no new update failures, rollback
  signals, notification-failure increases, or governed-action-failure
  increases in the `rc.5` and `rc.6` cohorts; the release owner accepted the
  shortened soak and bounded post-RC cutoff as version-bound risk acceptance
- Windows signing decision: version-bound unsigned-Windows exception because
  signing is not yet available; the binaries are not Authenticode-signed and
  may display an Unknown Publisher warning, while exact-SHA, checksum,
  detached-signature, immutable-manifest, and published-digest verification
  remain mandatory
- Mobile decision: `existing-mobile-build-compatible`; the changes since
  `v6.3.0-rc.6` preserve the checked-in mobile, Relay, onboarding, and
  mobile-facing API contracts, and no companion upload or public store rollout
  is required
