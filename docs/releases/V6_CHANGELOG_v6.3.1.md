# Pulse v6.3.1

_This changelog describes stable `v6.3.1` compared with stable `v6.3.0`._

## Changed

- Terminal notification failures can be retried with a fresh bounded budget or
  dismissed while retaining their delivery audit history.
- Governed-action refusals identify the resource, capability, refusal code, and
  missed Docker command-agent lookup in the server journal.
- Systemd installs provide a service-account CLI home and deterministic search
  path for local subscription providers, with explicit path overrides for
  custom installations.
- Full Docker daemon storage inventory is cached for 15 minutes, uses a single
  refresh attempt, and retains the last successful aggregate on failure.

## Fixed

- Disabled PBS offline alerts no longer dispatch notifications.
- Docker lifecycle and update operations recover safely after reporting-token
  rotation through exact tenant, agent-ID, and canonical-hostname admission.
- Docker update preflight again uses the Unified Agent, and terminal
  digest-drift receipts no longer block a later attempt.
- Local subscription setup reports missing executable and service-account login
  state instead of misclassifying those failures as provider reachability.
- Synology DSM hosts avoid repeated verbose daemon-wide disk-usage scans on the
  normal live-report cadence.

## Release Metadata

- Version: `v6.3.1`
- Previous stable: `v6.3.0`
- Rollback target: `v6.3.0`
- Rollback command: `./scripts/install.sh --version v6.3.0`
- Promotion path: emergency stable patch from `main`, using the single-build
  release workflow after an exact-SHA no-publication dry run
- Emergency reason: active customer harm across notification recovery, Docker
  control, local subscription setup, and Synology Docker host load
- Windows signing decision: version-bound `v6.3.1` owner exception because the
  SignPath production certificate remains CSR pending; Windows binaries are
  not Authenticode-signed and may display an Unknown Publisher warning, while
  the exact-SHA integrity controls remain mandatory
- Mobile decision: `no-mobile-impact`; no governed mobile-facing path changed
  from `v6.3.0`, so no companion build or public store rollout is required
