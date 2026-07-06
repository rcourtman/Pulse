# Pulse v6.0.4

_This changelog describes the stable `v6.0.4` patch release compared with
`v6.0.3`._

## Fixed

- Audit Log now tolerates legacy SQLite timestamp rows that include Go
  monotonic clock suffixes.
- Remembered-login state now persists the saved username when the checkbox is
  enabled.
- Legacy agent update recovery now handles additional recovered state and local
  HTTP URL cases.
- Platform CPU metrics, Proxmox live charts, Docker app-container percentages,
  and unified-resource metric adapters now use the same canonical percent
  contract.
- PBS and recovery backup rows now retain memory usage details through
  read-state conversion.
- Docker container metadata now keeps canonical container URL identity, and
  empty Docker facet tabs no longer enter the UI admission path incorrectly.
- OIDC provider detail edits now persist provider metadata.
- Docker, Helm, and installer release metadata now track the active stable
  patch version.

## Release Metadata

- Version: `v6.0.4`
- Rollback target: `v6.0.3`
- Promotion path: stable patch hotfix from `main`
