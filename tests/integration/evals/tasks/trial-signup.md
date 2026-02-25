# Scenario: Pro Trial Signup

## Goal
Prove the local trial activation path works without a credit card, activates Pro entitlements, and prevents duplicate trials.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Verify pre-trial entitlements show `trial_eligible=true`.
2. Start trial via `POST /api/license/trial/start` and confirm HTTP 200 with `subscription_state=trial`.
3. Verify post-trial entitlements are `tier=pro`, `subscription_state=trial`, and not lifetime.
4. Verify license panel shows trial expiry date and day countdown.
5. Verify second trial start is rejected with HTTP 409.

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
