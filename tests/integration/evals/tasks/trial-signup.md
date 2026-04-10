# Scenario: Pro Trial Signup

## Goal
Prove the self-hosted Pulse instance initiates the hosted trial flow correctly, does not mint local entitlement before activation, and enforces the hosted-signup retry-burst contract for duplicate initiation attempts.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Verify pre-trial entitlements show `trial_eligible=true`.
2. Start trial via `POST /api/license/trial/start`.
3. If the first response is HTTP `409`, confirm code `trial_signup_required` and a hosted `action_url` that targets `/start-pro-trial`.
4. If the first response is HTTP `429`, confirm code `trial_rate_limited` and that `Retry-After` matches `details.retry_after_seconds`.
5. Verify post-initiation entitlements remain unactivated locally (`trial_eligible=true`, `subscription_state=expired`).
6. Retry `POST /api/license/trial/start` immediately:
   - If the first response was `409`, accept either another `409 trial_signup_required` while the retry burst is still open or `429 trial_rate_limited` once the limiter engages.
   - If the first response was already `429`, confirm the retry remains `429 trial_rate_limited` with matching backoff metadata.

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
