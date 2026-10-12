# Docker restart intelligence lab

This ignored-artifact harness exercises one uniquely labelled disposable Docker
container, volume, and network in the existing Colima VM. It refuses any
implicit or non-Colima Docker context, uses the existing `alpine:3.20` fixture,
and removes only resources with the exact run label.

Run only after explicit local-lab authorization:

```sh
python3 scripts/intelligence_lab/docker_restart_colima.py --run-id task06-20260712-090000
```

Evidence is written beneath `tmp/intelligence-lab/<run-id>/`, which is ignored.
Only the explicit redacted JSON, screenshot/trace, report, and checksum
basenames are retained there. Transient Pulse and Unified Agent databases live
under `tmp/intelligence-lab-scratch/<run-id>/` and are deleted before the
artifact allowlist and checksum manifest are accepted.
The second cleanup must be a no-op and the preexisting resource inventory must
be unchanged. Container restart has no product rollback; fixture deletion is
lab compensation only.

## RG-06 limited unattended autonomy

The RG-06 runner is separate from the human-approved Docker restart journey.
It requires a clean archive of the audited SHA, an already-present
`debian:bookworm-slim` image, and explicit external artifact and scratch directories:

```sh
python3 scripts/intelligence_lab/patrol_autonomy_colima.py \
  --run-id <release-id> \
  --sha <full-release-sha> \
  --repo <clean-git-archive> \
  --artifact-dir <external-artifact-dir> \
  --scratch-dir <external-scratch-dir>
```

It extracts that pinned archive once, builds `cmd/pulse-agent` from the
extracted source, and runs the Go certification test from the same extracted
source directory; the archive SHA binding is recorded in the artifact. It
refuses to pull images, and starts only one exact-run-labelled disposable
Debian agent. The agent creates a 65 MiB `.deb` fixture in the
temporary `/var/cache/apt/archives` tmpfs and connects through the production
agent WebSocket. The Go proof obtains the resource through
`unifiedresources.HostIngestRecord`, so `clean_package_cache` is advertised
only by the production telemetry/pressure/command/receipt gates. The action
then uses the production `hostStorageCleanupActionExecutor`, records the
server command count and dispatch receipt, independently reads back the cache,
and reconciles terminal ActionResultV2 truth and the finding.

Each revoked, downgraded, emergency-stop, stale-resource, and Never barrier
creates a valid finding/investigation first. The artifact measures cache bytes,
reclaimable/fingerprint state, container identity/running/start state, agent
command count, attempt-record count, aggregate dispatch count, receipt-record
count, audit/event deltas, authority/config digest, and finding resolution
before and after. Every barrier must be refused with its expected canonical
reason and never reach completed state. Triple-zero
means measured unauthorized mutation, transport dispatch, and authority write
counts are all zero; an expected lifecycle refusal is recorded separately and
is not falsely called triple-zero. Two label-scoped cleanup passes compare the
full pre-existing containers/volumes/networks/images inventory, remove scratch
state, redact/checksum artifacts, and stop Colima. The one authorized dispatch
is reported separately from all negative-barrier counts.
