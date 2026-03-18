# Audit Log + Reporting Lifecycle

## Objective
Validate audit logging and reporting enterprise features: audit event
listing, export, summary, webhook configuration, notification webhooks,
single/multi-resource report generation, and UI rendering.

## Steps

### Audit Logging
1. Log in as admin.
2. GET `/api/audit?limit=10` — verify events list or 402 paywall.
3. GET `/api/audit/export?format=json` — verify file download (or 501 for no logger).
4. GET `/api/audit/summary` — verify summary statistics.
5. GET/POST `/api/admin/webhooks/audit` — CRUD audit webhook URLs.
6. Navigate to `/settings/security-audit` — verify audit log viewer renders.

### Notification Webhooks (not license-gated)
7. GET `/api/notifications/webhooks` — list webhooks.
8. GET `/api/notifications/webhook-templates` — verify templates available.
9. GET `/api/notifications/webhook-history` — verify history available.
10. GET `/api/notifications/health` — verify notification system health.

### Reporting
11. GET `/api/state` — find a resource to report on.
12. GET `/api/admin/reports/generate?resourceType=node&resourceId=...&format=csv` — generate report.
13. POST `/api/admin/reports/generate-multi` — generate fleet report.
14. Navigate to `/operations/reporting` — verify reporting page renders.

## Expected Outcomes
- Audit events and exports work when `audit_logging` is licensed.
- Reports generate when `advanced_reporting` is licensed.
- 402 paywall responses have correct format when not licensed.
- UI pages render relevant content or upgrade messaging.
