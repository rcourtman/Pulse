# Known RC Issue Closure For GA Metrics Write Amplification Record

- Date: `2026-05-03`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `https://github.com/rcourtman/Pulse/issues/1124`
- Linked comment: `https://github.com/rcourtman/Pulse/issues/1124#issuecomment-4275445806`
- Result: `passed`

## Context

Issue `#1124` remains open after the earlier v5 fixes for agent-config reads,
metrics retention cleanup, and WAL truncation. The linked 2026-04-19 comment
reports Pulse `5.1.28` on current PVE inside a Debian 13 LXC with disk writes
visible every 5 minutes and memory use around 3 to 4 GiB on a 5 GiB container.
The attached screenshot shows periodic write spikes of roughly 10 MB, matching
the hard-coded 5-minute metrics rollup cadence in the current metrics store.

Later comments on the same issue also ask for a way to keep metrics history in
memory or move the metrics database to a path that Docker can mount as tmpfs.
That feedback is valid for SSD-sensitive self-hosted installs, but the canonical
v6 fix belongs in the shared metrics persistence layer rather than in a
deployment-specific workaround.

## Disposition

The v6 metrics store now reduces default rollup write amplification and exposes
explicit operator controls:

- `pkg/metrics/store.go` no longer hard-codes the background rollup ticker to 5
  minutes. The default rollup cadence is 15 minutes, with a 5-minute lower bound
  and an upper bound tied to half of raw retention so data is aggregated before
  pruning can remove raw samples.
- `PULSE_METRICS_ROLLUP_INTERVAL` lets operators choose a longer cadence when
  they prefer fewer, larger rollup writes over fresher minute aggregates.
- `PULSE_METRICS_DB_PATH` lets Docker/LXC installs move only `metrics.db` to a
  tmpfs or dedicated mount while leaving `/data` or `/etc/pulse` durable for
  config, encrypted credentials, sessions, and tokens.
- The public metrics-history and Docker documentation now describe the tmpfs
  tradeoff and warn that tmpfs-backed metrics history is ephemeral.

This does not make durable metrics free of disk writes. Persistent history still
requires writes for raw samples, rollups, retention, and SQLite metadata. The
fix closes the v6 product gap raised by `#1124`: the default rollup path no
longer forces 5-minute write spikes, and SSD-sensitive operators have a governed
runtime path for memory-backed metrics storage without moving secrets off
durable storage.

## Proof

- `go test ./pkg/metrics -count=1`
- `go test ./internal/config -run 'TestLoad_EnvOverrides_(MetricsStorage|InvalidMetricsRollupIntervalIgnored)$' -count=1`
- `go test ./internal/monitoring -run '^$' -count=1`
- `go test ./internal/config -count=1`
- `python3 scripts/release_control/status_audit.py --check`
- `python3 scripts/release_control/contract_audit.py --check`

## Outcome

The metrics write-amplification path reported in `#1124` has a v6 runtime fix,
operator-facing controls, documentation, and subsystem-contract coverage. The
`known-rc-issue-closure-for-ga` gate remains satisfied for the current v6
release candidate.
