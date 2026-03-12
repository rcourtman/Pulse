# Hosted Signup Billing Replay Record

- Date: `2026-03-12`
- Gate: `hosted-signup-billing-replay`
- Assertion: `RA2`
- Environment:
  - Hosted-mode rehearsal instance: `http://127.0.0.1:17765`
  - Hosted-mode fail-closed instance without `PULSE_PUBLIC_URL`: `http://127.0.0.1:17766`
  - Authenticated platform admin: `admin`
  - Trial redirect target: `https://billing.example.com/start-pro-trial?source=rc-check`
  - Stripe webhook secret: local test secret on the live HTTP surface

## Automated Proof Baseline

- `go test ./internal/api -run 'TestHostedLifecycle|TestHostedSignupSuccess|TestHostedSignupValidationFailures|TestHostedSignupHostedModeGate|TestHostedSignupRateLimit|TestHostedSignupRateLimit_NoProvisioningSideEffects|TestHostedSignupCleanupOnRBACFailure|TestHostedSignupFailsClosedWithoutPublicURL|TestStripeWebhook_' -count=1`
- `go test ./internal/hosted -run 'TestProvisionTenantSuccess|TestProvisionTenantIdempotentDuplicateEmail|TestProvisionTenantIdempotentDuplicateEmailCaseInsensitive|TestProvisionTenantValidationFailures|TestProvisionTenantPartialFailureRollback|TestProvisionHostedSignupSuccess' -count=1`
- `cd frontend-modern && npx vitest run src/pages/__tests__/HostedSignup.test.tsx src/components/Settings/__tests__/BillingAdminPanel.test.tsx`
- Result: pass

## Manual Exercise

1. Started a hosted-mode Pulse instance without `PULSE_PUBLIC_URL` and confirmed `POST /api/public/signup` failed closed with:
   - `503`
   - `code=public_url_missing`
   - no new hosted organization metadata created (`org_count` stayed unchanged)
2. On the configured hosted-mode instance, started the self-hosted trial/upgrade path through `POST /api/license/trial/start` as the authenticated admin and confirmed:
   - response was `409`
   - `code=trial_signup_required`
   - `details.action_url` pointed at the hosted checkout origin with `org_id=default` and `return_url=http://127.0.0.1:17765/auth/trial-activate`
   - `GET /api/license/entitlements` stayed `tier=free` and `subscription_state=expired` before and after the redirect handoff
3. Exercised real hosted signup on the live HTTP surface via `POST /api/public/signup` and confirmed:
   - `201 Created`
   - returned `org_id=0147dd50-38db-4316-8d46-0e1f0d754bf6`
   - returned `message="Check your email for a magic link to finish signing in."`
4. Confirmed hosted magic-link access remained live by calling `POST /api/public/magic-link/request` for the same signup email and receiving `200` with `success=true`.
5. Confirmed billing-admin state for the newly provisioned hosted org reflected the seeded hosted signup state:
   - `GET /api/admin/orgs/0147dd50-38db-4316-8d46-0e1f0d754bf6/billing-state` returned `subscription_state=trial`
   - `plan_version=cloud_trial`
   - hosted trial capabilities were present
6. Exercised exact webhook replay behavior on the live HTTP surface with the same signed `checkout.session.completed` payload for `org_id=org-replay-20260312`:
   - first delivery returned `500 stripe_processing_failed` because the linked org did not exist yet
   - after adding the linked org metadata to the hosted persistence tree, replaying the exact same signed payload returned `200 {"received":true,"status":"processed"}`
7. Confirmed billing-admin state after the successful replay reflected the resulting subscription state:
   - `GET /api/admin/orgs/org-replay-20260312/billing-state` returned `subscription_state=active`
   - `plan_version=cloud_starter`
   - `limits.max_agents=10`
   - `stripe_customer_id=cus_retry_manual`
   - `stripe_subscription_id=sub_retry_manual`

## Outcome

- Hosted signup fails closed when the external public URL is missing.
- The self-hosted trial start path redirects to hosted checkout instead of minting a local entitlement immediately.
- Hosted public signup provisions an org and exposes coherent billing-admin trial state.
- Magic-link request flow remains available on the hosted public surface.
- Stripe webhook handling fails closed before org linkage exists and succeeds on replay once the linked org exists.
- Billing-admin state reflects the resulting trial and active subscription states coherently after both hosted signup and replayed checkout completion.

## Notes

- The exact replay rehearsal used a live localhost hosted-mode server, not test handlers, and replayed the same signed webhook payload before and after linked-org metadata existed.
- The linked org for the exact replay was inserted through the hosted persistence tree because the session-authenticated org-create route is separately guarded and not part of the public checkout/webhook path under test here.
