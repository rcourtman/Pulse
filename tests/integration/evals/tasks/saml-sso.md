# SAML SSO Provider Lifecycle

## Objective
Validate the full SAML SSO provider lifecycle: create a SAML provider,
verify it appears in the provider list, check SP metadata generation,
test-connection validation, enable/disable toggling, and cleanup via delete.

## Steps

1. Log in as admin.
2. GET `/api/security/sso/providers` — verify the endpoint responds with 200.
3. POST `/api/security/sso/providers` — create a SAML provider with stub IdP metadata.
4. GET `/api/security/sso/providers` — verify the new provider appears.
5. GET `/api/saml/{providerID}/metadata` — verify SP metadata XML is returned.
6. POST `/api/security/sso/providers/test` — validate the IdP metadata.
7. Navigate to `/settings/security-sso` — verify the SSO settings page renders.
8. PUT `/api/security/sso/providers/{id}` — disable the provider, verify disabled.
9. PUT `/api/security/sso/providers/{id}` — re-enable, verify enabled.
10. DELETE `/api/security/sso/providers/{id}` — delete and verify removal.

## Expected Outcomes
- Provider CRUD works end-to-end on the Community SSO capability.
- SSO settings page renders SSO-related content.

## Environment Variables
- `PULSE_E2E_SAML_IDP_METADATA_URL` — optional live IdP metadata URL.
