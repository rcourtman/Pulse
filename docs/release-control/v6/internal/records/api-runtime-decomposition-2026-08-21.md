# API Runtime Decomposition Qualification Record

- Date: `2026-08-21`
- Subsystem: `api-contracts`
- Baseline commit: `9dac68fd6`
- Boundary commit: `b70a658c8992158b8d0f8a6408053afa2915a8d7`
- Result: production decomposition and chart/resource PVE acceleration passed

## Production Boundaries

The former single `internal/api` compilation and test unit now delegates to
production packages with one-way dependencies:

- `internal/api/alerting` owns alert lifecycle, notification management,
  queue/DLQ behavior, monitor adapters, and their tests.
- `internal/api/configapi` owns configuration, node lifecycle, discovery,
  setup-script, auto-registration, export/import, and enrollment handlers with
  their tests.
- `internal/api/agenttokens` owns install-token scope, owner binding, metadata,
  issuance, rollback, and persistence policy.
- `internal/api/agentbinding` owns immutable install-token command-channel
  binding policy.
- `internal/api/apicontext` and `internal/api/apihttp` own the shared tenant and
  HTTP scope/error primitives used by extracted domains.

Root retains router composition, cross-domain integration proof, and
source-compatible aliases/delegates. Extracted packages do not import root.
The final ownership audit removed the last duplicate install-token and Proxmox command
implementations from root, leaving the extracted packages canonical.

The root package changed from 633 to 587 Go files and from 4,021 to 3,742
top-level tests. A total of 279 existing tests moved with their production
domains; four focused security-boundary tests were added. No test was skipped,
rerouted, or weakened.

## Measurement Method

Both hosts used uncached test execution after a compile-only warm-up and asset
verification:

```sh
bash scripts/ensure_test_assets.sh
go test -run '^$' ./internal/api/...
/usr/bin/time -lp sh -c 'go test -json -count=1 ./internal/api/... > api-timing.json'
# GNU time on PVE:
/usr/bin/time -v -o api-timing.time go test -json -count=1 ./internal/api/... > api-timing.json
```

The local host was Darwin arm64, Go 1.26.7, with 10 logical CPUs. The PVE host
was Linux amd64, Go 1.26.5, with 8 vCPUs. PVE received committed git bundles in
`/tmp/pulse-api-decomposition.kZB9Hb`; tests ran from a detached temporary
clone. The primary `/srv/pulse/pulse` checkout was not modified.

## Timing Evidence

| Host and revision | Wall | User | System | Average cores | Capacity used | Maximum RSS | Result |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Local monolith `9dac68fd6` | 241.64s | 237.74s | 62.05s | 1.241 | 12.41% of 10 | 2,725,707,776 B | 7,301 pass events |
| Local boundaries `b70a658c8` | 153.96s | 177.78s | 43.18s | 1.435 | 14.35% of 10 | 3,097,395,200 B | 7,305 pass events |
| PVE monolith `9dac68fd6`, sample 1 | 251.14s | 259.90s | 40.71s | 1.197 | 14.96% of 8 | 4,480,836 KiB | pass |
| PVE monolith `9dac68fd6`, reverse-order repeat | 258.87s | 261.38s | 44.30s | 1.181 | 14.76% of 8 | 4,377,360 KiB | pass |
| PVE boundaries `b70a658c8` | 268.57s | 272.21s | 45.12s | 1.181 | 14.76% of 8 | 4,473,448 KiB | 7,305 pass events |

The local boundary run was 87.68 seconds (36.28%) faster than its monolithic
baseline. PVE did not reproduce that gain: the final sample was 9.70 to 17.43
seconds slower than the two monolithic samples. This is a qualification miss,
not an accepted variance waiver.

PVE package timing explains the result:

| Package | Concurrent full run | Isolated control |
| --- | ---: | ---: |
| `internal/api` | 267.067s | 249.38s wall (`245.345s` package at `25f20f6ee`) |
| `internal/api/configapi` | 36.820s | not required |
| `internal/api/alerting` | 0.261s | not required |
| `internal/api/agentbinding` | 0.013s | not required |
| `internal/api/agenttokens` | 0.013s | not required |

