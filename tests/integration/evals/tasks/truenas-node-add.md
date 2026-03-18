# Scenario: TrueNAS Node Addition

## Goal

Prove that adding a TrueNAS connection via the API results in ZFS pools and datasets becoming visible in Pulse's unified resource model and UI.

## Environment

- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`
- TrueNAS Host: provided via `PULSE_E2E_TRUENAS_HOST`
- TrueNAS API Key: provided via `PULSE_E2E_TRUENAS_API_KEY`

## Required checks

1. Test TrueNAS connection succeeds (`POST /api/truenas/connections/test` returns 200).
2. Add TrueNAS connection succeeds (`POST /api/truenas/connections` returns 201 Created with an `id` field).
3. Connection appears in connections list (`GET /api/truenas/connections`).
4. TrueNAS resources appear in unified state within 90s (`GET /api/state` → `resources[]` entries with `sourceType: "truenas"` or `platformType: "truenas"`).
5. ZFS pools or datasets visible in unified state (`GET /api/state` → `resources[]` or `storage[]` with TrueNAS source).
6. TrueNAS resources visible on the infrastructure page in the browser.
7. Storage page shows TrueNAS pool/dataset information.
8. Cleanup: delete the test connection after all checks.

## Output contract

Write JSON to `{{result_json}}` with this shape:

```json
{
  "status": "pass",
  "summary": "short outcome summary",
  "evidence": ["bullet point 1", "bullet point 2"],
  "issues": []
}
```

Use `"status": "fail"` when any required check fails.
