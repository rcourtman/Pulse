# PBS metric-history qualification (#1882)

This is an operator-run release check, not evidence that an installed release
passes. Use an authorised disposable Pulse/PBS environment; do not interrupt a
customer datastore. Record the exact Pulse binary SHA-256 or image digest,
version, source SHA where available, configured poll intervals and UTC phase
boundaries. Use synthetic datastore names in shared receipts.

## Measurement

Whole-table counts across unlike resources cannot establish amplification:
retention windows and numbers of metric types differ. Equal values at **new
source observation times** are valid observations. A unique SQLite identity
alone also cannot detect cached values incorrectly stamped with new times.

Use a consistent SQLite online backup made by the authorised operator (not a
copy of the database file without its WAL). Run this read-only query against
that backup. Replace the synthetic resource ID and epoch bounds with the
measured datastore and a half-open UTC window, after buffered writes have
flushed. The default flush interval is five seconds; record any override.

```sql
WITH selected AS (
  SELECT metric_type, timestamp
  FROM metrics
  WHERE resource_type = 'storage' AND resource_id = 'pbs-synthetic/backups'
    AND tier = 'raw' AND timestamp >= 1000 AND timestamp < 1060
), samples AS (
  SELECT metric_type, timestamp, count(*) AS copies
  FROM selected GROUP BY metric_type, timestamp
), gaps AS (
  SELECT *, timestamp - lag(timestamp) OVER (
    PARTITION BY metric_type ORDER BY timestamp
  ) AS gap_seconds FROM samples
)
SELECT metric_type, sum(copies) AS rows, count(*) AS observation_times,
       min(timestamp) AS first_epoch, max(timestamp) AS last_epoch,
       min(gap_seconds) AS shortest_gap_seconds,
       max(gap_seconds) AS longest_gap_seconds,
       sum(copies - 1) AS duplicate_identity_rows
FROM gaps GROUP BY metric_type ORDER BY metric_type;
```

Compare each of `usage`, `used`, `total`, `avail` with independently recorded
successful PBS source observations, not unrelated PVE poll completions. Keep
the window within raw retention. An empty result is not a pass.

## Phases and acceptance

1. Healthy PBS with multiple unrelated PVE poll completions: each metric has
   one stored point per distinct source observation second, not per registry
   rebuild. Record successful source timestamps; do not infer them from rows.
2. Keep values unchanged across later successful PBS polls: those later
   observations must remain in history. Value-based deduplication is wrong.
3. Interrupt only the disposable PBS source, leaving other polls running:
   cached observations must not acquire new timestamps. Keep failure evidence
   separate from source observations; a missing observation is not zero usage.
4. Recover PBS: new observations resume without filling the outage with
   invented samples. Then restart the whole Pulse process and repeat healthy
   and outage phases; verify persisted pre-restart history remains intact.

PBS explicitly supports offline maintenance that blocks datastore reads and
writes ([upstream documentation](https://pbs.proxmox.com/docs/maintenance.html#maintenance-mode)).
This motivates interruption coverage; it does not establish a Pulse defect.

The existing `TestPBSObservationHistorySurvivesRebuildsAndStoreReopen` covers
synthetic registry rebuilds, unchanged later values and store reopen, not a
whole-process restart, real upstream outage or installed artifact. Attach the
phase table, per-metric query results and exact artifact identity to the
candidate receipt. Do not close #1882 on this checklist or on local tests alone.

## Write-cost companion measurement

Sample correctness, PBS API request load and persistence write cost are three
separate measurements. Upstream's historical
[RRD journal proposal](https://lists.proxmox.com/pipermail/pbs-devel/2021-October/004207.html)
illustrates why stored point counts alone cannot establish bytes written; it
is not evidence of a current Pulse defect or the installed PBS implementation.

For the same authorised environment, compare baseline and repaired artifacts
under matched inventory, polling, retention, logging and filesystem settings.
Record warm-up separately, then equal-duration steady-state windows covering
multiple flushes. Record UTC boundaries, elapsed seconds, process identity and
restarts, successful source observations and request counts alongside:

- Pulse process `write_bytes` and `cancelled_write_bytes` counter deltas from
  `/proc/<pid>/io`, where existing permissions allow; keep both counters rather
  than treating `wchar` as disk traffic. Never subtract across a process restart.
- Independently observed host/device write bytes over that window, naming the
  device and filesystem and recording other workload, including PBS jobs.
- Database and WAL sizes at each boundary as context only: file growth is not
  cumulative write traffic. Record checkpoint/retention activity where known.

[Linux documents these process counters and their limitations](https://man7.org/linux/man-pages/man5/proc_pid_io.5.html).
They include waited-for children and are not a measurement of SSD wear or
physical device write amplification. Device totals include other writers and
cannot alone attribute traffic to Pulse. Missing permission or unavailable
counters means “not measured”, not zero; do not elevate access for this check.

Report bytes/elapsed-second separately from per-metric observation counts.
Repeat anomalous windows under matched conditions before assigning causality;
there is no established universal byte-rate pass threshold. Take the SQLite
backup after the measured window, and record it separately so qualification's
own I/O is not mistaken for steady-state persistence. Attach raw numeric
counter boundaries with synthetic resource identifiers, not credentials,
process environments or customer database contents.
