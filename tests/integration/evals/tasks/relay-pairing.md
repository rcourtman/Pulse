# Scenario: Relay Pairing & Mobile Readiness

## Goal

Prove that relay pairing works end-to-end: configure relay settings, verify connection to the relay server, generate a QR code for mobile pairing, and confirm the onboarding deep link is available.

## Environment

- Base URL: `{{base_url}}`
- Username: `{{username}}`
- Password: `{{password}}`
- Relay Host: provided via `PULSE_E2E_RELAY_HOST`

## Required checks

1. Relay feature is available in license entitlements (`GET /api/license/entitlements` includes `relay` capability).
2. Relay settings can be configured with the test relay server (`PUT /api/settings/relay` with `enabled: true, server_url: "..."` returns 200).
3. Relay status becomes connected within 60s (`GET /api/settings/relay/status` → `connected: true`).
4. Onboarding QR code returns required fields (`GET /api/onboarding/qr` → `schema`, `instance_url`, `relay`, `auth_token`, `deep_link`).
5. Onboarding deep link is available (`GET /api/onboarding/deep-link` → `url` field non-empty).
6. Relay settings page in the UI shows relay-related content (Relay, Connected, Enabled, etc.).

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
