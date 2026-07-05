# Pulse v6.0.3

_This changelog describes the stable `v6.0.3` patch release compared with
`v6.0.2`._

## Fixed

- Legacy v5 agent updates now recover complete connection state when saved v6
  state is missing or incomplete.
- Agent update commands now merge explicit Pulse URLs with token and scope
  details recovered from the running v5 agent process.
- Agent URL validation now treats `100.64.0.0/10` carrier-grade NAT and overlay
  addresses as local network targets for HTTP and WebSocket URLs.
- Docker, Helm, and installer release metadata now track the active stable
  patch version.

## Release Metadata

- Version: `v6.0.3`
- Rollback target: `v6.0.2`
- Promotion path: stable patch hotfix from `pulse/v6-release`
