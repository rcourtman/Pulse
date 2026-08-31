# Cross-system Backup Task Timeline Contract

Status: Ready for implementation
Date: 2026-08-31
Scope: `pulse` only
Demand owner: `pulse-pro/FEATURE_REQUESTS.md`, "Cross-system backup task
timeline and outcomes"

## Decision

Add a read-only **Tasks** view to the canonical Proxmox Backups surface. The
first slice is a retained event timeline for observed PVE backup and PBS
backup, sync, verify, prune, and garbage-collection tasks across every
configured provider instance.

This is an operations-monitoring view, not a schedule editor. It shows what a
provider said ran, when it ran, and how it ended. It does not infer a planned
run from historical cadence, propose a better schedule, execute a task, or
claim that two tasks are related merely because their times and guest IDs
look similar.

## Why now

The demand ledger contains two independent Pulse asks: one paid operator
wanted currently running PBS tasks without opening PBS, and another operator
needed to correlate backup, verify, prune, sync, and GC work across two PBS
servers. A separate [homelab community
thread](https://www.reddit.com/r/homelab/comments/1pb99un/unifying_backup_status_synology_pbs_shell_on_my/)
describes backup checks as a neglected daily manual chore and asks for one
cross-system status surface.

The addressable Pulse cohort is already substantial. On the privacy-allowlisted
clean latest-ping basis on 2026-08-31, 2,548 of 6,601 persistent active
installs (38.60%) reported at least one PBS instance; 375 reported two or
more. The comparable previous week was 2,442 of 6,205 installs (39.36%), with
335 reporting two or more PBS instances. These inventory counts corroborate
the opportunity but do not prove that operators visited a backup page or that
the current workflow succeeded.

The provider contract also supports a bounded implementation. PBS exposes
task history and distinct job families, and its notification model treats
prune, sync, verification, and garbage-collection outcomes as typed events:
[PBS API viewer](https://pbs.proxmox.com/docs/api-viewer/) and [PBS notification
events](https://pbs.proxmox.com/docs/notifications.html#notification-events).
Pulse should centralize those provider-owned facts rather than recreate either
provider console.

## Current product evidence

Pulse has most of the collectors, but not a timeline authority:

- `pkg/proxmox.Client.GetBackupTasks` reads up to 200 `vzdump` tasks per
  online PVE node, including running tasks. The monitor expands multi-guest
  job logs into guest evidence and replaces the instance's current in-memory
  task slice on each successful poll.
- PVE backup attempts are also ingested as retained recovery points. That is
  useful compatibility evidence, but the recovery-point model means a
  restorable artifact; it is not the right owner for PBS prune, verify, sync,
  or GC operations.
- `pkg/pbs.Client.GetJobHealthEvidence` reads a 35-day task window for five
  families, with three bounded pages of 200 rows per family/store. It records
  permission errors and page truncation and retries warning/error scopes when
  a broad query truncates.
- PBS job evidence is projected onto the current PBS resource detail. It is
  not retained as an ordered event series, and a mutable resource snapshot
  cannot honestly answer what happened before a restart.

The first implementation therefore needs normalization, persistence, a read
API, and presentation. It does not need a new provider transport.

## Task event contract

### Identity

One provider task is keyed by:

1. provider kind (`proxmox-pve` or `proxmox-pbs`);
2. Pulse's stable configured provider-instance ID; and
3. the provider-owned UPID.

Provider display names, node names, datastore names, VMIDs, and job IDs are
not globally unique. Equal names on different configured instances must never
merge. A UPID-less row may be displayed as partial evidence but must not be
persisted as a completed task event unless the adapter can derive a stable,
provider-scoped key from worker type, worker ID, and provider start time.

### Closed fields

Each event contains:

- provider kind and provider-instance ID;
- UPID and provider node, when present;
- family: `backup`, `sync`, `verify`, `prune`, or `garbage`;
- state: `running`, `success`, `warning`, `failed`, or `unknown`;
- provider-authored start and optional end times;
- server observation and ingestion times;
- bounded provider status text, normalized separately from the closed state;
- job ID, datastore, remote, namespace, and guest identity only when directly
  supplied by the provider or a directly matched job configuration;
- evidence scope: `observed-task`, `configured-job-match`, or
  `partial-read`; and
- collection completeness: `complete`, `truncated`, or `permission-limited`.

Do not retain task logs, credentials, raw API responses, user/realm values, or
unbounded provider errors in the event row. A future log drill-down may fetch
a selected task directly under the provider's existing permissions, but it is
not part of this slice.

### Parent and guest evidence

A scheduled multi-guest PVE `vzdump` run remains one parent timeline event.
Guest outcomes parsed from its log are child evidence for drill-down and
reporting; they are not additional top-level duration bars and must not inflate
task success/failure totals. A direct single-guest task may carry that guest
identity on the parent.

PBS backup worker IDs may identify a backup group. Preserve that direct
identity, but do not equate it to a PVE guest unless the existing canonical
resource correlation produces one unambiguous match. Sync source/destination
labels come only from a direct job-configuration match.

### State and duration

Terminal provider statuses map to the closed state without browser heuristics.
Unknown statuses remain `unknown`; they do not count as success. Running
duration is `now - startTime` only while the latest successful poll still
reports the task as running.

Slice A does not label a task "stuck" from elapsed time alone. It shows the
elapsed duration and lets the operator inspect it. A later stalled-task alert
requires either an operator threshold or enough same-job history for an
explicitly tested baseline; neither should be invented in the browser.

## Persistence and collection

Create a dedicated task-event table in the recovery-data SQLite lifecycle,
separate from `recovery_points`. Reuse its file permissions, serialized
writer, backup, and pruning machinery, but keep task semantics independent of
restorable artifacts.

- Upsert by the provider-scoped identity. Polling the same event repeatedly
  must update running state without creating duplicates.
- A terminal transition is monotonic. A later partial read cannot turn a
  completed task back into running or unknown.
- Retain 35 days, expose 30 days, and default the UI to seven days. The extra
  five days allow stable boundary queries and align startup backfill with the
  existing PBS lookback.
- Backfill only from provider task history after upgrade. Existing PVE
  recovery-point rows may help validate parity, but must not be silently
  migrated when they lack the original UPID or parent/child distinction.
- Keep collection completeness by provider instance, family, store scope, and
  observation time. Empty results from a truncated or permission-limited read
  are not a clean zero.
- Preserve the current bounded paging and status-specific fallback. Do not
  remove a provider cap merely to make the timeline appear complete.

If polling is disabled for a PBS job family, the view says that family is not
monitored. It does not present an empty successful history.

## Read API

Expose a `monitoring:read` endpoint under `/api/backups/tasks` with:

- required bounded `from` and `to` times (seven-day default, 30-day maximum);
- repeatable provider-instance, family, and state filters;
- optional datastore, remote, and text search over normalized labels;
- stable descending pagination, then provider identity as the tie-breaker;
- a maximum of 500 events per page; and
- summary counts computed over the entire filtered window, not the current
  page.

The response carries events, summary counts, and collection scopes. The UI
must be able to distinguish "no failures observed" from "failure history may
be incomplete" without interpreting error strings.

## Presentation

Add `/proxmox/backups/tasks` beside the existing **By date** and **Coverage**
views. Keep the shared backup-location filter, then add URL-owned family,
state, provider, and time-window filters.

The default ordering is active running tasks, then failed/warning outcomes,
then the remaining newest tasks. The desktop view uses a time axis with one
row per provider task. Narrow layouts use an ordered activity list with start,
end or elapsed duration rather than compressing an unreadable chart.

Every row shows:

- family and normalized state;
- provider instance and node;
- start/end or running duration;
- datastore and source/destination context when directly evidenced; and
- guest/job identity when directly evidenced.

A completeness banner names the affected provider/family scope when history
was truncated, permission-limited, disabled, or not yet observed for a full
window. Absolute timestamps remain available in row details even when the
primary label is relative.

## Explicit non-goals

Slice A does not:

- read or edit provider schedules;
- scrape systemd timers or arbitrary host scripts;
- recommend schedule changes or resource-pressure fixes;
- start, stop, retry, prune, verify, sync, back up, or garbage-collect;
- accept arbitrary commands or runbooks;
- infer guest downtime, bottlenecks, or task dependencies from overlapping
  bars;
- claim backup completion proves repository integrity or restorability; or
- replace provider task logs and configuration screens.

## Acceptance gates

1. Equal UPIDs or job/datastore names on separate provider instances remain
   separate through collection, restart, API filtering, and rendering.
2. A running task updates one row and transitions once to its terminal state;
   stale or partial reads cannot regress it.
3. Multi-guest PVE jobs render one parent bar and retain child guest outcomes
   without double-counting summaries.
4. PBS backup, sync, verify, prune, and GC fixtures map to closed families and
   states; unknown worker/status values remain visible as unknown.
5. Truncated, permission-limited, disabled, and never-observed scopes cannot
   render as complete green history.
6. Restart tests prove retained ordering and identity; retention tests prove
   both event and completeness rows are pruned.
7. The API enforces the time and page bounds and computes summaries outside
   page slicing.
8. Rendered browser verification covers two PVE and two PBS instances, running
   and failed overlaps, duplicate names, partial history, and phone width.
9. No task action, schedule mutation, raw log, credential, or provider user
   identity appears in the payload or browser.

## Follow-on evidence gates

After the timeline ships, measure use with one content-free event for opening
the Tasks view and one for applying a failure/family filter. Do not emit task
IDs, provider names, counts, or filters.

The daily backup failure-rate summary requested for MSP reports can reuse this
authority after event completeness is proven. Missing-run alerts, schedule
overlays, pressure correlation, and orchestration remain separate ledger
decisions and require their own evidence and trust contracts.
