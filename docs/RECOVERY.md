# Recovery

Pulse collects backup, snapshot and replication evidence from connected
providers. Use it to find recorded artifacts, identify gaps in protection and
check the source of a backup before using the provider's own recovery tools.
Pulse does not restore workloads from these views.

## Where to look

There is no top-level Recovery page. Open the existing platform view:

- **Proxmox → Backups** (`/proxmox/backups`): **By date** lists recorded
  artifacts; **Coverage** groups them by workload. Expand a coverage row to
  inspect its posture reason and individual artifacts. Backup servers have
  their own rows on the same page.
- **TrueNAS → Protection** (`/truenas/protection`): snapshots and replication
  activity reported by the TrueNAS connection.
- **Kubernetes**: recovery records depend on the agent reporting VolumeSnapshot
  or Velero inventory. An installed agent or a connected cluster does not
  establish that either is available.

Provider collection is separate from presentation. Not every platform displays
the same columns or filters, and a missing reading is not proof that no backup
exists. [Check missing evidence](#missing-or-inconsistent-evidence) before
changing the provider or Pulse's stored data.

## What the evidence means

Keep these observations separate:

| Observation | What it establishes | What it does not establish |
|---|---|---|
| Successful task or `success` outcome | A provider-reported result or collected artifact | Guest thaw, application health or a tested restore |
| Running task or partial artifact | Work or an incomplete artifact is observed | A completed recovery point, even if a size is shown |
| Verified backup | The provider's verification result for that artifact | A successful application restore or guest thaw |
| Recent snapshot | A point on the source system | An independent copy that survives loss of that system |
| Protected posture | Current, linked evidence meets Pulse's protection policy | A recovery guarantee or permission to discard other backups |

**A backup task's OK status is not confirmation that the guest has thawed.**
For freeze-enabled Proxmox backups, stop Pulse before the backup and restart it
only after independently confirming guest thaw. Pulse monitoring and alert
delivery are unavailable while it is stopped. Do not send additional
guest-agent commands during freeze/thaw to diagnose the problem, or disable
filesystem freezing merely to make a task look successful. Follow the
[backup safety precaution](VM_DISK_MONITORING.md#backup-safety); it is not a
claim that Pulse has repaired or reproduced a native thaw failure.

Before relying on an artifact, confirm its provider, repository, workload
identity and time in the provider's own tools. For recovery assurance, test a
restore to an isolated destination using that provider's procedure and check
the restored application's data and operation. Do not overwrite the live
workload as a diagnostic test, delete older backups, or change retention to
hide a Pulse display problem.

### Artifacts and subjects

A **recovery point** is a collected backup, snapshot or replication result.
Its **subject** is the protected workload, dataset or PVC; its **repository**
is the storage location. Pulse links subjects to monitored resources where it
can, and retains provider-local references when it cannot.

Independent Proxmox installations can reuse a VMID. Compare the connection,
node, guest type/ID, datastore, namespace and artifact time; matching names or
VMIDs alone are not sufficient to identify a duplicate or the right restore.
Direct PBS observations and PBS-through-PVE observations can differ in detail.
See [PBS data sources](PBS.md#data-source-indicator).

### Outcomes and verification

Recorded outcomes are `success`, `warning`, `failed`, `running` or `unknown`.
An unknown outcome is not success; an error fetching records is not an empty
history. Where available, the API returns `startedAt`, `completedAt`,
`sizeBytes` and provider-specific `details`. Missing times and sizes are not
zero-valued measurements. A source backup timestamp is not necessarily the
completion time of a later copy or sync task.

The `verified` field is optional: `true` is a positive provider verification
observation; `false` supplies no positive verification, so inspect the
provider's detailed state; an omitted value supplies no verification result.
Do not interpret an omitted field as a failed verification. Rollups may also
report `verifyIntent` (`verified`, `stale` or `unknown`) and `lastVerifiedAt`;
these summarise observed verification recency, not an end-to-end restore test.

### Protection posture

Pulse combines subject-linked artifacts with current provider collection
evidence:

- **Protected**: a qualifying current recovery point is linked to the resource
  and complete provider evidence does not invalidate the claim.
- **Attention**: evidence is stale, failing, incomplete or unverified when
  verification is expected.
- **Unprotected**: complete evidence confirms that no qualifying protection
  exists.
- **Unknown**: identity, permissions or collection completeness cannot support
  a stronger claim.

A retained artifact can remain visible while posture is unknown. A running
backup is not a new completed recovery point; inspect any earlier completed
point separately rather than treating current activity as protection.

## Missing or inconsistent evidence

1. Open the relevant platform view, clear its search and filters, and check the
   selected connection and organisation. Record the route that is affected.
2. Inspect connection health, the latest observation time and any collection
   error. Wait for that provider's configured polling interval, not a fixed
   30-second assumption. A green connection badge does not prove that every
   history or backup read succeeded.
3. Check the expected artifact in the provider's own inventory using an
   existing authorised session. Verify its identity, location and permissions;
   do not grant write or restore permissions merely to fill a monitoring row.
4. For an **Unknown** posture, expand the coverage row and use the specific
   limiting evidence. Resolve that collection or identity gap; do not turn
   retained artifacts into a protection claim by clearing Pulse history.

If the guest stopped responding during backup, preserve the task's warning or
thaw error and the observations already available. Do not repeat the backup,
reset the guest, or run active guest-agent probes just to reproduce it. Use
the provider's recovery procedure and a separate trusted guest console to
assess guest liveness and thaw, not Pulse's backup badge.

When reporting missing records, share the failing view, running Pulse version,
affected provider, time range, expected reading and relevant redacted error.
Use consistent placeholders for private hostnames, datasets and IDs. Do not
share full backup inventories, request headers, tokens, cookies or recovery
keys. See [safe issue reporting](TROUBLESHOOTING.md#-getting-help).

## API reference

These are read-only monitoring endpoints. Use an existing authenticated browser
session or a `monitoring:read` token supplied through a private header file,
never a token in a URL or diagnostic command argument. See [API access](API.md).

| Method | Endpoint | Result |
|---|---|---|
| `GET` | `/api/recovery/points` | Paged recovery points |
| `GET` | `/api/recovery/rollups` | Paged subject summaries |
| `GET` | `/api/recovery/series` | Completed-point counts grouped by day |
| `GET` | `/api/recovery/facets` | Filter values from matching points |
| `GET` | `/api/recovery/postures` | Canonical resource postures and limiting evidence |

### Point and rollup filters

Points, rollups, series and facets accept these filters:

| Parameter | Values or meaning |
|---|---|
| `platform` | `proxmox-pve`, `proxmox-pbs`, `truenas` or `kubernetes`; `provider` is an alias |
| `kind` | `backup`, `snapshot` or `other`; TrueNAS replication results use `backup` |
| `mode` | `local`, `remote` or `snapshot` |
| `outcome` | `success`, `warning`, `failed`, `running` or `unknown` |
| `from` | Lower time bound in RFC3339, for example `2026-10-01T00:00:00Z` |
| `to` | Upper time bound in RFC3339 |
| `subjectResourceId` | Exact linked resource identity; `itemResourceId` is an alias |
| `rollupId` | Exact subject summary identity |
| `q` | Text search; `query` is an alias |

For example, this relative browser request selects PBS backup records within
the stated time bounds without putting a credential in the URL:

```text
/api/recovery/points?platform=proxmox-pbs&kind=backup&from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z&page=1&limit=100
```

Use `from`/`to`, not `since`/`until`, and `subjectResourceId`, not `subject`.
Unrecognised query names do not apply those filters. Invalid `from` or `to`
values return an error rather than an empty result.

Points and rollups accept `page` (starting at 1) and `limit` (default **100**,
maximum **500**). Their response has a `data` array and `meta` pagination;
inspect every required page rather than assuming the first is the inventory.
Series and facets return aggregate data, not that paged-list contract.

### Posture lookup

`/api/recovery/postures` uses a separate filter contract. Repeat `resourceId`
for a batch of at most **200** resource IDs, or omit it for a paged list.
`state=attention` selects attention postures; `page` and `limit` control list
pagination. Its response includes policy and provider evidence states. Do not
apply point filters to this endpoint or re-derive posture from artifact count.

## See also

- [PBS integration](PBS.md) — access, data sources and host History
- [TrueNAS integration](TRUENAS.md) — snapshots and replication monitoring
- [VM disk monitoring](VM_DISK_MONITORING.md) — guest-agent and backup safety
- [Migrating Pulse](MIGRATION.md) — configuration transfer versus a full-state backup of Pulse itself
