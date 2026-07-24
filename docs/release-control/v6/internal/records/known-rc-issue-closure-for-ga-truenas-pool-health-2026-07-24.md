# Known RC Issue Closure For GA TrueNAS Pool Health Record

- Date: `2026-07-24`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `#1506`
- Result: `fixed-main-proof`

## Context

TrueNAS already projected pools, datasets, disks, apps, native alerts, SMART
state, temperature, boot-pool state, snapshot and replication evidence, and
connection-local identity. The pool adapter retained only the pool status word,
however. It discarded the native GUID, status detail, structured scrub or
resilver state, read/write/checksum counters, vdev topology, path-only leaves,
and explicit missing-member evidence. Synthetic degraded-pool incidents fired
and cleared on one poll, while locked/unmounted datasets and failed app
runtime state did not participate in the shared incident lifecycle. Native
Ceph health checks were flattened to one message.

## Disposition

TrueNAS `pool.query` and `boot.get_state` now preserve the complete native pool
report needed by the canonical storage-health contract. The provider projects
the full ZFS report plus one additive provider-neutral `PoolHealth` envelope.
The normalized state contract covers `ONLINE`, `DEGRADED`, `FAULTED`,
`OFFLINE`, and `UNAVAIL`; structured scrub and resilver work, terminal scan
errors, pool and vdev counters, path-only leaves, spare/mirror/RAIDZ topology,
and explicit native missing members remain distinct evidence.

Synthetic pool, vdev, dataset, disk, app, and app-container incidents require
two consecutive observations to activate and two healthy observations to
recover. Active state, acknowledgement, suppression, escalation, history, and
restart recovery remain in the shared alerts runtime. Missing resources and
unknown telemetry do not prove recovery. Native TrueNAS alerts suppress only
equivalent synthetic signals; distinct error evidence remains visible. Pulse
does not introduce a second TrueNAS email path.

Locked and unmounted datasets are availability conditions. Readonly alone is
not a fault, and receive-side replication targets classified by native
replication evidence remain healthy. Stopped and crashed apps plus
crashed/exited child containers participate through the canonical workload
incident path and can be suppressed for intentional downtime.

Ceph uses the same provider-neutral envelope only at cluster scope and only
from native `HEALTH_OK`, `HEALTH_WARN`, `HEALTH_ERR`, and native health-check
evidence. It does not infer a failed OSD, disk, ZFS leaf, or replacement target.

## Proof

- complete TrueNAS client/provider, storage-health, unified-resource, alerts,
  monitoring, and API package suites
- provider shape coverage for pool GUID/detail, structured scan, errors,
  mirror/spare/path-only topology, and explicit native missing-device evidence
- alert lifecycle coverage for transient activation, confirmed recovery,
  severity escalation under one identity, acknowledgement/history continuity,
  missing telemetry, and manager restart
- two-appliance poller coverage proving identical hostname, pool name, and
  restored pool GUID do not merge connection-local resources
- API serialization coverage proving `PoolHealth`, full ZFS evidence, and
  confirmation fields survive list pruning and canonical refresh
- frontend model coverage for actionable pool detail and native-alert
  deduplication, plus the complete frontend TypeScript check
- current-build Chromium proof in managed mock mode: the TrueNAS storage page
  rendered the degraded `archive` pool, and its shared resource drawer showed
  canonical and native `DEGRADED`, the evidence-bounded recommendation,
  `zfs_pool_state`, and `pool.query`; the browser console had zero errors
- issue-owned Go race tests and the v6 control-plane, status, registry, and
  contract audits

## Outcome

The core degraded-pool, native fault-evidence, dataset, and TrueNAS app outcome
is complete on `main` for a future v6 release. It is not part of `v6.1.1`, and
no publication date is claimed. Production-batch disk provenance, guest
`lm-sensors` guidance, NVIDIA temperature collection, and Portainer-managed
container lifecycle remain separate follow-ups because the current TrueNAS API
evidence does not support those claims. Issue `#1506` remains open until a
release containing this change is available for reporter confirmation.
