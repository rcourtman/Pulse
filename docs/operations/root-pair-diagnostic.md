# Fixed root-route diagnostic

This manual, main-only workflow measures the unresolved hosted root-route
comparison, not release readiness. It never runs package tests, other benchmarks,
or layout interventions, and never changes a release threshold.

The source pair is fixed in `scripts/diagnose-root-pair.py`: baseline
`9168d16a32e948665eba80e2899604933292cdc0` and durable main candidate
`dd388decf5896123b6587b1b16f9aee7c3a747f3`. The candidate has the exact
`1120cce4d631bca5525cafc5b5620251998773f7` tree measured from ephemeral
source commit `4cdf250f474ad9cde6ffb121527f021b7387603f7`; metadata retains that original
evidence identity. Tree and HTTP source hashes must match. Both API test binaries
use Go 1.26.8, readonly modules and identical CI
embed text; compilation finishes before either warm-up. Four available CPUs
are selected and inherited by children, GOMAXPROCS is four. After two 100ms
warm-ups and ten seconds settling, ten rounds alternate AB/BA with one 1s
root-only sample per binary per round. No selective retries are supported.

Artifacts retain source/tree, binary and embed hashes, build logs/toolchain,
root/benchmark disassembly and symbols, sample order/start/end timestamps,
affinity, CPU model, pressure and cgroup throttling observations. Unavailable
telemetry is explicit, not zero. Executables are not published. Partial evidence
survives failure; only complete collection sets `complete=true`. A crash stops
collection and does not authorise crash investigation. The original adverse
result remains evidence even if this comparison finds no gap.

Dispatch requires the exact reviewed current-main workflow SHA and attempt one;
permissions are contents-read only, with no production credentials or writes.
Independent review and protected landing precede any hosted collection. Inspect
the retained attempt before another operation; a successful collection is not a
qualification verdict. No beta.3 scope or installed acceptance changes here.

Focused contract proof (mocked commands, no benchmark):
`python3 -m unittest discover -s scripts/tests -p test_root_pair_diagnostic.py -v`.
It checks the fixed order and selection, build-before-sample sequencing,
partial failure retention, source/toolchain refusal, receipt non-overwrite,
workflow authority and complete artifact hashes.