The scheduler overlapped the complete extracted package workloads, but the
root test binary slowed by about 18 seconds under concurrent filesystem and CPU
pressure. That contention consumes the current architectural gain on this
worker.

## Qualification Evidence

- `go test -json -count=1 ./internal/api/...`: 7,305 pass events, zero test or
  package failures, 153.96 seconds locally at `b70a658c8`.
- `go test -race -timeout 20m -count=1 ./internal/api/...`: passed every
  package with no race report or panic at `b70a658c8`; 811.93 seconds wall,
  872.47 seconds user, 144.59 seconds system, and 3,382,788,096 bytes maximum
  RSS. Package elapsed times were 806.736 seconds for root and 107.852 seconds
  for `configapi`.
- Release-control registry, contract, status, and control-plane audits passed
  with zero errors and zero warnings after this record was added.

## Remaining Critical-Path Coupling

The residual root imports more than 70 internal runtime packages. Its PVE
critical path is dominated by chart/mock projection and high-scale
resource/router integration tests, including:

- `TestContract_MockChartRoutesUseCanonicalMockUnifiedReadStateForVMwareHosts`
- `TestHandleCharts_UsesCanonicalMockUnifiedReadStateForVMwareHosts`
- `TestLoad_500Node_ConcurrentResources`
- `TestLoad_500Node_MixedEndpoints`

At that boundary revision, chart handlers and response builders still spanned
`internal/api/router.go` and `internal/api/types.go`; the resource query path
still shared one `ResourceHandlers` type with action lifecycle and
operator-state mutation.
Moving only the expensive tests, copying list filters into a new package, or
wrapping root behavior in callbacks would create an artificial boundary and is
not acceptable. The next decomposition must separate the production chart and
resource-query services, give each a lower-level monitor/registry interface,
move their full contract/load tests with them, and leave root with route
composition and cross-domain proof only.

## Chart and Resource Query Continuation

The continuation was mapped from `418402bf9eeb4ddb5a2f437ac9b93290bc96f103`,
then rebased onto the corrected release selector at
`1327dddad5200f07271e16abdf4dd83fa1f2eb4f`. Implementation commit
`2e4bd36ec5fd0121a2a4cc3564be89eeb9d5c750` completed the two production
boundaries identified above:

- `internal/api/chartapi` owns all six chart handlers, monitor history queries,
  aggregation/downsampling, response contracts, per-tenant response caches,
  singleflight coordination, and the chart handler/contract suite.
- `internal/api/resourceapi` owns registry construction, tenant stores,
  unified seed ingestion, list/detail/facet/timeline queries, discovery and
  metrics projections, response contracts, and the 500-node resource load
  proof.
- Root Router and `ResourceHandlers` retain source-compatible delegates and
  type aliases. Root continues to own route authorization, action lifecycle,
  operator-state mutation, and cross-domain integration proof. Neither domain
  package imports root or delegates behavior back through generic callbacks.

The root package changed from 587 to 581 Go files and from 3,755 to 3,645
top-level tests. Exactly 110 existing test/benchmark entry points moved with
their production domains: 35 to `chartapi` and 75 to `resourceapi`. The moved
resource set includes `TestLoad_500Node_ConcurrentResources`; the moved chart
set includes 17 end-to-end handler tests that previously ran serially in the
root test binary.

The exact local warm correctness run used the same command form as the earlier
record:

```sh
/usr/bin/time -lp sh -c 'go test -count=1 ./internal/api/...'
```

It passed in `118.77s` wall, `177.00s` user, and `30.73s` system with
`3,455,729,664` bytes maximum RSS. Average CPU occupancy was 1.749 cores
(`17.49%` of the 10-logical-CPU local host). Concurrent package elapsed times
were `114.725s` root, `96.313s` config, `22.665s` chart, and `8.971s`
resource. The prior recorded local boundary run was `153.96s`; because the
continuation baseline also includes later mainline release fixes, that
`35.19s` difference is supporting evidence rather than the controlled PVE
before/after result.

Full race qualification passed with no detector report or panic:

```sh
/usr/bin/time -lp sh -c 'go test -race -timeout 20m -count=1 ./internal/api/...'
```

