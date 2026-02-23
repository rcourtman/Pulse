# Scenario: Pro Trial Signup Return

## Goal
Prove the real trial activation path requires hosted signup, completes Stripe sandbox checkout, and returns with real trial expiry details (not lifetime).

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Open settings and locate Pro trial action.
2. Start trial and confirm redirect to hosted signup endpoint.
3. Submit hosted trial form and complete Stripe sandbox checkout with test card details.
4. Verify license panel shows trial expiry date and day countdown.
5. Verify backend entitlements are `tier=pro`, `subscription_state=trial`, and not lifetime.

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
