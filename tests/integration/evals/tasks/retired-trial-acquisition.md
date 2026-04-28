# Scenario: Retired Trial Acquisition

## Goal
Prove the self-hosted Pulse instance no longer initiates a hosted Pro trial from the ordinary in-app runtime and does not mutate local entitlements when the retired route is probed.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Capture current entitlements.
2. Probe `POST /api/license/trial/start`.
3. Confirm the response is HTTP `404`.
4. Confirm the response does not contain legacy hosted-signup or trial-rate-limit acquisition payloads.
5. Verify post-probe entitlements match the pre-probe entitlement summary.

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
