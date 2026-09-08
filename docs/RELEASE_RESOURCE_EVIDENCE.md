# Backend preflight resource evidence

`release-preflight-worker.sh` emits one `RELEASE_RESOURCE_SNAPSHOT` JSON line
before and after the backend phase, plus `RELEASE_BACKEND_TOOLCHAIN` and
`RELEASE_BACKEND_EXIT`. The exact-preflight launcher's retained log preserves
these lines even when it removes its disposable worker directory. No success
receipt is produced by this evidence collector. Backend exit status, parallelism and assertion thresholds are unchanged.

The collector reads only allowlisted kernel resource counters and numeric Go
runtime settings. It does not dump the environment, process command lines,
credentials or application data. Cgroups are labelled by ancestor distance,
not service/session names. An unmapped cgroup mount, missing file or collector
failure is explicitly unavailable evidence, never a zero counter. The standard
Linux cgroup2 root mount is required for hierarchy collection; other mappings
are deliberately not guessed.

Compare cumulative counters between boundaries, checking counter availability
and for resets first. Parent counters include other work; host pressure and load
are not exclusive to the worker. Boundary snapshots cannot localise contention
to a test, detect all transient pressure, prove causation or prove exclusivity.
They do not record changes between boundaries. Abrupt termination may leave
only the before record. This instrumentation is not a repair of a product
latency failure and cannot clear prior adverse qualification evidence.

Rationale: failed rehearsal 20260907T131615Z on
940f788dc29162d110fa7b896f5668262d8e4e7c retained a p95 latency miss but no
contemporaneous resource counters. The subsequent fixed twelve-sample isolated
comparison passed without measured throttling, but omitted preceding full-suite
workload and did not establish the cause of the original miss. Instrument future
otherwise-justified qualification rather than replaying to obtain a pass.

The [kernel cgroup v2 documentation](https://docs.kernel.org/admin-guide/cgroup-v2.html)
defines ancestor restrictions, `cpu.stat` counters and pressure interfaces.
These interfaces supply context, not release readiness. No excluded crash or
database investigation is part of this collection.

Focused checks:

```sh
python3 -m unittest discover -s scripts/release_control/internal -p 'release_resource_snapshot_test.py' -v
python3 -m unittest discover -s scripts/release_control/internal -p 'release_preflight_test.py' -v
bash -n scripts/release-preflight-worker.sh
```

## Stress-test event window

Rehearsal backend Go output is streamed with `-json` and decoded by
`release-go-test-events.py`. Every Output string is retained verbatim (including
verbose test logs, skips and package summaries); stderr remains on stderr.
Shell pipefail retains a failing Go exit. Invalid event input is drained,
retained and fails the reader rather than quietly losing diagnostics.

Only the exact API `TestMultiTenant_ConcurrentAPIStress` lifecycle receives
`RELEASE_GO_TEST_EVENT` records with Go's event Time, a separately labelled
receipt timestamp and the same allowlisted resource snapshot. Resource time is
collection time, not the Go event time. Sampling and verbose streaming add
measurement overhead; they do not reproduce uninstrumented execution. Cached
results can replay events and are not fresh execution timing: check the package
summary for `(cached)`. No cache, scheduling, threshold or test-selection policy
is changed. Release-profile sharded tests are unchanged.

This closes a prospective observability gap, not the historical qualification
failure. Use only with an independently justified future run; no qualification
retry is warranted merely to collect these records. Abrupt termination can omit
terminal events. Cgroup ancestry remains shared context, not per-test CPU usage.

```sh
python3 -m unittest discover -s scripts/release_control/internal -p 'release_go_test_events_test.py' -v
```
