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
