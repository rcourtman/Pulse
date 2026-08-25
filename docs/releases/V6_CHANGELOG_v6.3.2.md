# Pulse v6.3.2

_This changelog describes stable `v6.3.2` compared with stable `v6.3.1`._

## Fixed

- Metrics-history retention now releases oversized backing arrays after dense
  seed data ages out, preventing sustained memory growth that can wedge Pulse
  under container memory limits
  ([fix](https://github.com/rcourtman/Pulse/commit/f18f15bf7b1fbdf1078bf412754c059694c8b996)).
- Platform connection alerts now honor disabled per-resource and global
  offline policies, expected-offline intent, and offline quiet-hours routing
  ([#1721](https://github.com/rcourtman/Pulse/issues/1721)).
- Pulse's own `rcourtman/pulse` container image now uses the product update
  service instead of displaying a false container-update badge
  ([#1774](https://github.com/rcourtman/Pulse/issues/1774)).
- Availability targets retain their configured fixed polling interval when
  adaptive scheduling is disabled, instead of polling on every main monitor
  tick ([#1745](https://github.com/rcourtman/Pulse/issues/1745)).

## Release Metadata

- Version: `v6.3.2`
- Previous stable: `v6.3.1`
- Rollback target: `v6.3.1`
- Rollback command: `./scripts/install.sh --version v6.3.1`
- Promotion path: emergency stable patch from `main`, using the single-build
  release workflow after exact-SHA qualification
- Emergency reason: metrics-history memory growth can wedge Pulse under
  container memory limits, while disabled offline policies can still produce
  alert noise on stable
- Windows signing decision: Authenticode signing is required for `v6.3.2`; no
  unsigned Windows owner exception is authorized for this version
- Mobile decision: `no-mobile-impact`; no governed mobile-facing path changed
  from `v6.3.1`, so no companion build or public store rollout is required
