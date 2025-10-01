# OpenID Connect (OIDC) Single Sign-On

Pulse ships with first-class OIDC support so you can authenticate through identity providers such as Authentik, Keycloak, Okta, Azure AD, and others.

## Requirements

- Pulse v4.16.0 or later
- A reachable Pulse public URL (used for redirect callbacks)
- An OIDC client registration on your IdP with the redirect URI:
  ```
  https://<pulse-host>/api/oidc/callback
  ```
- Scopes that include at least `openid`

## Quick Start

1. Open **Settings → Security → Single sign-on (OIDC)**.
2. Toggle **Enable** and fill in the following fields:
   - **Issuer URL** – Base issuer endpoint from your IdP metadata document.
   - **Client ID** and **Client secret** – From your IdP application registration.
   - **Redirect URL** – Optional. Pulse auto-populates this based on the public URL; override only when necessary.
3. Use the **Advanced options** section to customise scopes, claim names, or access restrictions by group, domain, or email.
4. Click **Save changes**. After a successful save the OIDC login button appears on the Pulse sign-in page.

### Using the bundled mock IdP

For local end-to-end testing Pulse ships with a Dex-based mock server configuration. With Docker running:

```bash
./scripts/dev/start-oidc-mock.sh
```

This exposes an issuer at `http://127.0.0.1:5556/dex` with:

- Client ID: `pulse-dev`
- Client secret: `pulse-secret`
- Redirect URIs: `http://127.0.0.1:5173/api/oidc/callback`, `http://127.0.0.1:7655/api/oidc/callback`
- Test user: `admin@example.com` / `password`

Point the OIDC settings screen at that issuer, save, and use the SSO button to exercise the full login flow.

## Classic password login stays

OIDC is optional. Pulse continues to ship with the familiar username/password flow:

- First-run setup still prompts you to create an admin credential or you can pre-seed it via `PULSE_AUTH_USER` / `PULSE_AUTH_PASS`.
- If OIDC is **enabled**, the login page shows both the password form and the **Continue with Single Sign-On** button. Either path issues the same session cookie (`pulse_session`).
- To run **password-only**, leave OIDC disabled (the default). To go **OIDC-only**, set `DISABLE_AUTH=true` after you confirm SSO works.
- The `allowedGroups`, `allowedDomains`, and `allowedEmails` settings only affect OIDC logins; password authentication continues to honour the account you created locally.

## Provider Cheat-Sheet

You do not need to ship per-provider templates. Pulse speaks standard OIDC, so administrators bring their own identity provider and supply the issuer URL, client ID, and client secret they created for Pulse. Below are the high-level steps we tested against three common providers—share these with users who ask “what do I enter?”

### Authentik

1. In Authentik, create a new **Provider** of type **OAuth2/OpenID**.
   - **Name**: Pulse
   - **Client type**: Confidential
   - **Redirect URIs**: `https://pulse.example.com/api/oidc/callback` (replace with your Pulse URL)
   - **Scopes**: Include `openid`, `profile`, and `email`
   - Note the generated **Client ID** and **Client Secret**

2. Create an **Application** and link it to the provider you just created.

3. In Pulse, configure OIDC with:
   - **Issuer URL**: `https://auth.example.com/application/o/pulse/` (the full path to your application)
   - **Client ID**: The client ID from your Authentik provider
   - **Client Secret**: The client secret from your Authentik provider
   - **Scopes**: `openid profile email`

4. In Authentik, open **Applications → [your Pulse app] → Advanced** and set a **Signing Key** that advertises the `RS256` algorithm (generate or assign an RSA key). Authentik defaults to `HS256` when no signing key is configured, which Pulse rejects with the error `unexpected signature algorithm "HS256"; expected ["RS256"]`.

