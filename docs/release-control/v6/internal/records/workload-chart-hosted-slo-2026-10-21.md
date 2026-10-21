# Hosted workload chart SLO verdict, 21 October 2026

Scope: the optional hosted `Workload chart 90ms SLO` job in Build and Test,
introduced on 18 October by `2a77054035` (see
[workload-chart-latency-2026-10-18.md](workload-chart-latency-2026-10-18.md)).
The 90 ms p95 budget, the idle admission rules, and the capacity guard are
unchanged. This record changes only how a capacity-invalid hosted run is
reported.

## Finding: the hosted runner is never idle during the measurement

From its introduction to this record, the job ran on 21 pull-request runs and
failed all 21. It is skipped on `main`. Twenty runs still have their
`chart-latency` artifact. The first run, `117757206920`, used the earlier
baseline-plus-candidate layout, and its findings are already in the 18 October
record. In every one of the twenty, the proof stopped on
`capacity_errors`, and the competing process was the hosted runner's own
`provjobd<N>` daemon:

- The admission window was quiet every time: six to thirteen 5 s rows with at
  least 92.8% idle CPU and a 30 s quiet streak. Every after-sample row was at
  most 1.7% busy.
- 3 to 10 s after the measured process started, `provjobd` took one full vCPU
  of the four for two to four 1 s intervals (24.8–25.4% of total CPU). It is
  inside the 200-iteration timed loop, not the fixture seed, so the guard
  correctly voids the measurement.
- No other process ever crossed the 10% background threshold. Steal, guest
  and swap were zero throughout.

The burst is triggered by the measured workload itself and never appears in
admission, so waiting longer for idle capacity cannot avoid it. A hosted
runner cannot satisfy this proof as currently defined. Excluding the daemon
would waive real contention for a shared core, which the 18 October records
explicitly refuse to do.

