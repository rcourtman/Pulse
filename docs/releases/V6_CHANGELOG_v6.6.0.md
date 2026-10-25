# Pulse v6.6.0

This changelog describes stable `v6.6.0`, which promotes `v6.6.0-rc.6`. It records the changes since stable `v6.5.0`.

## Changes since stable v6.5.0

- Stable `v6.6.0` ships the `v6.6.0-rc.6` release candidate. That candidate's changelog, `docs/releases/V6_CHANGELOG_v6.6.0-rc.6.md`, and those of the earlier `v6.6.0` candidates record the changes since stable `v6.5.0`.
- The stable cut itself changes only release metadata: version pins, Helm chart metadata, release notes and upgrade pointers.

## Release Metadata

- Version: `v6.6.0`
- Promoted prerelease: `v6.6.0-rc.6`
- Previous stable: `v6.5.0`
- Rollback target: `v6.5.0`
- Rollback command: `sudo /bin/update --version v6.5.0`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: the standing SignPath-unavailable policy publishes unsigned Windows Unified Agent binaries with published checksums and detached signatures. The standing SignPath-unavailable policy applies until availability is explicitly restored. Windows Unified Agent binaries may display an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`. No governed mobile route, payload, Relay, pairing, approval, push or onboarding contract changed from `v6.5.0`, so no companion mobile build or store rollout is required
