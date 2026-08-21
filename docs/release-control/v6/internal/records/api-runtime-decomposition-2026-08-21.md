# API Runtime Decomposition Qualification Record

- Date: `2026-08-21`
- Subsystem: `api-contracts`
- Baseline commit: `9dac68fd6`
- Boundary commit: `b70a658c8992158b8d0f8a6408053afa2915a8d7`
- Result: production decomposition passed; PVE acceleration remains open

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

The chart handlers and response builders currently span `internal/api/router.go`
and `internal/api/types.go`; the resource query path shares one
`ResourceHandlers` type with action lifecycle and operator-state mutation.
Moving only the expensive tests, copying list filters into a new package, or
wrapping root behavior in callbacks would create an artificial boundary and is
not acceptable. The next decomposition must separate the production chart and
resource-query services, give each a lower-level monitor/registry interface,
move their full contract/load tests with them, and leave root with route
composition and cross-domain proof only.
