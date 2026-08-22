# Pulse v6.3.0 Release Notes

`v6.3.0` is a stable minor release for the Pulse v6 line. It follows stable
`v6.2.1` and promotes the monitoring-first operations work exercised across
the `v6.3.0-rc.1` through `v6.3.0-rc.6` line, plus the bounded fixes included
in the final stable cutoff.

## Highlights

- Patrol adds durable objectives and verified work receipts; Actions provides
  a dedicated inbox for governed approvals and execution.
- Estate-first pages add canonical search, status facets, relationships,
  timelines, and Operational Trust signals across mixed infrastructure.
- Safer Unified Agent operation, clearer delivery evidence, and a fail-closed
  release pipeline strengthen monitoring and controlled action.

## Added

- Durable scoped Patrol objectives, validated read-only observer missions, and
  content-free telemetry for operational outcomes.
- A first-class Actions workspace for approval requests, governed plans,
  execution records, audit evidence, and verification state.
- Typed Unified Agent action preflight for supported host and Docker
  operations, with stable refusal codes for stale plans, changed targets,
  missing prerequisites, policy decisions, and unavailable capabilities.
- Canonical estate summaries, status facets, shared platform search, resource
  relationship views, and resource-change timelines.
- A seven-day activity log for real notification delivery attempts and a
  supported least-privilege Unified Agent installation profile.

## Improved

- Patrol surfaces provider-unavailable state directly and clears stale blocked
  findings when a configured provider becomes available again.
- Resource presentation keeps same-short-name hosts from separate estates
  distinct instead of collapsing them into one row.
- Per-resource severity overrides can re-enable an offline alert even when the
  corresponding global threshold is disabled.
- Subscription-backed turns bound canceled command cleanup so descendant-held
  output pipes cannot extend the caller-owned idle timeout.
- Docker-in-LXC discovery backs off against slow or failing Proxmox hosts, and
  Unified Agent observers remain report-only with destination-scoped trust.
- Release compilation, backend admission, container qualification, public and
  private staging, and post-publication convergence make fuller use of the
  dedicated PVE capacity without moving customer pointers before exact-SHA
  readiness.

## Fixed

- Release dry-run diagnostics now fail closed when the diagnostic runner or
  stable-tier test path fails, while preserving actionable artifacts.
- Native-agent fixtures are path-portable on Windows, and pre-commit Go linting
  follows the repository toolchain declared by `go.mod`.
- Release qualification retains the resource controls required by the release
  asset builder and rejects insufficient measured worker headroom before
  backend shard execution starts.
- Activation recovery, Helm convergence, private-license checks, and paid
  runtime proof continue to join the canonical release result rather than
  relying on duplicate or moving state.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 26 release
  gates passed, with `release_ready=True` at the stable cutoff.
- Production telemetry on 2026-08-22 showed 18 active `v6.3.0-rc.6` installs
  across binary and Docker, 56 recorded update successes, zero update
  failures, zero rollback or version-departure signals, and no new notification
  or governed-action failure counters on follow-up heartbeats.
- The preceding `v6.3.0-rc.5` cohort likewise showed no rollback signal; one
  install advanced to `v6.3.0-rc.6`, and its single rolling-window update
  failure was already present on the first heartbeat rather than increasing on
  the candidate.
- The release owner explicitly accepted the shortened `rc.6` soak and the
  bounded post-RC cutoff for `v6.3.0`. This is version-bound risk acceptance,
  not 72-hour soak evidence.
- The exact pushed stable SHA must pass the no-publication Release Dry Run
  before the same SHA enters the single-build publication workflow.
- Windows Unified Agent binaries are not Authenticode-signed for `v6.3.0` and
  may display an Unknown Publisher warning. The release owner approved this
  version-bound exception because Windows signing is not yet available.
- The unsigned-Windows exception changes only Authenticode. Exact-SHA builds,
  SHA-256 checksums, detached signatures, the immutable candidate manifest,
  and published-digest verification remain required.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.3.0`. Existing configurations
remain valid and no manual data migration is required.

The rollback target is `v6.2.1`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.2.1
```

This server release is compatible with the existing Pulse Mobile candidate.
The changes since `v6.3.0-rc.6` preserve the checked-in mobile API, Relay,
pairing, approval, push, authentication, and onboarding contracts. No companion
upload or public mobile-store rollout is part of this server release.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
Unproved self-service commercial plan or billing-cadence transitions remain
disabled and are not introduced by this release.
