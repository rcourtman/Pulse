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

## Provider Cheat-Sheet

You do not need to ship per-provider templates. Pulse speaks standard OIDC, so administrators bring their own identity provider and supply the issuer URL, client ID, and client secret they created for Pulse. Below are the high-level steps we tested against three common providers—share these with users who ask “what do I enter?”

### Authentik / Dex / other self-hosted issuers

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
| Users see `single sign-on failed` | Check `journalctl -u pulse-dev-hot.service` (or `pulse.service`) for detailed OIDC audit logs. Common causes include mismatched client IDs, incorrect redirect URLs, or group/domain restrictions. |
| Redirect loops back to login | Ensure clock skew between Pulse and the IdP is <5 minutes. Verify the redirect URL is reachable from the user’s browser. |
| UI shows "OIDC settings are managed by environment variables" | Remove the relevant `OIDC_*` environment variables or update them directly in your deployment. |

For advanced debugging, temporarily set `LOG_LEVEL=debug` and check the backend logs for `oidc_login` audit events.
