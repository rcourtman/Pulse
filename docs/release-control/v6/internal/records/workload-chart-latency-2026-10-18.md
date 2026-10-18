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
proof surface. A fresh hosted VM compiles base and candidate API test
executables before timing, records their committed HEAD/tree and binary
hashes, and observes idle capacity before each process. CPU, guest, swap and
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
