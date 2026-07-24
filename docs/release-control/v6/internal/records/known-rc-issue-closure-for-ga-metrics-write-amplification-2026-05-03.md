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

## 2026-07-24 Physical-Write Revalidation

The earlier proof established rollup controls and bounded retention, but it did
not attribute physical writes by SQLite object or quantify WAL/checkpoint
amplification. Issue `#1124` therefore remained open after a v6.1.1 operator
reported continued writes. A deterministic 2,197-resource estate
(Docker containers, Proxmox guests, agents, Kubernetes pods, storage targets,
and physical disks) now persists 393,630 samples over 30 simulated 10-second
polls and reports logical payload, per-object `dbstat`, page-cache, WAL,
checkpoint, process-write, restart, integrity, and retention counters.

The v6.1.1 and pre-fix `main` store implementations were byte-identical. Their
four-index schema wrote 279,063 WAL frames (1,149,739,592 bytes) for
21,214,200 logical payload bytes. The retained main database was 106,991,616
bytes, of which the overlapping `idx_metrics_lookup` and
`idx_metrics_unique` trees each occupied 24,014,848 bytes. This identifies the
amplification source directly: each metric insert maintained the table,
`sqlite_sequence`, and four indexes before checkpointing the dirty pages back
to the main file.

The revalidated schema makes the lookup index unique and orders its columns as
the metric identity, eliminating the second overlapping tree without changing
query order or durability settings. On the same workload it wrote 187,616 WAL
frames (772,977,952 bytes), a 32.8% frame/byte reduction, and retained
82,948,096 bytes. Linux process I/O accounting measured WAL-phase writes
falling from 1,155,710,976 to 774,930,432 bytes and explicit checkpoint writes
from 106,983,424 to 82,939,904 bytes (32.1% fewer total bytes). With the
production 4,000-page auto-checkpoint enabled, a traced 157,452-sample run
reduced total process write bytes from 392,445,952 to 257,208,320 and `fsync`
calls from 42 to 32.

The migration keeps the legacy unique index authoritative until deferred
startup maintenance transactionally swaps the indexes. Tests cover legacy
duplicates, close/restart, a process exit between index drop and replacement,
backup/restore from both schema generations, writes racing the migration, and
reads remaining available on the four-connection WAL pool. Query-plan checks
still require indexed searches. Concurrent read/write p95 remained 0.88 ms
against a 5 ms SLO, and the 500-node/10-reader dashboard p95 remained 2.37 ms
against a 30 ms SLO. Retention removed all 393,630 expired rows and incremental
vacuum reduced the 82,948,096-byte database to 36,864 bytes with zero freelist
pages in 2.57 seconds.

The durable store remains in WAL mode with `synchronous=NORMAL`; no
authoritative history moved to volatile memory. The fix is on `main` for the
next release and is not part of v6.1.1.
