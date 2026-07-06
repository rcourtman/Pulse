# Pulse v6.0.5

_This changelog describes the stable `v6.0.5` patch release compared with
`v6.0.4`._

## Fixed

- Patrol readiness checks now detect Gemini tool-call capability from Gemini
  candidate parts as well as top-level tool-call lists.
- Remembered-login state now persists the saved username when the checkbox is
  enabled during submit.
- Docker, Helm, installer, and release-helper metadata now track the active
  stable patch version.

## Release Metadata

- Version: `v6.0.5`
- Rollback target: `v6.0.4`
- Promotion path: stable patch hotfix from `main`
