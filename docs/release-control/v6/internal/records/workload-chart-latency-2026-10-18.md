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
