# Scenario: Cloud Billing Lifecycle

## Goal
Prove post-checkout billing lifecycle behavior works end-to-end for Pulse Cloud by validating tenant provisioning and cancellation transitions.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Create a real Stripe sandbox checkout from `/cloud/signup`.
2. Complete checkout with Stripe test card details.
3. Verify tenant reaches active state in control-plane admin tenant listing.
4. Trigger subscription cancellation and deliver corresponding webhook event.
5. Verify tenant transitions to canceled state after webhook processing.

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
