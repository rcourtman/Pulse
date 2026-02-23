# Scenario: Multi-tenant Isolation

## Goal
Prove that multi-tenant controls and isolation behave correctly from the user surface.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Sign in and open organization management.
2. Create two organizations.
3. Switch between orgs and confirm org-scoped data does not leak across tenants.
4. Verify organization CRUD behavior (create/update/delete) works for the active org.
5. Record any failures, flaky transitions, or unexpected UI states.

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
