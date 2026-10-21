# Workload chart latency investigation, 18 October 2026

Scope: the local 90 ms workload-chart SLO within L10's existing
`performance-post-rc-headroom` follow-up. This work does not close L10 or
change source/release qualification gates.

## Defect and correction

The retained metrics identity index is time-major. Ungrouped multi-metric
reads nevertheless requested metric-major output, forcing SQLite temporary
sorts on the fresh-service workload-chart path. Ungrouped reads now use the
index order for both filtered and unfiltered queries. The result assembler
already supports interleaved metric series; streaming display buckets keep
their required grouping. No index, retention rule, chart sampling algorithm,
payload shape or response-cache boundary changes.

The expanded query-plan guard fails against the original store with
`USE TEMP B-TREE FOR ORDER BY` and passes with the correction. Tier-coverage
tests compare multi-metric filtered reads (including duplicate requested
metrics) against unfiltered reconciliation for raw and display-bucket reads.
The full metrics package and seven SLO-budget selection cases pass.

## Diagnostic evidence

Baseline source: `52b40fd611112904cf7e987e0fc1e8df1c7a0cbf`, tree
`b1e58aeecb1c83ab2d17223d55bfd8353df3dbae`. Both diagnostic binaries use
Go 1.26.9 linux/amd64 and GOMAXPROCS=2 on pulse-dev (Ryzen 5 3600).

| Fixed executable | SHA-256 | 4 h cold-service diagnostic |
| --- | --- | --- |
| Baseline | `f9afc621e05d69d7c13769fa4e162badab9f90c679909184417602b1ef2ca47d` | 167.43 ms/op, 38 iterations |
| Sort correction | `f8318a86724b775e46cbd186944f17ce49e01711f9b9029499bbc9cee0be28f8` | 188.39 ms/op, 33 iterations |

These separate five-second CPU/allocation profiles ran on a shared, busy
worker. They are attribution evidence, not a matched speed comparison or
an idle-worker SLO result. The original profile shows SQLite sorter work;
the corrected profile leaves row reads as the dominant cost. Neither timing
establishes a speedup or satisfies 90 ms.

The native idle sampler required 30 continuous seconds with load1 below 2,
at least 90% idle CPU, at most 1% I/O wait, 0.1% steal and 2% guest CPU,
and no swap activity. An initial wait was stopped before any SLO measurement;
a later bounded five-minute wait expired without qualifying. Both retained
their raw capacity evidence and produced zero SLO samples. No other workload
or service was stopped to obtain an idle result.

Native raw evidence is retained under
`/opt/pulse-release-worker/task-proofs/chart-latency-slo-20261018/`.

## Absolute SLO verification

The optional `Workload chart 90ms SLO` job in Build and Test is the repeatable
proof surface. A fresh hosted VM compiles the candidate API test executable
before timing, records its committed HEAD/tree and binary hash, and observes
idle capacity before the process. CPU, guest, swap and
pressure samples remain in the `chart-latency` artifact alongside raw output
and post-measurement samples. The candidate must execute exactly one fresh
200-sample workload-chart test and report the existing 90 ms budget; skipped
tests and ordinary 550 ms hosted passes cannot satisfy it. Actual hosted
runner identity remains intact. Background contention invalidates the proof.

Since 21 October a contended hosted run reports a neutral, inconclusive
verdict instead of failing. In every hosted run since this job was
introduced, the runner's own `provjobd` daemon contended during the
measurement. See
[workload-chart-hosted-slo-2026-10-21.md](workload-chart-hosted-slo-2026-10-21.md).

The associated source-check run and retained artifact supply the measured
percentiles for that exact candidate. This record does not substitute a
diagnostic benchmark mean for those percentiles. The earlier historical
12.72% comparison remains preserved; changed harness boundaries cannot
retroactively resolve it. Broader performance headroom remains governed by
the existing L10 follow-up.

## Retained adverse evidence

