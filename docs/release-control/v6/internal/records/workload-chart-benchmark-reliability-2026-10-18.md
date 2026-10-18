# Workload chart benchmark reliability, 18 October 2026

Scope: the optional chart benchmark failure in #3052 and the measurement
contract for `BenchmarkHandleWorkloadCharts_StoreBacked`. The owning residual
remains `performance-post-rc-headroom` in L10. This investigation does not close
that residual or change release qualification.

## Original evidence

[Build and Test 39135315179, Benchmarks job 117698772365](https://github.com/rcourtman/Pulse/actions/runs/39135315179/job/117698772365)
failed the significant-regression threshold for
`HandleWorkloadCharts_StoreBacked/range=4h-4`:

| Measurement | Baseline | Candidate | Comparison |
| --- | --- | --- | --- |
| Time per operation | 333.0 µs ±16% | 375.3 µs ±13% | +12.72%, p=0.043, n=10 |
| Bytes per operation | 1.577 MiB | 1.577 MiB | unchanged |
| Allocations per operation | 25.50 | 25.50 | unchanged |

The failure threshold was greater than 10% with p less than 0.05. Collection
used alternating paired 100 ms samples with Go 1.26.9 on Linux amd64, an AMD
EPYC 7763 runner and the runtime-default GOMAXPROCS setting (benchmark suffix
`-4`). These identities come from the retained run metadata, including the
synthetic merge used by the PR checkout:

| Identity | Baseline | Candidate |
| --- | --- | --- |
| Commit | `98817ebce4e02545ca8870519ae929cee9c724c8` | `2ff11395ed9a08cbd4c38134d3645330aaade109` |
| Committed tree | `6599a0d8b8296f1ca91ddcf156ba1445e48a1d82` | `3d7d273e14f4d7c045cd35a16d2095c6da8eafa8` |
| Executable SHA-256 | `978566737d0a53c6a64fe42fdf482ed694483a1fbc60fbf869837f912bfa6d36` | `e3562eda32a9662df28a5e5cc6725dcab13b775e7b02a3c182d96a3face67868` |

The candidate tree equals the PR head and landed merge tree. Original
executables were not retained. A prior 20-sample focused reconstruction on
`pulse-dev` gave the opposite direction, 692.6 µs versus 575.0 µs
(-16.97%, p=0.020), with different executable hashes and a loaded host.
That diagnostic did not reproduce or clear the original result. An unchanged
benchmark body also does not prove equivalent executable timing.

## Confirmed benchmark defect and correction

The fixture seeds 84,000 points for 70 workloads, then makes a sanity request
that fills the short-lived response cache. The legacy timed loop mostly served
that cached response into a fresh `httptest.ResponseRecorder`. It allocated a
second copy of roughly 1.6 MiB on each iteration and did not reliably exercise
the store-backed query it claimed to protect.

A 5,000-iteration profile of the reconstructed candidate measured 363,878 ns/op,
1,653,504 B/op and 25 allocs/op. `bytes.growSlice`, reached through
`ResponseRecorder.Write`, accounted for 7,874.91 MiB of 8,022.89 MiB allocated
(98.16%). The CPU profile also contains untimed fixture seeding, so its global
SQLite and GC percentages are not handler-only attribution.

The benchmark now separates three boundaries:

- `warm-handler` reuses a request and a counting writer without retaining the
  response body. It measures handler work under the production cache policy.
- `cold-service` constructs a fresh router and chart service per request. It
  includes construction, store-backed reads and serialization with an empty
  response cache. SQLite and OS caches are not cleared.
- `http-transfer` warms a loopback connection, then reads and closes every full
  response. It includes client, server and loopback transport costs.

The shared metrics-store fixture also waits for queued startup maintenance,
matching the existing SLO harness boundary. All cases retain the seeded
payload sanity checks. The adjacent `TestSLO_WorkloadCharts` had the same
cache-warming blind spot. It now constructs a fresh router and chart service
for each sample, retaining its existing 90 ms local and 550 ms hosted budgets.
No runtime endpoint, cache policy, query, schema,
retention or rendered behavior changed. The runtime already writes the
immutable cached payload directly, so adding another cache or changing its
semantics would not address the confirmed recorder allocation.

## Final verification

A diagnostic applied the same revised router benchmark to both exact recorded
runtime trees and compiled fixed executables with Go 1.26.9. It warmed both,
then alternated execution order for 20 paired 100 ms samples with `-cpu=4`
and no affinity. It used Linux amd64 on the shared 8-CPU Ryzen 5 3600 worker.
Initial load was 2.92/3.55/3.85 with 16% guest CPU in the final initial vmstat
sample. At the last pair, load was 5.18/5.11/4.54 with 13% guest CPU. This was
not an idle-host proof.

| Case | Baseline | Candidate | Comparison |
| --- | --- | --- | --- |
| warm-handler | 1.348 µs ±6% | 1.331 µs ±8% | no significant change, p=0.779 |
| cold-service | 153.7 ms ±6% | 152.2 ms ±10% | no significant change, p=0.925 |
| http-transfer | 606.0 µs ±11% | 619.8 µs ±9% | no significant change, p=0.883 |

The comparison's router harness SHA-256 was
`a7ce0422aae85d5a9fc5076d987bd548cf922ddb300388bc748fc1c0efaa6496`.
Its fixed executables were:

- Baseline: `6afeb3965503d218f83c12581545acf1c51a0a291c93d6eff9ff0a260e2e23a1`.
- Candidate: `c8905f56c8ae80400a619a7318f56b21ea883d3ebc62be50a9e9f0c090d800e6`.

That comparison predates the final test-client timeout correction, actual
transfer-byte accounting and SLO sample correction. Its source overlays and executable identities remain
explicit. It is a diagnostic of the revised boundaries, not qualification
of the final test sources. The race smoke first stopped on a missing ignored
CI embed fixture. With that supplied, cold generation under race instrumentation
took 5.96 seconds and the HTTP warmup exceeded the fixture's five-second client
timeout. The test client now has a bounded 30-second allowance. This changes
neither a production timeout nor the SLO budgets.

The corrected SLO ran on the shared worker with its final test source hash
`aecd6d939d1b2f2b767d28f6b9ae49917a58dbc748141fba4fa62886eda8bac5`.
It measured p50 175.081 ms, p95 215.634 ms and p99 234.951 ms. All requests
completed, but the existing shared-host policy skipped the 90 ms local budget
assertion. This is not a strict latency pass. Initial load was 2.40/3.09/4.04,
and final load was 2.64/3.09/3.89. The non-race Release Dry Run enforces the
unchanged 550 ms hosted envelope. Build and Test uses race-enabled API shards,
which intentionally skip latency SLOs. Required source checks therefore do not
establish a strict latency pass. This source-only investigation does not run
or claim release qualification.

The next race smoke passed all three cases. A final throughput-accounting edit
overlapped its initial metadata capture, so that run is retained as diagnostic
evidence only. The final race proof uses frozen sources, a retained executable,
and source hash checks before compilation and after execution.

The frozen-source race smoke passed all three cases. Both source hashes
matched before and after execution. The router benchmark hash is
`7b3dd99ffb24c621252193545b024edb3dd70a93c8c056b5da644784d6112ddd`,
and the SLO hash is the one recorded above. The retained race executable has
SHA-256 `b960e7386b84074ed1dc97188681359b1d0d684577af914c6d911a998c0d7800`.
Collection ran from 16:16:45 to 16:17:50 UTC. Host load rose from
8.75/5.95/4.46 to 11.55/7.14/4.96, with 18% guest CPU in the final initial
vmstat sample. Race timings are not production latency evidence.

## Interpretation and retained evidence

The revised names intentionally start new benchmark series. A PR comparison
against the old harness has no matching chart names and cannot establish a
chart improvement. The controlled overlay comparison uses the same revised
harness on both recorded runtime trees. It measures those boundaries with the
new harness and does not recreate the original executable bytes or runner.

The original +12.72% remains an unresolved historical benchmark observation.
The confirmed measurement and coverage defects are corrected. There is no
validated chart-runtime regression or product speedup from this investigation.
L10 remains partial with a bounded residual under the existing follow-up.

Worker evidence is retained under
`/opt/pulse-release-worker/task-proofs/chart-benchmark-investigation-20261018`:
`profile-metadata.txt`, `profile.log`, CPU and allocation profiles and summaries,
`paired/` for the initial revised-harness diagnostic, `paired-final/` for the
identical-harness comparison, `race.log` for the retained fixture failure,
and `race-final.log`, `slo-local.log` and `final-checks-metadata.txt` for the
SLO and diagnostic race checks. `bound-source.sha256`, `race-bound.log`,
`race-bound-metadata.txt` and `race-final.test` bind the final race proof to its
source and executable bytes. The original reconstructed
executables and first focused repeat remain under the previous notification
investigation's `chart-benchmark/` evidence directory. Each reconstructed
executable has its own hash. None is represented as the original CI binary.
