# Scenario: Pulse Cloud Hosted Signup

## Goal
Prove hosted cloud signup and magic-link request flows work end-to-end using real Stripe sandbox checkout from the public signup page.

## Environment
- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`

## Required checks
1. Open `/cloud/signup` and verify form renders.
2. Submit valid workspace creation details.
3. Complete Stripe sandbox checkout and verify return to hosted signup completion page.
4. Request a magic-link email from the same page.
5. Validate the magic-link request endpoint responds with success contract.

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
