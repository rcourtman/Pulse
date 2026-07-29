# Pulse v6.2.0-rc.4

_This changelog describes the changes since `v6.2.0-rc.3`.
`v6.2.0-rc.4` remains a prerelease and rolls back to stable `v6.1.2`._

## Fixed

- Installer container-runtime discovery prefers a live rootful Docker daemon
  over any rootless socket, so a transient rootless Podman API socket no
  longer pins the agent service environment (#1647).
- `system_docker_runtime_is_active` gates rootless discovery with a
  `docker info` check that strips `DOCKER_HOST` and `CONTAINER_HOST`, falling
  back to probing `/var/run/docker.sock`, and the environment-application block
  no longer runs for explicit `--enable-docker` installs on such hosts.
- The Docker agent treats the podman runtime preference as an ordering hint
  rather than an identity override, so a podman-preferred connection that
  falls through to a docker endpoint reports docker and keeps Swarm collection
  enabled.
- The agent re-runs runtime discovery after three consecutive
  daemon-unavailable collects and swaps the connection behind a swappable
  client so concurrent goroutines keep a stable handle.
- Host CPU usage is averaged across the report interval from the cumulative
  CPU time counters instead of a blocking one-second spot sample, which
  overstated CPU on mostly-idle guests and delayed every collect by a second
  (#1648).
- The spot sample remains only as the fallback for the first collection and
  for counter resets such as a reboot or migration, and `CPUUsagePercent`
  keeps its meaning in the report schema.
- Proxmox storage constrained by a datacenter-level node restriction is
  omitted from nodes that cannot mount it instead of appearing Offline there
  (#1645).
- Executing actions with an interrupted or tombstoned correlated receipt
  settle as inconclusive, and accepted, started, or not-found receipts settle
  after the existing one-hour stuck threshold when no terminal result arrives
  (#1649).
- Administrators can force-fail legacy or otherwise unrecoverable executing
  actions through a capability-gated, approval-gated lifecycle route that
  never redispatches transport or overwrites terminal truth (#1649).
- Monitoring-only kiosk sessions remain confined after Escape, and truncated
  security-status payloads no longer create false security warnings or links
  into inaccessible settings (#1650).
- Terminal notification failures are persisted and reported through fixed,
  content-free diagnostic classes, and Alert Delivery Health surfaces the
  dominant class with targeted operator guidance.
- Repeated writes for one open alert occurrence coalesce immediately, while
  genuine refires and explicit severity transitions keep distinct history,
  preventing inflated alert and telemetry counts.

## Changed

- Build and Test runs frontend, sharded backend, script-smoke, and benchmark
  jobs in parallel, and a scoped local pre-push helper covers the most common
  governance, mutation-registry, Go, build, and frontend failures.

## Release Metadata

- Version: `v6.2.0-rc.4`
- Previous candidate: `v6.2.0-rc.3`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is part of this candidate
