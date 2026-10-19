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

## First paired CI result and rollup correction

[Run 39174382045](https://github.com/rcourtman/Pulse/actions/runs/39174382045)
compared baseline `ed4a2c6b54ee1d139d5f198950c53b403596c17c` with PR head
`5c37e620b45b50426c159a8a68af8e7bea04896e` through synthetic merge
`1b5a54e8a050217d84ae5ea0ddb64058840a5f63` (tree
`c7ecd028d86d4fafc8854f05182048e1f2bc2163`). All nine required source checks
passed. Source landing was paused after the optional paired benchmark exposed
a related production regression.

Ten paired 100 ms samples measured the four-hour cold chart at 124.37 ms
versus 83.75 ms, a 32.66% improvement (p < 0.001). Allocation volume remained
21.47 MiB and about 508,200 allocations per request. The warm handler and HTTP
transfer comparisons were not statistically significant. The same comparison
flagged `RollupTierBatched` at 3.919 versus 4.899 ms (+25.03%) and its 500-node
counterpart at 115.8 versus 145.7 ms (+25.78%), both p < 0.001. These are real
same-source production paths, not the earlier unchanged history-filter
benchmark signal. Artifact
[12154581287](https://github.com/rcourtman/Pulse/actions/runs/39174382045/artifacts/12154581287)
has GitHub digest
`5f9c9eeb9cfe3d3e67d07e5d50245f15dc1df19a937639e4d58b90227ae2c56c`.

The separate hosted strict job measured p50 84.827550 ms, p95 119.864148 ms and
p99 124.118998 ms. It failed both the explicit 90 ms assertion and capacity
validation: four proof intervals included a process named `provjobd4100855`
using 12.69% to 25.31% of total CPU. No matching process source was found in
the chart proof, its tests or reviewed runner scripts; its origin is not
established. The runner exposed four AMD EPYC 7763 CPUs, with GOMAXPROCS=2 and
real Actions identity. Admission and after samples were quiet; the interfering
intervals remain retained. Fixed executable SHA-256:
`f2d2dc6f86bd9e0a064d52d61df377bb4d3d71d0ac148c3e8ec18c48585a1085`.
Artifact
[12153956818](https://github.com/rcourtman/Pulse/actions/runs/39174382045/artifacts/12153956818)
has GitHub digest
`7ae238c3982119d122f1eb1a5b3c67c7059894bd61b2e1ecd602b43b2ad82b1a`.
Neither the faster cold benchmark nor this invalid-capacity SLO run establishes
the idle-worker 90 ms budget.

A fixed-executable native rollup profile attributed 34.09% cumulative CPU to
`BtreeIndexMoveto` and 17.18% to text comparisons in the aggregate sorter.
The old rollup scanned the tier/time secondary index, then sought the wider
composite primary tree once for every value. Clustered rollups now discover
only the resource identities in the source window through that covering
index, then read each resource's contiguous primary range. A query-local
integer resource number reduces repeated identity-string group comparisons.
It never changes persisted resource identity. The legacy aggregate remains
in use while the physical table is rowid-based. No extra index, cached data,
changed bucket limit or wider write budget is introduced.

Two retained native SQL probes compared the original aggregate, primary-order
grouping, resource ranges and integer resource grouping. Every variant
returned the same 4,000 rows from 40,000 source points on both schemas, digest
`eb61ee4816ff653ad69d7415d28fd234f04496a1c257bc4aad3ad31be8c1e288`.
In eight alternating rounds of the second probe, the clustered original and
integer-grouped medians were 161.180 and 103.572 ms. The legacy original median
was 123.433 ms. Shared-worker timing is diagnostic only; the retained paired
CI comparison must establish whether the rollup regression is removed.
Both temporary probes were hash-checked and moved outside the package before
compiling the revised production test executable.

The revised fixed executable passed targeted rollup tests, the full metrics
suite and the rollup race suite (3.866 s). Its SHA-256 is
`b439f1c36464ad14f6ea1392b31dbb83ced8f66948af3c06a504bd24856e2867`;
five-file source hashes are retained in `clustered-rollup/source.sha256`.
Tests compare legacy and clustered aggregates across three resource types,
reused resource names, three metrics, all rollup tiers, nullable/zero bounds,
window edges and authoritative pre-existing outputs. The query-plan guard
requires covering time-window discovery and a primary-key resource/time range.

Ten native 100 ms samples per benchmark gave revised/previous clustered
medians of 4.735637/5.7315405 ms for the small rollup and
118.3057575/165.052439 ms for the 500-node rollup. These sequential diagnostics
are retained without substituting for the revised paired CI result or the
still-pending strict idle-worker chart proof. The previous executable and
initial adverse CI results are preserved.

## Revised paired result and shared bindings

The revised comparison in
[run 39177248750](https://github.com/rcourtman/Pulse/actions/runs/39177248750)
used PR head `fc58c597caf0051389c7bb3a5a6e10e7dfb3ac9b` through synthetic merge
`e79e0546086124d96ee75bd64722c8fc3f8ed6bf`, tree
`2143d881c0c8cbcb7d8696c204f42149623a3d5b`. Its other parent was current main
`6209d03d5a3895005f48b365cc2eeb8f1483f072`; the main advance changed only
Proxmox guest-test fixtures. The paired baseline remained
`ed4a2c6b54ee1d139d5f198950c53b403596c17c`.

The small rollup improved from 4.309 to 4.254 ms (-1.28%, p = 0.011), and the
500-node rollup from 121.2 to 112.1 ms (-7.48%, p < 0.001). The four-hour cold
chart improved from 113.97 to 78.69 ms (-30.96%, p < 0.001), with unchanged
21.47 MiB and approximately 508,200 allocations. The remaining benchmark flag
was the small rollup's 3.117 to 3.453 KiB (+10.76%, p < 0.001). Its allocation
count rose from 100 to 105. The 500-node case likewise gained five allocations
and about 352 bytes, below that case's relative flag threshold. Artifact
[12155027036](https://github.com/rcourtman/Pulse/actions/runs/39177248750/artifacts/12155027036)
has GitHub digest
`f238c3230b214064fde984765b81c6f93f22f7addb410f7aad43d5ba404a69cd`.

The duplicated discovery tier/window arguments caused those extra allocations.
Both stages now reuse the ordinary aggregate's same six SQLite binding slots,
removing the second argument array and repeated boxing without changing
resource discovery, primary ranges or aggregation semantics. The existing
cross-layout aggregate and query-plan tests exercise the shared bindings.

The hosted strict test in that revised run passed its latency assertion:
p50 50.135919 ms, p95 67.674439 ms and p99 74.205989 ms, explicit SLO 90 ms.
It still failed capacity validation. Two of 12 concurrent capacity samples
recorded `provjobd1628928` at 21.0% and 25.25% total CPU. The four-vCPU runner
was an AMD EPYC 9V45, with GOMAXPROCS=2 and Go 1.26.9. This is different
hardware from the prior public attempt; their absolute timings are not a
source-only comparison. Admission/after receipts, both adverse intervals and
the passing test remain retained. Fixed executable SHA-256:
`89dfcd17f28ccd16d88d1b663ff8266975486adb5804876ae65c1269a4952b98`.
Artifact
[12155196024](https://github.com/rcourtman/Pulse/actions/runs/39177248750/artifacts/12155196024)
has GitHub digest
`65d01e1a1614836a025f747869d577daa21032a6a362a621413807d4b2c1e150`.
The test establishes a passing latency assertion on that runner; the failed
capacity guard still prevents an idle-worker qualification claim.

The shared-binding executable passed the complete targeted rollup correctness
and plan suite and the rollup race suite (6.010 s). SHA-256:
`ce878df8c9df1193b457742ef913dac8073f29777b53aeb9148fecfd8b9a1686`.
Its five source hashes are retained in `rollup-bindings/source.sha256`.
Ten native samples per case measured small-rollup median allocation volume
3,204.5 versus 3,532.5 bytes, and 500-node volume 3,896 versus 4,248 bytes.
Median allocation counts were 106 versus 105 and 113 versus 112 respectively;
reused numbered bindings reduce volume while the driver handles their names.
Sequential shared-worker median times were 7.8832245 versus 6.780017 ms and
164.7742585 versus 149.3372525 ms. These adverse timing observations are
retained without attributing them to the binding change or substituting for
fresh paired CI. The full metrics suite passed before the binding-only change;
the final native proof exercises all affected rollup paths and boundaries.
All nine required source checks for `fc58c597caf0051389c7bb3a5a6e10e7dfb3ac9b`
ultimately passed, including REST-0 at 23:57:41Z. Its allocation flag and
capacity-invalidated SLO are preserved separately from those source checks.
