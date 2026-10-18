# Workload chart storage and idle-worker qualification, 18 October 2026

Scope: the remaining cold-service four-hour workload-chart latency gap under
L10 `performance-post-rc-headroom`. The acceptance budget remains 90 ms p95.
Source landing, a relative benchmark, and an idle-worker SLO proof are separate
observations. This record does not close the broad L10 residual.

## Retained failed proof and worker inventory

The installed disposable `pulse-ci-vm-large` profile provides two vCPUs and
4 GiB of RAM. The persistent worker shares other workloads. A configured
performance profile alone is not an available trusted proof route. The manual
source-bound route is owned by the L10 contract at
`pulse-enterprise:.github/workflows/verify-workload-chart-latency.yml` and its
runbook `pulse-enterprise:scripts/workload-chart-latency-proof.md`.

[Run 39168929815](https://github.com/rcourtman/pulse-enterprise/actions/runs/39168929815)
used public source `8a590102fef2089e938815bea280fee5423334fc`, private control
`4f8f9542ee122471c0bfe4f51334e7ef0d19796b`, Go 1.26.9 and the disposable runner
`pulse-ci-vm-enterprise-large-1018214122-359c52` (two Intel Xeon CPUs). Executable
SHA-256: `0bec46b33b3f3a1e23adac8309093966d528c4d6b8c9b2b2786b3e1786255553`.

Its unchanged strict proof failed: p50 99.322850 ms, p95 112.164352 ms and p99
121.091084 ms. Admission was quiet, but one proof interval recorded 0.49505%
CPU steal against the existing 0.1% limit. The result therefore establishes
neither the latency budget nor an idle-worker proof. All 24 during-proof
capacity samples, admission/after samples and diagnostics are retained. The
logs were extracted with original per-file byte counts verified, without
omitting the failing sample or substituting a second SLO attempt.

The separate diagnostic benchmark measured 99.635869 ms/op. Its CPU profile
attributed 80.55% cumulative CPU to the retained query, 52.74% to SQLite's VM and
21.18% to `BtreeTableMoveto`. Finding each identity entry then seeking its rowid
table payload remained a substantial cost. The pinned SQLite driver opens
FULLMUTEX connections; an unsupported connection-string option would not
remove that cost. Cache and mmap size experiments did not demonstrate a
reliable benefit and were not adopted.

## Canonical storage change

`pkg/metrics/clustered_layout.go` stores observations in
`metrics WITHOUT ROWID`, keyed by
`(resource_type, resource_id, tier, timestamp, metric_type)`. Values and nullable
bounds now live in the same B-tree used by retained range reads. The only
secondary tree remains `(tier, timestamp)` for retention. The identity column
set and all eight observation columns retain their existing semantics.

Installed rowid databases keep their authoritative unique index while deferred
startup maintenance copies and swaps the table in one `BEGIN IMMEDIATE` WAL
transaction. Existing readers retain their old SQLite snapshots. Writes queued
behind the transaction resume after commit. Copy failure, cancellation or a
crash before commit rolls back the entire schema and payload change. A
conflicting replacement table is preserved. The free-space guard requires
three times the used database pages plus 16 MiB; inadequate capacity defers the
migration and leaves the installed format readable. Layout and auto-vacuum
share the existing 30-minute rewrite context, canceled on Store shutdown.

Legacy stores with no authoritative unique index still establish uniqueness
synchronously before upserts. Already-authoritative stores skip the redundant
old index rebuild and migrate directly. Readers fix their index selection for
each snapshot; a stale legacy-index error retries once only after verifying
the new primary key. The physical layout participates in the bounded SQL cache
shape. No results or tier presence are cached. An older executable can reopen
the row-compatible table and recreate its old index; subsequent maintenance
removes those known redundant indexes after checking the clustered primary key.

The retained query engine, tier reconciliation, Docker observation provenance,
nullable-bound fallback, value projection, LTTB sampling, chart point limits,
cache bounds, write retry policy and SLO thresholds remain the same.

## Qualification evidence

The parent-owned native evidence directory is
`/opt/pulse-release-worker/task-proofs/chart-latency-slo-20261018/clustered-qualification`.
The initial full metrics suite passed in 219.902 s. The 12-tick deterministic
write profile preserved 157,452 observations across 2,197 resources in 72 writes:
20,842 WAL frames (85,869,072 bytes), below the unchanged 23,000-frame ceiling.
The rowid comparison used 20,223 frames, so the clustered format trades about
3.06% more WAL traffic for removal of the per-observation table lookup. Its
checkpoint wrote 20,672,512 bytes and the main database used 20,725,760 bytes,
versus 23,797,760 and 23,846,912 respectively in the rowid comparison. These
physical write/size observations are distinct from latency qualification.

With the production 4,000-frame checkpoint threshold, four connections and a
paced concurrent reader, the initial 30-tick profile preserved 393,630 rows
through restart and integrity checking. It completed 2,107 reads (p95 0.497286
ms, maximum 17.202902 ms), and retention removed every row. The shared native
worker is unsuitable for an absolute chart SLO claim; these profiles qualify
storage behavior only. Final source-bound verification is recorded separately
from these initial diagnostics.

The final fixed-executable production comparison used 393,630 observations,
180 writes, 4,000-frame checkpointing and the same paced-reader configuration.
The historical rowid profile wrote 728,674,304 process bytes; the clustered
profile wrote 789,917,696 (+8.40%). Final explicit checkpoint writes were
9,256,960 versus 10,158,080 bytes (+9.73%). Main database size was 61,054,976
versus 52,621,312 bytes (-13.81%). Both profiles preserved every row through
restart and integrity checking, then retained zero rows after purge. Their
respective read counts were 18,917 and 4,080. The shared worker produced large
write/read timing outliers, so these sequential profile timings do not prove
a throughput or tail-latency improvement. Retain the increased write volume
as a measured tradeoff; the production checkpoint threshold, 23,000-frame
no-auto-checkpoint ceiling and steady-state write guard are unchanged.

The final full metrics suite and migration race tests passed. The race run
completed in 64.407 s, including the atomic-swap cancellation/crash paths,
independent Store handles and cached read shapes. Temporary layout/cache probe
code was hash-checked and moved out of the package before compiling the final
fixed test executable.

Final fixed metrics executable SHA-256:
`aa501813ca9cd8d6de5d61ae03ac36005e640dd9aa812c9b6da7ca651b6dd689`.
Per-file Go source hashes are retained in `clustered-final/source.sha256`.
The v5 metrics compatibility tests, downstream monitoring/chart sampling tests
(67.669 s), and 11 chart API tests (0.625 s) passed. The separate cold-service
benchmark reported 111.005964 ms/op, 22,262,176 B/op and 508,181 allocs/op on the
shared AMD Ryzen worker. That timing is retained without treating it as a
90 ms SLO proof. Status and contract audits passed; the new explicitly routed
test must be staged before the registry audit can recognize its tracked path.
