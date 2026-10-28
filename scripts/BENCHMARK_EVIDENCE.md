# CI benchmark evidence

`run-ci-benchmarks.sh` writes `bench-metadata.txt` beside the timing files.
The workflow uploads all four files even when the regression gate fails.
Metadata records the candidate and baseline Git HEAD and committed tree IDs,
selected Go versions, platform, GOMAXPROCS setting, sample count, duration,
package list, shard and paired sample start order/timestamps. It deliberately does
not dump environment variables, remote URLs or filesystem paths.

HEAD/tree identify committed source, not a clean-worktree or clean-build
attestation. The workflow adds a frontend embed stub. A PR checkout may be
a synthetic merge rather than the PR head; use the captured identity, not the
run API's head SHA alone, when reproducing it. Non-Git trees and nested archives
report identity unavailable rather than inheriting an enclosing checkout's SHA.

Retain the original failed comparison. Compile both exact trees with the same
Go version before measuring, warm both, alternate their execution order and
collect at least ten samples. Record CPU/load observations and any departures
from CI (affinity, GOMAXPROCS, benchmark filtering or duration). The maintainer
heavy-work wrapper must still be used for builds; it does not prove an idle
host. A focused reproduction is diagnostic, not replacement qualification.

Unchanged source in a benchmark's file does not establish unchanged generated
code, binary layout or runtime behaviour. Matching address-normalised
instructions alone do not prove equivalent timing. Do not waive the gate from
an unrelated diff, a single passing repeat or an unverified noise hypothesis.

Each measured Go test executable is now SHA-256 hashed immediately before
execution through Go's `-exec` wrapper. Metadata `binary=label,round,name,sha256`
records bind each paired ordinal to the executed bytes; unpaired runs use
`unpaired` because Go performs all counts in one invocation. Warmups are not
recorded. Hashing is outside the benchmark timer, but it touches executable
pages and is an instrumentation change, not a controlled comparison with older
runs. Execution arguments, temporary paths and environment contents are not
recorded. A failing executable still fails collection.

These hashes do not retain binaries or establish equivalent instructions,
linked data, host state or performance. They make later reconstructions
checkable; a hash mismatch means they are not the original measured bytes.
Historical failures without hashes remain unresolved evidence.

Paired runs sample only packages whose in-module dependency closure (`go list
-deps -test` in the candidate tree) holds a file whose committed content
differs between the two HEADs. A changed `_test.go` file selects only its own
package, since no other test binary compiles it; any other file selects every
package whose closure holds its directory or a parent of it, which covers
embeds and testdata. Module or workspace files, trees that are not both clean
Git roots, or an unreadable package graph select every package. Metadata `selection=` names the rule applied and
`packages=` the packages sampled; an empty list leaves empty timing files and
nothing to compare. A benchmark reading files outside that closure at run time
escapes selection. Unpaired runs select every package before sharding.

CI now partitions that inventory across three isolated hosted jobs using
`PULSE_BENCH_SHARD=0`, `1` and `2`. The fixed assignment in the collector keeps
monitoring and unified resources apart. Each package's baseline, candidate,
warmups and all ten alternating pairs stay on the **same** runner; samples for
one comparison are never combined across hosts. Each shard still uses the
existing duration, budget and regression rules. With no shard specified,
local collection retains the full inventory. Invalid shards and unassigned
packages fail; an empty affected shard records an explicit empty package list
without invoking Go on the repository root.

The aggregate `Benchmarks` check requires every shard to succeed, with matrix
fail-fast disabled so a failed sibling does not cancel another comparison.
Each job uploads its own `bench-results-0`, `-1` or `-2` artifact even on failure.
Read **all three** artifacts and their source/shard identities before calling
collection complete. Source controls and a disposable fixture do not establish
hosted runtime improvement or clear an older incomplete comparison. Sharding
adds hosted setup work in exchange for a smaller per-job measurement workload;
it does not assert a reduction in total compute or an idle worker.

With `PULSE_BENCH_BUDGET_SECONDS` set, a paired run projects its total,
warm-up included, after each round and exits 1 once the projection reaches the
budget, printing each package's mean seconds per sample. CI keeps the budget
below the step timeout so an outgrown suite fails with that attribution, not a
bare timeout. Partial timing files are uploaded but are not a comparison.

Keep untimed work per iteration in proportion to timed work. Setup between
`b.StopTimer` and `b.StartTimer` is excluded from the estimate Go uses to pick
`b.N`, so a nanosecond operation is repeated hundreds of thousands of times
with all of that setup. `BenchmarkRegistryProjectionSnapshot`'s empty cold cases
took 16 s per sample at a 100 ms benchtime that way and pushed paired runs past
their timeout.

The workload-chart benchmark has three distinct measurement boundaries under
`HandleWorkloadCharts_StoreBacked/range=4h`:

- `warm-handler` reuses a request and a counting response writer. It measures
  cached handler work without retaining another copy of the response body.
  It does not measure HTTP transfer or provide a throughput number.
- `cold-service` creates a fresh router and chart service for every request.
  It includes construction, empty-response-cache computation, store-backed
  reads and serialization. SQLite and OS caches remain warm. It is not a
  cold-disk or process-startup measurement.
- `http-transfer` uses a real loopback HTTP server and fully drains each body.
  It includes client, server and local transport costs. Connection and response
  caches are warmed before timing. Its throughput applies to that loopback
  exchange, not a deployed network.

All three share the same 70-workload fixture and payload sanity checks. The
store fixture waits for queued startup maintenance before seeding and timing.
`TestSLO_WorkloadCharts` also creates a fresh service for each timed sample,
so its existing latency budgets cover chart generation rather than cached
payload copies. It retains the separate sanity request and payload checks.
The warm cases use the production cache lifetime, so a run longer than that
lifetime also includes refreshes. Keep duration and cache policy visible when
interpreting or comparing them.

These names replace the legacy `range=4h` case, whose timed work included a
fresh `httptest.ResponseRecorder` and a payload-sized buffer allocation after
warming the response cache. Their timings are not interchangeable. The first
CI comparison across this test change has no matching chart-case names and
establishes a new baseline. Retain the old failed comparison and use the same
revised harness on both recorded runtime trees for diagnostic continuity.
The original +12.72% signal from #3052 remains historical evidence, not a
validated chart-runtime regression or a claimed speedup from this harness fix.
See the scoped [investigation record](../docs/release-control/v6/internal/records/workload-chart-benchmark-reliability-2026-10-18.md).
