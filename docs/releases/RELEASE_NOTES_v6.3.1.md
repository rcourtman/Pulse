# Pulse v6.3.1 Release Notes

`v6.3.1` is a stable patch release for the Pulse v6 line. It follows stable
`v6.3.0` and contains focused corrections for alert delivery, Unified Agent
control, local subscription providers, and Docker monitoring overhead.

## Highlights

- Recover or dismiss terminal notification failures without losing delivery
  history, while disabled PBS offline alerts remain silent.
- Docker commands recover safely after token rotation, and Synology hosts avoid
  repeated full-daemon storage scans on every report.
- Local subscription providers now resolve and diagnose their CLIs as the
  actual Pulse service account.

## Improved

- Notification delivery history now exposes confirmed retry and dismiss
  operations for terminal failures. Retried items receive a fresh bounded
  attempt budget while their prior audit history remains available.
- Refused governed actions record the resource, capability, stable refusal
  code, and the specific Docker command-agent lookup that missed.
- Standard systemd installs give the Pulse service account a private CLI home
  and a deterministic executable search path. Explicit provider CLI path
  overrides remain available for non-standard layouts.
- Docker host storage totals are refreshed on a bounded 15-minute cadence and
  retain the last good aggregate when a refresh fails. Live host and container
  metrics continue on the configured reporting interval.

## Fixed

- Disabled PBS offline alerts no longer enter the notification dispatch path;
  enabled alerts and recovery notifications retain their existing lifecycle.
- Docker start, stop, restart, remove, and update commands recover after an
  agent reporting-token rotation by proving the exact tenant, agent ID, and
  canonical hostname rather than weakening identity matching.
- Docker update preflight once again routes through the Unified Agent, and
  terminal digest-drift refusals no longer strand later update attempts.
- Local subscription setup failures now distinguish a CLI that is missing or
  not logged in for the Pulse service account from provider network
  reachability failures.
- Synology DSM Docker monitoring no longer launches a verbose daemon-wide disk
  usage inventory every 30 seconds or immediately retries a slow failed scan.

## Release Qualification

- The release uses the emergency stable-patch path because the fixes address
  active customer harm across alert delivery, infrastructure control, local AI
  setup, and Docker host load without introducing a same-version RC.
- The exact pushed release SHA must pass the no-publication Release Dry Run and
  its integrated exact-SHA candidate checks before that same SHA is submitted
  to the single-build publication workflow.
- The release owner approved a `v6.3.1`-only exception because SignPath's
  production certificate remains CSR pending. Windows Unified Agent binaries
  are not Authenticode-signed and may display an Unknown Publisher warning;
  exact-SHA checksums, detached signatures, immutable-manifest verification,
  and published-digest verification remain mandatory.
- No mobile-facing path changed between `v6.3.0` and this release, so the mobile
  decision is `no-mobile-impact`; no companion build or store rollout is
  required.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.3.1`. Existing configurations
remain valid and no manual data migration is required.

The rollback target is `v6.3.0`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.3.0
```

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