5. For group-based access control:
   - Set **Groups claim** to `groups` (Authentik's default)
   - Add your allowed group names to **Allowed groups** (e.g., `admin`)

**Important**: If you see "invalid_id_token" errors, the issuer URL might not match what Authentik puts in tokens. Check your Pulse logs with `LOG_LEVEL=debug` to see the exact error. The issuer claim in the token must match your configured `OIDC_ISSUER_URL` exactly.

### Dex / other self-hosted issuers

This matches the bundled Dex mock server:

1. Create a new OAuth 2.0 / OIDC application, mark it *confidential*, and note the generated `client_id` and `client_secret`.
2. Add every Pulse hostname you expose (for example `https://pulse.example.com/api/oidc/callback`) to the list of redirect URIs.
3. Ensure the application scopes include at least `openid profile email`.
4. Paste the issuer URL (for Dex that is `https://<issuer-host>/dex`) plus the client credentials into Pulse.

### Azure Active Directory

1. Register a **Web** app in Azure AD and capture the **Application (client) ID**.
2. Create a **Client secret** under *Certificates & secrets*.
3. Add a redirect URL `https://<pulse-host>/api/oidc/callback` (type **Web**).
4. Under *Token configuration*, add optional claims for `email` and `preferred_username`. If you plan to restrict by groups enable the *Groups* claim.
5. In Pulse, use `https://login.microsoftonline.com/<tenant-id>/v2.0` as the issuer and paste the client credentials.

### Okta

1. Create an **OIDC Web App** integration.
2. Trusted redirect URIs: `https://<pulse-host>/api/oidc/callback`.
3. Assign the integration to the users or groups who need access.
4. Copy the **Client ID**, **Client secret**, and **Okta domain**. Use `https://<your-okta-domain>/oauth2/default` as the issuer within Pulse.

### Group and domain restrictions

- Set `OIDC_GROUPS_CLAIM` to the claim that carries group names (default `groups`).
- Combine `allowedGroups`, `allowedDomains`, or `allowedEmails` in the UI to fence access without editing your IdP.
- Azure AD group names appear as GUIDs unless you enable *Security groups* in token configuration; Okta and Authentik emit the literal group name.

## Environment Overrides

All configuration can be provided via environment variables (see [`docs/CONFIGURATION.md`](./CONFIGURATION.md#oidc-variables-optional-overrides)). When any `OIDC_*` variable is present the UI is placed in read-only mode and values must be changed from the deployment configuration instead.

## Login Flow

- The login screen shows a **Continue with Single Sign-On** button when OIDC is enabled.
- Users are redirected to the configured issuer for authentication and returned to `/api/oidc/callback`.
- Pulse validates the ID token, enforces optional group/domain/email restrictions, then creates the usual session cookie (`pulse_session`).
- Existing username/password login remains available unless explicitly disabled in the environment.

## Troubleshooting

| Symptom | Resolution |
| --- | --- |
| `invalid_id_token` error | The issuer URL configured in Pulse doesn't match the `iss` claim in the ID token from your provider. Enable `LOG_LEVEL=debug` to see the exact verification error. For Authentik, try both `https://auth.domain.com` (base URL) and `https://auth.domain.com/application/o/pulse/` (application URL) to see which matches your provider's token issuer. |
| `unexpected signature algorithm "HS256"; expected ["RS256"]` in logs | Authentik falls back to HS256 if no signing key is configured. Assign an RSA signing key to the application (token settings → Signing key) so ID tokens are issued with RS256. |
| Redirect loops back to login | After successful OIDC login, if you're redirected back to the login page, check that: (1) cookies are enabled in your browser, (2) if behind a proxy, ensure `X-Forwarded-Proto` header is set correctly, (3) check browser console for cookie errors. |
| Users see `single sign-on failed` | Check `journalctl -u pulse.service` for detailed OIDC audit logs. Common causes include mismatched client IDs, incorrect redirect URLs, or group/domain restrictions. |
| UI shows "OIDC settings are managed by environment variables" | Remove the relevant `OIDC_*` environment variables or update them directly in your deployment. |
| Provider discovery fails | Verify the issuer URL is reachable from the Pulse server and returns valid OIDC discovery metadata at `/.well-known/openid-configuration`. |
| Group restrictions not working | Enable debug logging to see which groups the IdP is sending and verify the `groups_claim` setting matches your IdP's claim name. |
| Auto-redirect to OIDC when password auth still enabled | This is expected behavior when OIDC is enabled. Users can still use password auth by clicking "Use your admin credentials to sign in below" on the login page. To disable auto-redirect, comment out the auto-redirect code in the frontend. |

### Debug Logging

For detailed troubleshooting, set `LOG_LEVEL=debug` in your deployment and restart Pulse. Debug logs include:

- OIDC provider initialization (issuer URL, endpoints discovered)
- Authorization flow start (client ID, scopes requested)
- Token exchange details (success/failure with specific errors)
- ID token verification (subject extracted)
- Claims extraction (username, email, groups found)
- Access control checks (which emails/domains/groups were checked and why they passed or failed)

Example debug log output:
```
DBG Initializing OIDC provider issuer=https://auth.example.com redirect_url=https://pulse.example.com/api/oidc/callback scopes=[openid,profile,email]
DBG OIDC provider discovery successful issuer=https://auth.example.com auth_endpoint=https://auth.example.com/authorize token_endpoint=https://auth.example.com/token
DBG Starting OIDC login flow issuer=https://auth.example.com client_id=pulse-client
DBG Processing OIDC callback issuer=https://auth.example.com
DBG OIDC code exchange successful
DBG ID token verified successfully subject=user@example.com
DBG Extracted user identity from claims username=user@example.com email=user@example.com email_claim=email username_claim=preferred_username
DBG Checking group membership user_groups=[admins,users] allowed_groups=[admins] groups_claim=groups
DBG User group membership verified
```

After reviewing the logs, set `LOG_LEVEL=info` to reduce log volume.
