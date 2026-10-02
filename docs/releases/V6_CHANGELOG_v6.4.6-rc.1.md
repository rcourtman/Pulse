# Pulse v6.4.6-rc.1

This changelog describes the changes since `v6.4.5`. Stable `v6.4.5` remains the rollback target.

## What's improved

- Keep PBS History on the corroborated host identity and discard stale retained host mappings.
- Follow quiet update streams with status checks and confirm server readiness before reload.
- Bound local health probes across IPv4 and IPv6.
- Update modernc.org/sqlite to v1.59.0 and modernc.org/libc to v1.75.7.
- Require affirmative consent before enabling automatic updates on a new installation.

## Release Metadata

- Version: `v6.4.6-rc.1`
- Previous stable: `v6.4.5`
- Rollback target: `v6.4.5`
- Rollback command: `sudo /bin/update --version v6.4.5`
- Promotion path: exact-SHA single-build release candidate from `release/v6.4`
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable. Windows Unified Agent binaries may display an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`. No governed mobile route, payload, Relay, pairing, approval, push or onboarding contract changed from `v6.4.5`, so no companion mobile build or store rollout is required
