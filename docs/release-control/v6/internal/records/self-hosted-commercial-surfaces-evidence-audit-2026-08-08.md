# Self-Hosted Commercial Surfaces Evidence Audit

- Audit date: `2026-08-08`
- Audited record: `self-hosted-commercial-surfaces-revision-2026-08-07.md`
- Source cutoff under test: `2026-08-07T12:47:46Z`
- Result: `exact rationale not reproducible`
- Data handling: `aggregate-only read, no install, customer, subscription, or resource identifiers returned`

## Provenance

- Source repository: `pulse-pro@6fa9bc6dc2c2df917c16964a0305d41967162f1f`
- Source schema: `license-server/main.go`, table `telemetry_pings`
- Access path: tracked pinned-host wrapper `pulse-pro/scripts/license_ssh.sh`
- Query mode: SQLite `-readonly`
- Measured at: `2026-08-08T01:47:00Z`
- Original query or snapshot: `not found`

The audit searched current tracked files, all reachable git history, commit
messages, and git notes in `pulse` and `pulse-pro`. The exact figures and the
`Richard` approval label existed only in the audited prose record. No git notes
namespace was present.

## Sanitized Reconstruction Query

This query is a forensic reconstruction. It is not represented as the original
query. It returns aggregate counts only.

```sql
WITH latest AS (
  SELECT
    p.*,
    ROW_NUMBER() OVER (
      PARTITION BY p.install_id
      ORDER BY p.received_at DESC, p.rowid DESC
    ) AS rn
  FROM telemetry_pings p
  WHERE p.received_at >= datetime('2026-08-07 12:47:46', '-7 days')
    AND p.received_at < '2026-08-07 12:47:46'
    AND COALESCE(p.version_is_development, 0) = 0
), classified AS (
  SELECT
    CASE
      WHEN COALESCE(pve_nodes, 0) >= 5
        OR COALESCE(docker_hosts, 0) >= 10
        OR COALESCE(vmware_hosts, 0) >= 3
      THEN 1 ELSE 0
    END AS business,
    CASE WHEN COALESCE(paid_license, 0) != 0 THEN 1 ELSE 0 END AS paid
  FROM latest
  WHERE rn = 1
)
SELECT
  COUNT(*) AS installs,
  SUM(business) AS business_estates,
  SUM(CASE WHEN business = 1 AND paid = 1 THEN 1 ELSE 0 END) AS business_paid,
  ROUND(
    100.0 * SUM(CASE WHEN business = 1 AND paid = 1 THEN 1 ELSE 0 END)
      / NULLIF(SUM(business), 0),
    4
  ) AS business_paid_pct,
  SUM(CASE WHEN business = 0 AND paid = 1 THEN 1 ELSE 0 END) AS smaller_paid,
  ROUND(
    100.0 * SUM(CASE WHEN business = 0 AND paid = 1 THEN 1 ELSE 0 END)
      / NULLIF(SUM(CASE WHEN business = 0 THEN 1 ELSE 0 END), 0),
    4
  ) AS smaller_paid_pct
FROM classified;
```

The pre-GA comparison used the same aggregate distinct-install calculation with
cutoffs at the start and end of 2026-07-04 UTC:

```sql
SELECT COUNT(DISTINCT install_id)
FROM telemetry_pings
WHERE received_at >= datetime(:cutoff, '-7 days')
  AND received_at < :cutoff;
```

## Sanitized Snapshot

| Cutoff and filter | Weekly active installs | Business estates | Business paid | Business paid share | Smaller paid | Smaller paid share |
|---|---:|---:|---:|---:|---:|---:|
| Landing commit, non-development | 9,426 | 672 | 53 | 7.8869% | 86 | 0.9824% |
| Landing commit, published non-development | 9,424 | 672 | 53 | 7.8869% | 85 | 0.9712% |
| End of 2026-08-07 UTC, non-development | 9,527 | 672 | 53 | 7.8869% | 83 | 0.9373% |

Pre-GA weekly active install counts varied materially with the unspecified
cutoff:

| Cutoff | Weekly active installs |
|---|---:|
| `2026-07-04T00:00:00Z` | 362 |
| `2026-07-05T00:00:00Z` | 958 |

Schema inspection at audit time found only two schema-v8 rows, first received
on `2026-08-08T00:10:01Z`. The private landing commit explicitly added the
stored `business_estate` column without backfill. The original 2026-08-07
figures therefore had to derive classification from earlier resource counts.

## Findings

1. The exact 9,421-install and 671-estate population does not reproduce from
   the documented thresholds and observable non-development or published
   filters. Reaching it would require an additional exclusion rule that was not
   recorded.
2. The 7.9% and 0.98% figures closely match paid-license share in the latest
   weekly ping per install. The data does not establish that either cohort
   converted during the window or that estate size caused conversion.
3. The pre-GA baseline lacks an exact cutoff and changes substantially across
   the GA date boundary.
4. No query or durable report was found for the claim about new paid
   subscriptions per month. The 2026-08-07 canonical finance snapshot reports
   active subscriptions and revenue, not that acquisition rate.
5. No repository or git artifact confirms project-owner approval of the product
   reversal.

The reconstructed data is useful context for a new explicit owner decision. It
is not sufficient to preserve the earlier approval label or causal conversion
claim.