| Started (UTC) | Job | CPU | p95 ms | Invalid intervals | `provjobd` peak |
| --- | --- | --- | --- | --- | --- |
| 2026-10-18 19:34 | [117769795027](https://github.com/rcourtman/Pulse/actions/runs/39159417029/job/117769795027) | AMD EPYC 9V74 | 130.0 | 3 of 25 | 25.1% |
| 2026-10-18 19:57 | [117774534298](https://github.com/rcourtman/Pulse/actions/runs/39161106967/job/117774534298) | AMD EPYC 9V74 | 126.4 | 2 of 24 | 25.1% |
| 2026-10-18 20:43 | [117784843299](https://github.com/rcourtman/Pulse/actions/runs/39164581154/job/117784843299) | AMD EPYC 7763 | 143.7 | 3 of 26 | 25.4% |
| 2026-10-18 20:51 | [117787029851](https://github.com/rcourtman/Pulse/actions/runs/39165194779/job/117787029851) | AMD EPYC 9V45 | 97.4 | 2 of 18 | 25.1% |
| 2026-10-18 22:02 | [117802471451](https://github.com/rcourtman/Pulse/actions/runs/39170505357/job/117802471451) | AMD EPYC 9V45 | 95.1 | 2 of 17 | 24.9% |
| 2026-10-18 22:34 | [117809461554](https://github.com/rcourtman/Pulse/actions/runs/39172806077/job/117809461554) | AMD EPYC 7763 | 152.2 | 3 of 27 | 25.4% |
| 2026-10-18 22:56 | [117814030496](https://github.com/rcourtman/Pulse/actions/runs/39174382045/job/117814030496) | AMD EPYC 7763 | 119.9 | 4 of 20 | 25.3% |
| 2026-10-18 23:36 | [117822460240](https://github.com/rcourtman/Pulse/actions/runs/39177248750/job/117822460240) | AMD EPYC 9V45 | 67.7 | 2 of 12 | 25.2% |
| 2026-10-19 00:16 | [117830754387](https://github.com/rcourtman/Pulse/actions/runs/39180037366/job/117830754387) | AMD EPYC 7763 | 119.0 | 4 of 21 | 25.3% |
| 2026-10-19 08:43 | [117968715873](https://github.com/rcourtman/Pulse/actions/runs/39224836563/job/117968715873) | AMD EPYC 9V45 | 59.7 | 2 of 12 | 25.1% |
| 2026-10-20 03:14 | [118434065104](https://github.com/rcourtman/Pulse/actions/runs/39363199566/job/118434065104) | Intel Xeon Platinum 8370C | 124.0 | 3 of 20 | 25.2% |
| 2026-10-20 03:26 | [118437583129](https://github.com/rcourtman/Pulse/actions/runs/39364305201/job/118437583129) | AMD EPYC 9V74 | 77.7 | 3 of 15 | 24.8% |
| 2026-10-20 23:04 | [118937106686](https://github.com/rcourtman/Pulse/actions/runs/39512566526/job/118937106686) | AMD EPYC 7763 | 109.7 | 3 of 18 | 25.4% |
| 2026-10-21 08:18 | [119116030107](https://github.com/rcourtman/Pulse/actions/runs/39568279139/job/119116030107) | AMD EPYC 7763 | 110.0 | 4 of 19 | 25.2% |
| 2026-10-21 09:23 | [119143585395](https://github.com/rcourtman/Pulse/actions/runs/39576548723/job/119143585395) | Intel Xeon Platinum 8370C | 122.5 | 3 of 20 | 25.2% |
| 2026-10-21 09:33 | [119148498987](https://github.com/rcourtman/Pulse/actions/runs/39577907445/job/119148498987) | AMD EPYC 9V45 | 67.2 | 2 of 12 | 25.1% |
| 2026-10-21 13:16 | [119244588986](https://github.com/rcourtman/Pulse/actions/runs/39606600658/job/119244588986) | AMD EPYC 7763 | 106.5 | 4 of 18 | 25.1% |
| 2026-10-21 14:06 | [119273108926](https://github.com/rcourtman/Pulse/actions/runs/39613949889/job/119273108926) | AMD EPYC 7763 | 107.8 | 3 of 19 | 25.4% |
| 2026-10-21 14:19 | [119294654326](https://github.com/rcourtman/Pulse/actions/runs/39615861460/job/119294654326) | AMD EPYC 7763 | 107.7 | 3 of 18 | 25.2% |
| 2026-10-21 14:43 | [119289111050](https://github.com/rcourtman/Pulse/actions/runs/39619418966/job/119289111050) | AMD EPYC 7763 | 109.7 | 3 of 19 | 25.4% |

## Not a regression

None of these percentiles is a verdict. Even read as hosted observations,
they show no regression after 18 October. On AMD EPYC 7763, p95 fell from
143.7–152.2 ms before the 18 October storage and projection fixes to
106.5–110.0 ms since 20 October. On EPYC 9V45 it fell from 95.1–97.4 ms to
59.7–67.7 ms. Runs on the same day differ by up to 1.8×
across CPU models, so the hosted absolute number is mostly a property of
the hardware. The controlled measurement remains the two-vCPU disposable
worker. Its 18 October strict run passed at p95 82.450475 ms with an empty
`capacity_errors` (see
[workload-chart-idle-proof-2026-10-18.md](workload-chart-idle-proof-2026-10-18.md)).
Relative regressions remain the job of the paired Benchmarks job, which
compares baseline and candidate on the same runner.

## Change

`internal/api/testdata/chart_latency_proof.py` now records a `verdict`
(`valid` or `inconclusive`) in `candidate.result.json`. When the capacity
guard trips, contention voids only the latency verdict. The proof still
requires exactly one executed test with a single 90 ms report, ending as a
PASS or as a FAIL whose only test diagnostic is the budget violation. A
crash, a skip, a broken fixture, any other test error (such as a TempDir
cleanup failure) or a changed executable still fails. A capacity-invalid run writes the
percentiles, the number of invalid intervals and the competing processes to
the job summary, then exits 78.

In Build and Test, the evidence job is now named `Workload chart SLO
evidence`. It treats exit 78 as success and still fails on a valid budget
failure or a harness error. A separate `Publish chart SLO verdict` job holds
the only `checks: write` token and runs no PR code. It publishes the
`Workload chart 90ms SLO` check run as `success`, `failure` or `neutral`.
It publishes `failure` when the evidence job failed before the proof ran,
for example on a guard test or compile failure. Every verdict is also
written to the evidence job summary. Dependabot's read-only token cannot
create the check, so that job then warns and the summary still carries
the verdict. All twenty retained
artifacts above replay through the new classifier as `inconclusive`, each
caused by `provjobd`, with none raised as a harness error.
