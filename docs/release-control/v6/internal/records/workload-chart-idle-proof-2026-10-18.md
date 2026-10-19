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


## Final source comparison and absolute qualification

The final shared-binding head is
`e9dbca7c2b7159e84b922eeb5e4895ad4bb95611`. Its hosted proof in
[run 39180037366](https://github.com/rcourtman/Pulse/actions/runs/39180037366)
used synthetic merge `1cbbe76f2d6e063c3c59508f6da7c8688cbc82b3`, tree
`7ab6a9f73e466e362e2fd45f877c480f4d2fe160`, with current main parent
`6209d03d5a3895005f48b365cc2eeb8f1483f072` and that final PR head. The runner
exposed four AMD EPYC 7763 CPUs, Go 1.26.9 and GOMAXPROCS=2. Fixed executable
SHA-256: `74965528935c2a0da9c21dc9b33d874727aa12e430ab6916244f0b44088bbe25`.

This hosted attempt failed both the explicit latency assertion and capacity
validation: p50 85.225129 ms, p95 119.002157 ms and p99 122.928564 ms. Four of
21 proof intervals recorded 12.16% to 25.56% background CPU, predominantly
`provjobd2939212`. Admission's six intervals were at least 98.998% idle and the
three after intervals at least 99.649% idle; no guest, steal or swap activity
was recorded. Quiet admission does not excuse interference during measurement.
All intervals and the adverse result remain retained. Artifact
[12154824982](https://github.com/rcourtman/Pulse/actions/runs/39180037366/artifacts/12154824982)
has GitHub digest
`ed22be6f8ebc55bebeb5d5ba6015159b206f6f6271192d8c93665608bc6ddb7f6`.
The entire job log and a per-file hash manifest are retained in the task's
`pr-3070/publication-e9dbca` evidence directory. This attempt does not establish
the idle-worker SLO.


The final paired comparison passed. It sampled baseline
`ed4a2c6b54ee1d139d5f198950c53b403596c17c` (tree
`d11056db9ba2b097525b90000a550695b3a3355c`) and synthetic merge
`1cbbe76f2d6e063c3c59508f6da7c8688cbc82b3` on the same Intel Xeon Platinum
8573C worker, Go 1.26.9, runtime-default GOMAXPROCS, ten alternating 100 ms
samples each. Four-hour cold charts improved 117.84 to 79.53 ms (-32.51%,
p < 0.001); warm-handler and HTTP-transfer differences were not significant.
Cold-chart allocation volume remained 21.47 MiB and approximately 508,200
allocations. Small rollups improved 3.400 to 3.205 ms (-5.73%, p < 0.001),
with 3.106 to 3.126 KiB (+0.64%) and 100 to 106 allocations (+6%). The
500-node rollup improved 101.20 to 81.24 ms (-19.73%, p < 0.001), with
3.875 to 3.480 KiB (-10.18%) and median 109 allocations in both samples.
The 100-point write batch measured 1.568 to 1.672 ms (+6.58%, p = 0.007).
The complete time, bytes and allocation comparison raised no flags against
the unchanged >10%, p < 0.05 rule; the smaller increases are retained.
Artifact
[12156880181](https://github.com/rcourtman/Pulse/actions/runs/39180037366/artifacts/12156880181)
has GitHub digest
`d0d5af9567df10b4aa8b12e4d9495138b0ab3dd9026b03188161211edbf9ad6b`.
Baseline/candidate metrics executable SHA-256 values were
`52c6ab5b03a351a4a2ecdbff7875bd834a6eeb47647cae1daf463a2e51216173` and
`c6b9500e6f752acde29a0c3441edfb0de3e39098ee05d5cddb2b08dbc245f9d3`.
Full metadata retain the other affected-package executable identities and
all 20 sample timestamps.

All nine required source checks passed at the final PR head. Protected
[PR 3070](https://github.com/rcourtman/Pulse/pull/3070) merged at
2026-10-19T00:36:33Z as `5153c4ccda35cc50f12f32e3f6b98a87cb90f6b5`, with
the same tree `7ab6a9f73e466e362e2fd45f877c480f4d2fe160` as the tested synthetic
merge. That source landing neither waives the capacity-invalidated hosted
proof nor constitutes release qualification.

## Observed idle failure and remaining row-boundary cost

The next disposable proof,
[run 39181569505](https://github.com/rcourtman/pulse-enterprise/actions/runs/39181569505),
used that exact landed public source, private control
`4f8f9542ee122471c0bfe4f51334e7ef0d19796b`, Go 1.26.9 and two Intel Xeon
3.30 GHz vCPUs with GOMAXPROCS=2. The fixed executable SHA-256 was
`416ac833fc16778efe0fe09e94775d25e93ffe8fde0d4bf0e1b42d060f0beed9`.
This time capacity validation passed: six admission samples established
30.003 seconds of quiet capacity (at least 98.596% idle), all 19 measurement
intervals had at most 2% background CPU, and guest, steal and swap were zero.
The three after samples were at least 99.4% idle. The unchanged strict test
still failed: p50 77.749800 ms, p95 90.425504 ms and p99 98.667173 ms against
90 ms. The 0.425504 ms miss remains a failure; rounding or another unchanged
attempt does not establish the SLO.

All ten retained textual files were extracted with original byte counts and
SHA-256 hashes verified. The complete job log SHA-256 is
`b1f55ca54d3dc9b500eb1cde3e12db7950d4a037020d1b737964ebe6cc4b63d6`.
The package also resides in the native task's
`idle-evidence/private-proof-39181569505` directory, with its per-file manifest
verified after transfer. No Actions diagnostic artifact was published.

The separate diagnostic measured 78.198713 ms/op, 22,322,470 B/op and 508,181
allocations. Its profile attributed 77.81% cumulative CPU to the retained
reader, 63.66% to `database/sql.Rows.Next`, 40.78% to SQLite's VM and 11.79%
to record comparisons. Text-column decoding accounted for 12.62%. These CPU
samples include the benchmark's setup; they are attribution evidence, not a
replacement SLO measurement.

A bounded linear metric-filter experiment returned identical observations
but did not demonstrate a consistent timing improvement, so the production
membership predicate remains unchanged. A second alternating probe replaced
repeated resource-ID text columns with a request-local integer and explicitly
ordered completed series in Go. Both variants returned 36,000 observations
with digest
`0720c6e78804f01075db817e20fb5920f078519523af67e5fce7b2d1db04f8b0`.
Twelve native rounds had median 58.2208005 ms for text identities versus
51.237231 ms for resource numbers. The new plan retained bounded primary
ranges and no temporary value sort. Shared-worker timings remain diagnostic.

The shared retained reader now binds a bounded requested-resource table for
raw fleet reads and returns the statement-local resource number rather than
decoding/copying its resource ID for every observation. Numbers resolve against
that statement's normalized resource list and never become persisted or public
identity. The same metric/tier/time predicates and coverage probes select the
observations. Completed detached series explicitly restore stable timestamp
order in Go, removing SQLite's merge of tier branches by repeated identity
strings. Single-resource reads and ordered streamed display buckets retain
their existing paths and bounds. Regression coverage compares both point
projections with single-resource reads on rowid and clustered layouts, including
mixed tiers, nullable/zero extrema, reused names in another resource type,
unrequested metrics, reordered scopes, missing/duplicate IDs and the
500-resource chunk boundary. Final qualification is recorded below.

The candidate passed the native full metrics suite, retained-reader correctness
and query-plan checks, retained race checks (31.366 s), chart API tests, selected
downstream chart sampling/SLO tests (24.315 s), and v5 metrics compatibility
(0.143 s). The probe was hash-checked and moved outside the package before
compiling either baseline or candidate. All qualification exits were zero.

The baseline metrics/API executable SHA-256 values were
`ce878df8c9df1193b457742ef913dac8073f29777b53aeb9148fecfd8b9a1686` and
`ce964286839a7621c149a0ee3b7ffec06a1114f999f70a1053bc3c0b34159469`.
The corresponding candidate values were
`3de1f7a264c1b09a2cb89d2c4440fa4b66417576853b4755757c255665ef5b79` and
`51c7b0cdb6d2fa371b9513e355f4a681e9b4bb134732f6967fd009ed4f0becd4`.
Four source hashes are retained in `ordinal-query/candidate-source.sha256`;
`store.go` is `535bce025ab3e544b6104bbd537b9c40022debdad0b96df91e6d5469c4162cba`
and `store_query_plan_test.go` is
`5aa9149ea8c4e1e201112fbe83d636240d1435f5bee8b686c87506e3d4b4f9b3`.

Ten alternating native 100 ms samples per variant/case, with fixed binaries
and GOMAXPROCS=2, measured cold-chart median allocation volume of 22,509,840
versus 19,696,848 bytes (-12.50%) and counts of 508,193 versus 340,887.5
(-32.92%). Median chart times were 102.660828 versus 103.846864 ms; their
respective full ranges were 87.728537–134.246896 and
85.662542–146.783133 ms. The adverse median and all samples remain retained.
This shared-worker comparison establishes neither a chart latency improvement
nor the absolute budget.

Batch-read medians for 10, 50 and 100 resources were respectively
7.429404/5.795782, 36.9792995/31.2997015 and 69.734954/62.305637 ms
(baseline/candidate). The unchanged single-resource path measured
0.6401185/0.5719735 ms. The unchanged streamed 50-resource display-bucket path
measured 43.810747/44.7852855 ms, with median 120,605 allocations in both
variants. These native timings are diagnostic. Complete logs, exits, source
and executable manifests and the 120-row raw sample table are retained in
`ordinal-query-qualified`. Fresh paired CI and the unchanged strict proof on
observed idle capacity remain required after source landing.
