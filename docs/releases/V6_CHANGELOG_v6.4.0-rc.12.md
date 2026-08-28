# Pulse v6.4.0-rc.12

This changelog describes the changes since `v6.4.0-rc.11`.

## Changed

- GitHub Atom fallback parsing now uses the feed structure directly, retains published or updated timestamps, and deterministically selects the current Linux architecture's release archive from a validated version tag.
- Release-note generation and visual-plan validation handle mixed existing notes and structured capture metadata more defensively.
- Missing update download URLs now produce an explicit error notification while leaving the confirmation dialog available for retry.
- Unraid disk-state normalization uses one canonical sentinel boundary across host-agent collection and server monitoring.

## Fixed

- GitHub API rate limiting no longer leaves in-app release checks with a new version but no archive URL, which previously made Start Update appear to do nothing.
- Feed fallback results no longer expose the zero-value release date when Atom includes a valid publication timestamp.
- Empty Unraid array slots no longer materialize as physical disks or contribute false degraded state when the sentinel is observed through either agent path.

## Release Metadata

- Version: `v6.4.0-rc.12`
- Previous candidate: `v6.4.0-rc.11`
- Previous published candidate: `v6.4.0-rc.11`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable. Windows may show an Unknown Publisher warning.
- Mobile decision: `existing-mobile-build-compatible`. Published iOS build 12 and Android versionCode 9 do not consume the server self-update Atom fallback or Unraid sentinel parsing paths, and no mobile-facing contract changes in this candidate require a companion upload.