The race run used `667.59s` wall, `885.43s` user, `105.78s` system, and
`3,728,080,896` bytes maximum RSS. Package elapsed times were `649.623s` root,
`123.379s` chart, `104.904s` config, and `14.749s` resource.

A static equivalence audit extracted every top-level `Test`, `Benchmark`, and
`Example` function name from the continuation baseline and the completed tree.
Both sets contain 4,038 unique names; the set difference is zero in both
directions. This complements executable contract qualification by proving the
package move did not rename, delete, or replace any existing test entry point.

### Controlled PVE release qualification

The final comparison ran on `pulse-dev`, Linux amd64 with Go 1.26.5 and 8
vCPUs. Exact committed bundles were checked out detached beneath
`/tmp/pulse-api-chart-resource.vxQAJx`; no primary checkout or runner service
was modified. Both measured revisions used warmed race build and test caches,
a fresh isolated data-root value, and the same fixed two-shard release plan:

```sh
/usr/bin/time -v -o baseline-steady.time \
  ./scripts/run-release-backend-tests.sh \
  --data-root /tmp/pulse-api-chart-resource.vxQAJx/data-baseline-steady \
  --api-shards 2

/usr/bin/time -v -o after.time \
  ./scripts/run-release-backend-tests.sh \
  --data-root /tmp/pulse-api-chart-resource.vxQAJx/data-after-fresh \
  --api-shards 2
```

The explicit shard count removes a discovered environmental confound: the
first baseline saw 11,116 MiB available and auto-selected two shards, while an
immediate split attempt saw 9,591 MiB and auto-selected one. That one-shard
attempt was terminated and excluded. A subsequent matched attempt was also
excluded after its reused data-root exposed an asymmetric `resourceapi` test
cache hit. The recorded pair is steady-state warm on both sides, used distinct
fresh data-root values, saw 10,392 and 10,377 MiB available respectively, and
produced the same two shards with one exact-name batch per shard.

| Warm release gate | Wall | User | System | Average cores | 8-vCPU capacity | Maximum RSS | Result |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Baseline `1327dddad` | 13:46.65 | 1,545.57s | 148.79s | 2.050 | 25.62% | 6,381,264 KiB | pass |
| Chart/resource split `2e4bd36ec` | 11:16.05 | 1,369.46s | 138.27s | 2.230 | 27.88% | 6,526,072 KiB | pass |

The production split reduced the warm release wall time by 150.60 seconds
(18.22%) and total CPU time by 186.63 seconds (11.01%). Average occupancy rose
from 2.050 to 2.230 cores, or 2.26 percentage points of worker capacity. Peak
RSS increased by 144,808 KiB (2.27%). Both release runs completed with two API
`PASS` results, no race report, and the terminal
`Race-enabled backend release tests passed with 2 API shard(s).` marker.

The split release log reports native warm package results for both
`internal/api/chartapi` and `internal/api/resourceapi`; the uncached local race
run above remains the execution proof for every moved test. Live checkpoints
also showed the shorter root shard finishing near 5m13s after the split versus
about 7m in the steady baseline. The remaining root shard therefore remains
the release critical path even though the architectural cut materially reduced
it.

Preserved local evidence files and checksums:

- `/tmp/pulse-api-next-cut.Cxp7CT/baseline-steady.time`:
  `d6673f5a0f06a076e8fc661695b813129a831c5900eaf6d0bd74a122b6363f11`
- `/tmp/pulse-api-next-cut.Cxp7CT/baseline-steady.log`:
  `1e83269680567e1d92e361cfde26414e6bc6a815f44710a0353b94dfaeac3784`
- `/tmp/pulse-api-next-cut.Cxp7CT/after.time`:
  `c9570f253460e360432bacd9803b23f703a73baa93929d6cfce06c65dfce4f71`
- `/tmp/pulse-api-next-cut.Cxp7CT/after.log`:
  `5a8dfa99f184514467669990dd667e22507a04602809d2a1d48895c897964c00`

After the measurements, the worker had no matching release-backend, API race,
or Docker build process. The temporary remote files and logs were preserved.
