# Known RC Issue Closure For GA Audit Log Store Resilience Record

- Date: `2026-05-28`
- Gate: `known-rc-issue-closure-for-ga`
- Lanes: `L6`, `L8`, `L14`
- Issue: `https://github.com/rcourtman/Pulse/issues/1464`
- Result: `fixed-local-proof`

## Context

Issue `#1464` reports Pulse v6.0.0-rc.4 returning `Failed to fetch audit
events: Internal Server Error` when the operator opens Settings > Security >
Audit Log. The original server logs did not include the underlying audit query
error, and the follow-up asked for an rc5 retest to capture the exact cause.

Even without that rc5 log line, the v6 audit path had clear canonical gaps:
audit SQLite busy/locked failures were not retried or classified, missing or
corrupt audit-store failures collapsed into generic 500 responses, the list
endpoint accepted unbounded limits, and the frontend surfaced raw internal
server errors instead of operator-facing audit-log recovery guidance.

## Disposition

The v6 audit-log path now treats audit storage as a first-class runtime
boundary:

- SQLite audit reads, writes, schema initialization, and webhook config writes
  retry transient busy/locked failures before surfacing an error.
- The audit package classifies transient store-busy errors separately from
  unavailable, corrupt, missing, readonly, or uninitialized audit stores.
- Audit list and verification endpoints return structured `503` responses with
  `audit_store_busy` or `audit_store_unavailable` codes instead of generic
  `query_failed` 500s for those store conditions.
- Audit list pagination defaults to 100 rows and clamps oversized limits to
  1000 rows.
- Persistent audit logger detection unwraps `AsyncLogger`, so an async console
  backend is no longer treated as queryable audit storage.
- The Audit Log settings panel maps structured audit fetch failures to
  operator-facing recovery copy instead of displaying raw internal server
  errors.

This does not prove the reporter's rc4 instance hit one exact SQLite failure
mode, because the rc5 query-error log line has not been provided. It does fix
the canonical resilience and presentation gaps that could turn audit-store
pressure or initialization failures into the reported generic 500 experience.

## Proof

- `go test ./pkg/audit ./internal/api`
- `npm --prefix frontend-modern test -- --run src/utils/__tests__/auditLogPresentation.test.ts`
- `npm --prefix frontend-modern test -- --run src/components/Settings/__tests__/settingsArchitecture.test.ts`
- `npm --prefix frontend-modern run type-check`
- Browser inspection of `http://127.0.0.1:5173/settings/security-audit`

## Outcome

The v6 code path for generic Audit Log internal-server failures is hardened
with local automated proof and browser inspection. No public GitHub comment,
issue retitle, label change, or issue closure was made during this work.
Public issue closure should wait for normal maintainer issue hygiene or a
current rc retest if exact reporter-environment confirmation is required.
