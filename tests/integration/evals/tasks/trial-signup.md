# Scenario: Pro Trial Signup

## Goal
Prove the self-hosted Pulse instance initiates the hosted trial flow correctly, does not mint local entitlement before activation, and rate-limits duplicate initiation attempts.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Verify pre-trial entitlements show `trial_eligible=true`.
2. Start trial via `POST /api/license/trial/start` and confirm HTTP `409` with code `trial_signup_required`.
3. Verify the response includes a hosted `action_url` that targets `/start-pro-trial`.
4. Verify post-initiation entitlements remain unactivated locally (`trial_eligible=true`, `subscription_state=expired`).
5. Verify a second immediate trial start attempt is rate limited with HTTP `429`.

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
