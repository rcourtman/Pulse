# AI Patrol Finding Lifecycle

## Objective
Validate the full AI Patrol closed-loop lifecycle: patrol status, findings,
force run, run history, autonomy settings, approval queue, investigation,
finding acknowledge/resolve, suppression/dismissed endpoints, and AI page UI.

## Steps

### Patrol Status & Configuration
1. Log in as admin.
2. GET `/api/ai/patrol/status` — verify response includes `enabled`, `running`, `healthy`, `summary` (with severity counts), `findings_count`, `interval_ms`.
3. GET `/api/ai/patrol/autonomy` — verify `autonomy_level` is one of `monitor|approval|assisted|full`.

### Patrol Execution
4. POST `/api/ai/patrol/run` — trigger patrol run (accept 200, 429 rate-limited, or 503 unavailable).
5. GET `/api/ai/patrol/runs` — verify run history array with `id`, `started_at`, `status`, `resources_checked`.

### Findings & Investigation
6. GET `/api/ai/patrol/findings` — verify array of findings with `id`, `severity`, `category`, `resource_id`, `title`.
7. For any finding: GET `/api/ai/findings/{id}/investigation` — verify 200 (investigation exists) or 404 (no investigation yet).

### Finding Lifecycle (Closed-Loop)
8. POST `/api/ai/patrol/acknowledge` with `{ finding_id }` — verify success.
9. GET `/api/ai/patrol/findings` — verify finding still present (acknowledged ≠ removed).
10. POST `/api/ai/patrol/resolve` with `{ finding_id }` — verify success.
11. GET `/api/ai/patrol/findings` — verify finding is removed from active list.

### Approval Queue
12. GET `/api/ai/approvals` — verify 200 with `approvals` + `stats`, or 402 paywall.

### Supplementary Endpoints
13. GET `/api/ai/patrol/suppressions` — verify 200 response.
14. GET `/api/ai/patrol/dismissed` — verify 200 response.

### UI Verification
15. Navigate to `/ai` — verify AI page renders with patrol/intelligence content.

## Expected Outcomes
- Patrol status reflects actual service state.
- Force patrol triggers or rate-limits correctly.
- Finding lifecycle completes: acknowledge → resolve → removed.
- Approval queue responds (200 or 402 paywall).
- AI page renders meaningful content.
