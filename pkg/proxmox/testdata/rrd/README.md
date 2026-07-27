# Recorded Proxmox RRD API fixtures

Real `rrddata` responses recorded from Proxmox VE hosts, wrapped in the
`{"data": [...]}` envelope exactly as the HTTP API returns them. They exist so
that the decode tests in `issue1634_rrd_fixture_test.go` validate our parsing
structs against what PVE actually sends, not against hand-written mocks
(issue #1634 happened because every test mocked `memused`/`memavailable`
columns that real guest responses never contain).

## PVE 9 fixtures (recorded live)

Captured 2026-07-27 from a live two-node cluster running pve-manager 9.2.3
via `pvesh get ... --output-format json` on node `minipc`, then trimmed to a
dozen rows while preserving the full column set of the original response.

- `pve9_lxc_rrddata.json` — `/nodes/minipc/lxc/108/rrddata --timeframe hour --cf AVERAGE`
  (running container). Note memory is reported as cache-inclusive `mem`/`maxmem`;
  there is no `memused`/`memavailable` in guest RRD.
- `pve9_qemu_rrddata.json` — `/nodes/minipc/qemu/100/rrddata --timeframe year --cf AVERAGE`.
  The only QEMU VM on the cluster was stopped, so the hour window carried no
  memory samples; the year window includes rows from when it last ran. The
  column schema (`pve-vm-9.0`) is identical across timeframes. Two sparse rows
  (stopped periods) are kept deliberately: PVE omits keys for absent samples
  rather than sending nulls. `memhost` is the PVE 9 QEMU host-side memory
  column; it also is not `memused`/`memavailable`.
- `pve9_node_rrddata.json` — `/nodes/minipc/rrddata --timeframe hour --cf AVERAGE`.
  Node RRD is the only place `memtotal`/`memused`/`memavailable` exist.

## PVE 8 fixtures (recovered from pre-migration RRD files)

The same host was upgraded from PVE 8; the upgrade renames the old RRD
databases (`pve2-vm/<vmid>.old`, `pve2-node/<node>.old`) instead of deleting
them. These fixtures were recovered 2026-07-27 from those files with
`RRDs::fetch(..., "AVERAGE")` and serialized the same way
`PVE::RRD::create_rrd_data` does (columns with NaN samples omitted), i.e. the
shape a PVE 8 API would have returned for the recorded data. The column sets
are the data-source names of the actual PVE 8 RRD schemas, not a guess.

- `pve8_guest_rrddata.json` — from `pve2-vm/100.old`. PVE 8 serves both
  `/lxc/{vmid}/rrddata` and `/qemu/{vmid}/rrddata` from this same `pve2-vm`
  schema, so one guest fixture covers both endpoints. No pressure columns, no
  `memhost`, and — as on PVE 9 — no `memused`/`memavailable`.
- `pve8_node_rrddata.json` — from `pve2-node/minipc.old`. PVE 8 node RRD has
  `memtotal`/`memused` but no `memavailable` (added in PVE 9), which is why
  `NodeRRDPoint.MemAvailable` must stay an optional pointer.