The initial hosted attempt at head `2a77054035513eabc884135d3a8c19fc76e0956b`
passed its pre-run idle window, then observed 25.5% and 24.6% competing CPU
during the baseline process. Its 138.45634 ms p95 used the ordinary 550 ms
envelope and is not a valid idle-worker result. The candidate produced zero
samples because the initial proof stopped on that context failure. See
[job 117757206920](https://github.com/rcourtman/Pulse/actions/runs/39153766278/job/117757206920)
and [raw artifact 12147832177](https://github.com/rcourtman/Pulse/actions/runs/39153766278/artifacts/12147832177).

Absolute verification now runs the candidate directly; the existing paired
benchmark job retains relative context. The sampler also retains per-process
CPU names/counters and the measurement's I/O counters. It requires quiet I/O
before admission and keeps the test's own I/O in its measured latency rather
than classifying every I/O wait as an unrelated workload.

A separate snapshot-safe fleet-tier-pruning experiment passed the full metrics
suite. Ten alternating current-source comparisons measured 196.1 ms versus
179.8 ms (p=0.063), with 0.20% more bytes and 0.07% more allocations. This
shared-worker result is inconclusive; the experiment was reverted and its raw
samples remain under the native proof directory. It establishes no speedup or
SLO result.


## Follow-up decoding cost and candidate evidence

The candidate-only proof at head `0a6eb9b541f12a9dd6cb64caa7c231355426077e`
executed the strict 90 ms test: p50 106.672837 ms, p95 130.023547 ms and p99
144.836677 ms. It failed the budget. The retained capacity receipt also
invalidates idle qualification: `provjobd4515890` consumed 18.7–25.1% of the
four-logical-CPU worker during three intervals, with competing busy CPU
18.7–25.4%. See [job 117769795027](https://github.com/rcourtman/Pulse/actions/runs/39159417029/job/117769795027)
and [raw artifact 12148113764](https://github.com/rcourtman/Pulse/actions/runs/39159417029/artifacts/12148113764).
Neither the failed budget report nor idle admission alone establishes an
idle-worker SLO result. Native capacity remained unsuitable as well (load
above 15 and sustained I/O wait at 19:33 UTC). No provider or native workload
was stopped and no repeated measurement was selected for a passing result.

The original paired hosted comparison measured cold-service cost from
148.8 to 133.6 ms/op (-10.20%, p<0.001, n=10); warm-handler and HTTP-transfer
changes were not significant. [Job 117757206872](https://github.com/rcourtman/Pulse/actions/runs/39153766278/job/117757206872)
failed on two unchanged test-only legacy history-filter cases, which exercise
neither SQL nor collectors. These relative results do not establish 90 ms.

A second retained-read improvement avoids decoding duplicate floats when
stored extrema are NULL. Non-aggregated SQL now returns nullable bounds;
Go applies the same per-observation value fallback. SQL aggregation and
streamed buckets retain their original extrema semantics, including explicit
zero and independently missing bounds. The full metrics suite passed in
95.727 seconds. The final strengthened mixed-tier nullable guard passed all
twelve read-shape cases in 0.024 seconds. The first new test assertion incorrectly compared time-zone representations; it was
corrected to compare timestamp instants, and its failed log is preserved.

Ten alternating base/candidate diagnostic samples on the shared native
worker measured 142.3 versus 136.9 ms/op (p=0.579), so no latency gain is
established. Allocated bytes fell 4.24% (29.85 to 28.58 MiB/op) and allocations
fell 24.59% (674.6k to 508.7k/op), both p<0.001. This change is retained for
its measured allocation reduction. Executable hashes are
`737f1383deecd2d34e20f49731e1a634a690e8de789bb0221bd63a44b3441b70`
(base) and `843ea5877f711966fdbdf590854b5f3dca96319c5a9d4cd1f5f2fa3b03f98856`
(candidate); raw samples and metadata are retained as `nullable-*` under the
native proof directory. This is not an idle-worker result. The strict proof
for the resulting source remains the source-check run's retained artifact;
the 90 ms verification and broader L10 headroom follow-up remain open.

## Nullable candidate qualification and available capacity

At head `ea3d71a7990798c026a39480635a43ae1f3ea6c0`, the strict hosted
test reported p50 99.543808 ms, p95 126.435066 ms and p99 132.762651 ms.
The 90 ms assertion failed. Idle qualification also failed: two measurement
intervals had up to 25.373% competing CPU, with `provjobd3520822` consuming
24.875–25.124% of the four-logical-CPU guest. See
[job 117774534298](https://github.com/rcourtman/Pulse/actions/runs/39161106967/job/117774534298)
and [artifact 12149191504](https://github.com/rcourtman/Pulse/actions/runs/39161106967/artifacts/12149191504).
These percentiles are retained adverse evidence, not an idle-worker result.

The paired hosted comparison for the same source passed its regression
check. Against the PR base, cold-service time fell from 145.5 to 122.2 ms/op
(-16.04%, p<0.001, n=10). Allocated bytes fell 3.89% and allocations fell
24.59%. Warm-handler time increased 5.23% (1.137 to 1.197 microseconds,
p=0.001); this adverse change remains recorded even though it is below the
job's 10% regression threshold. Transfer time was not significantly changed.
See [job 117774534268](https://github.com/rcourtman/Pulse/actions/runs/39161106967/job/117774534268).
The earlier failed history-filter comparisons remain historical evidence;
this passing run does not erase them or establish absolute latency.

A fresh native proof for that exact committed source waited five minutes
and reached at most 15.001675 continuous quiet seconds. It never admitted
the test and produced zero SLO samples. Its final-native-proof metadata and
60 capacity intervals remain in the native proof directory.

The worker inventory has no registered self-hosted runner in the public
repository. The legacy enterprise runner shares pulse-dev; the separate
disposable CI pool was occupied or queued, and the cloud latency runner is
not an installed, eligible capability. No workload was interrupted, reserved
VM started, runner label changed or infrastructure provisioned for this proof.
Capacity and the 90 ms result remain unresolved.

## Canonical value-only chart reads

Chart consumers use timestamp/value points, but persisted chart reads first
transported retained extrema and built full metrics points, then discarded
the extrema while copying every series into the existing canonical model.
The store now exposes value-only single-resource and batch projections using
`models.MetricPoint`. Both projections use the same retained-read engine for
identity normalization, filters, chunking, tier reconciliation, snapshots,
ordering, display buckets and Docker observation provenance. The full reader
continues to return extrema. Query shape keys distinguish the projections
within the existing bounded cache; no result cache or tier cache was added.
Monitoring applies its existing LTTB sampling directly to fresh owned value
results. Payloads, point budgets and sampling algorithms are unchanged.

Verification passed all 48 query-plan shapes, mixed nullable/tier projections,
Docker observation/filter cases and fresh snapshot/caller-mutation checks.
The full metrics suite passed in 209.013 seconds and the full monitoring
suite in 480.630 seconds. A 1,000-point fixture proves the sampled chart
output equals the full-reader conversion reference for all/single/multiple
metric filters and remains detached across calls. Targeted API/chart route
contracts also passed.

Ten fixed alternating native pairs compared only the three runtime files
against `ea3d71a7990798c026a39480635a43ae1f3ea6c0`, with identical API tests,
Go 1.26.9 and GOMAXPROCS=2. Allocated bytes fell from 28.59 to 21.47 MiB/op
(-24.90%, p<0.001); allocations fell 0.10%. The shared-worker timing was
162.0 versus 183.8 ms/op (p=0.393, n=10, both +/-24%), so neither a latency
improvement nor a regression is established. The projection is retained for
its measured memory reduction and removal of unused transport/copies.
This diagnostic does not satisfy the 90 ms SLO.

The baseline executable SHA-256 is
`843ea5877f711966fdbdf590854b5f3dca96319c5a9d4cd1f5f2fa3b03f98856`;
the value-projection executable is
`e737a0db1ea10c44a491f3ff14311ef913271c8300306bab6e259370107fb4c8`.
All eight source hashes matched after both verification and comparison.
Raw results, metadata and logs are retained as `value-*` in the native proof
directory. The 90 ms proof and broader L10 follow-up remain open.
