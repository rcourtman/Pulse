# Pulse v6.2.0-rc.4 Release Notes

`v6.2.0-rc.4` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.3`. This hardening candidate
repairs actions stranded after an agent interruption, tightens kiosk-session
containment, improves alert and notification diagnostics, and carries the
latest storage, container-runtime, and host-metrics fixes.

## Highlights

- Actions left executing after an agent restart now reconcile to an honest
  inconclusive result, and administrators have a guarded force-fail route for
  legacy or otherwise unrecoverable rows (#1649).
- Kiosk tokens remain confined to monitoring surfaces even after Escape, and
  limited security-status responses no longer produce false configuration
  warnings or links into inaccessible settings (#1650).
- Alert delivery health now classifies terminal failures into safe diagnostic
  categories, while repeated open alerts no longer inflate history or
  telemetry counts.
- Proxmox storage restricted to specific nodes no longer appears Offline on
  nodes that cannot mount it (#1645).
- A working rootful Docker daemon now outranks any rootless socket when the
  installer picks the container runtime, so a transient socket-activated
  rootless Podman socket can no longer capture the agent service (#1647).
- Host CPU usage is averaged across the report interval instead of a blocking
  one-second spot sample, so mostly-idle VMs stop reporting inflated CPU
  (#1648).

## Fixed

- `scripts/install.sh` checks for a live system Docker daemon before it globs
  `/run/user/*` for rootless sockets, and returns without pinning
  `PULSE_DOCKER_RUNTIME`, `CONTAINER_HOST`, `PODMAN_HOST`, or
  `XDG_RUNTIME_DIR` into the agent unit when that daemon answers. This applies
  to auto-detection and to explicit `--enable-docker` runs alike.
- The Docker agent no longer reports `podman` purely because the runtime
  preference says podman. A podman-preferred connection that lands on a
  docker endpoint is reported as docker and keeps Swarm collection enabled,
  while an unlabeled endpoint with no runtime signals still honors the pin.
- When the bound container socket disappears mid-run, the agent re-runs
  runtime discovery after three consecutive daemon-unavailable collects and
  swaps the connection behind a stable client handle, so concurrent cleanup
  and container-update work is not disrupted.
- Host CPU usage is computed from the busy/total delta of the cumulative CPU
  time counters between collections. The old one-second spot sample is kept
  only as the fallback for the first collection and for counter resets such
  as a reboot or a live migration, and it no longer delays every collect by a
  second.
- Proxmox storage rows are filtered through the datacenter-level node
  restriction before they enter the per-node inventory. A shared or local
  storage constrained to one node therefore appears only on that node, while
  unrestricted and deliberately disabled storage remains visible (#1645).
- An executing action whose correlated agent receipt is interrupted or
  tombstoned now settles immediately as inconclusive. A receipt that remains
  accepted, started, or not found is allowed to finish and settles only after
  the existing one-hour stuck threshold; transport errors still leave the
  action pending because they are not execution evidence (#1649).
- Administrators can force-fail an unrecoverable executing action through the
  governed action lifecycle without touching transport or overwriting an
  already-terminal result. The route requires the execute capability,
  `settings:write`, and the admin approval floor, and records the operator
  attribution (#1649).
- Escape no longer disables kiosk confinement for monitoring-only sessions.
  Those sessions stay away from Settings, Patrol, and alert-configuration
  routes, while the existing temporary header reveal remains available.
  Security warnings now respect the authority tier of
  `/api/security/status`, so a deliberately truncated response does not look
  like disabled security controls (#1650).
- Notification audit records classify terminal failures as authentication,
  rate limiting, connectivity, TLS, configuration, rejection, or unknown.
  Alert delivery health shows the dominant class with practical guidance, and
  optional usage telemetry exports only bounded aggregate counts—never error
  text, destination details, or alert content.
- Repeated writes for the same open alert occurrence coalesce at insertion
  time, while genuine refires and explicit severity transitions retain
  separate history. This keeps incident history and alert telemetry aligned
  without waiting for the daily cleanup pass.

## Changed

- The normal Build and Test workflow now runs frontend, sharded backend,
  script-smoke, and benchmark jobs in parallel. A local pre-push helper covers
  the canonical governance, mutation registry, touched Go packages, build, and
  frontend type-check failures that most often make `main` red.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.4` only when you are
comfortable testing an RC. The rollback target is `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Hosts that were previously pinned to a rootless Podman socket by the installer
pick up the corrected runtime when the agent install is re-run. `CPUUsagePercent`
keeps its meaning in the host report schema, so no dashboard, threshold, or
alert configuration needs changing.

The action recovery loop automatically revisits executing rows after upgrade;
no manual database repair is needed. Notification audit storage adds its
failure-class column in place, and existing rows remain valid as the `unknown`
aggregate until new terminal outcomes are recorded.

This server candidate has no mobile compatibility change and does not require a
companion build upload. No public mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
