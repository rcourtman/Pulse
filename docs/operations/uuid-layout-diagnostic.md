# Fixed UUID layout diagnostic — not release qualification

This main-only workflow collects evidence for the failed PR1943 UUID comparison
in [run 34055039318](https://github.com/rcourtman/Pulse/actions/runs/34055039318).
It does not change the benchmark gate, its threshold, product declaration order,
release checks, or the candidate. Green here means collection completed, **not**
that the candidate passed its failed check.

## Exact experiment

- Baseline `5845692daf302a1524c2ca8fe6d06339ae7f8a24`, tree
  `dedef52efe4fc28d1c05c9d957a0cc439f54db22`.
- Candidate `57f3b6401a6553efaa7646d5f1db446a2725b9dc`, tree
  `7c2765cb06f424ce49647b33a5ebf3ade375e828`, identical to the failed
  synthetic PR merge `ad6869a29cd0b54c918967d3cf41320914ba5790`.
- Ubuntu 24.04 hosted runner, Go **1.26.7** linux/amd64, no toolchain
  auto-upgrade or restored build cache, GOMAXPROCS/test.cpu=4.
- Original baseline/candidate and the same two trees with `normalizeSegment`
  moved immediately before `normalizeRoute`, without changing its body. Both
  HTTP source and benchmark files must match their recorded SHA256 first.
- Four full API test-package compilations before measurement, identical frontend
  embed stubs. No unit tests run. Only
  `^BenchmarkNormalizeSegment$/^uuid$` executes, including warm-up.
- Ten rounds, forward/reverse condition order alternating, 300ms per sample,
  one invocation per condition per round. No retries, early statistical stopping,
  broad benchmark selection, or automatic verdict. A failed invocation stops
  collection while preserving partial evidence.

This reproduces the *design* of Core's retained 6 September controlled experiment,
not its local measurements. Hosted CPU and layout effects are still hypotheses.
Contemporaneous paired/interleaved samples reduce time-drift bias; they do not
eliminate shared-host interference. See [Go performance monitoring guidance](https://go.dev/wiki/PerformanceMonitoring).

## Execution authority: unresolved dependency

Source integration alone does not authorise Delivery to dispatch this workflow.
Delivery's standing dispatch permission covers **only Pro license/relay**;
starting the release assessment service is not permission to run arbitrary
Actions. Do not use either mechanism to smuggle this experiment into execution.

After ordinary source review and main integration, an operator with repository
Actions dispatch authority must explicitly approve and perform **one** execution
of `uuid-layout-diagnostic.yml` on main, supplying its full reviewed main commit
as `expected_workflow_sha`. Alternatively the operator must explicitly grant a
bounded execution route. No such grant is supplied by this document. The workflow
rejects branch/SHA mismatch and Actions reruns (`run_attempt != 1`); a new manual
dispatch is not automatically a justified retry. Record the approval and dispatch
run identity internally. Before any further attempt, reconcile the first run and
its partial artifacts and obtain a reasoned diagnostic decision, not a green-run
search. A new source pair or toolchain requires source review, not free-form inputs.

## Evidence and disposition

Download the one `uuid-layout-diagnostic-<run>-<attempt>` artifact. Require
`metadata.json` to say `complete: true`, verify `SHA256SUMS`, and verify all four
conditions have ten UUID-only samples. Retain raw per-invocation samples,
aggregates, chronological load/order records, exact source/tree identities,
workflow SHA/run/attempt, Go environment, CPU/kernel observations, binary hashes,
HTTP-only symbols/disassembly, and the four HTTP source files showing the
intervention. Experimental executables and source archives are temporary and
are never uploaded, packaged, tagged, installed, or proposed as candidates.
Partial artifacts cannot establish a completed experiment.

Reuse the original hosted artifact (archive SHA256
`61acc9f4e14fc0672f1eacaa81481e2ec497c71d302fe2bca30bc598c2a5354d`)
and Core's already-retained controlled results; do not overwrite or substitute
them. Compare original and reordered pairs separately with a recorded benchstat
version, retaining all results including adverse ones. The historical result is
+21.04%, ten samples; a smaller later number alone is not causal disposition.

The release qualification owner must then explain whether the intervention
establishes a defensible disposition of the unchanged candidate or requires a
reviewed repair. Existing adverse evidence and the HOLD remain until that
judgment. This diagnostic neither investigates nor clears the separately excluded
SQLite work. No release assessment is warranted merely because this workflow
exists or completes.

## Focused harness checks

`python3 scripts/tests/test_uuid_layout_diagnostic.py` uses fake subprocesses to
check fixed scope, ordering, source/toolchain rejection, partial evidence,
non-overwrite behaviour and receipt completeness without executing benchmarks.
For any real local build/benchmark execution use `pulse-heavy-run -- <command>`;
a further local sample is not the missing hosted evidence.
