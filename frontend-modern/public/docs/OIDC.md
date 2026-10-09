# 🔐 OIDC Single Sign-On

Enable Single Sign-On (SSO) with providers like Authentik, Keycloak, Okta, and Microsoft Entra ID.

## 🚀 Quick Start

1.  **Configure Provider**: Create an OIDC application in your IdP.
    - **Redirect URI**: `https://<your-pulse-domain>/api/oidc/<provider-id>/callback`
    - **Scopes**: `openid`, `profile`, `email`
2.  **Enable in Pulse**: Go to **Settings → Security → Single Sign-On**.
3.  **Enter Details**:
    - **Issuer URL**: The base URL of your IdP (e.g., `https://auth.example.com/application/o/pulse/`).
    - **Client ID & Secret**: From your IdP.
4.  **Save**: The login page will now show your configured SSO provider button(s).

> **Tip**: To hide the username/password form and only show the SSO button, set `PULSE_AUTH_HIDE_LOCAL_LOGIN=true` in your environment. You can still access the local login by appending `?show_local=true` to the URL (e.g., `https://your-pulse-instance/?show_local=true`).

## ⚙️ Configuration

| Setting | Description |
| :--- | :--- |
| **Issuer URL** | The OIDC provider's issuer URL. Must match the `iss` claim in tokens. |
| **Client ID** | The application ID from your provider. |
| **Client Secret** | The application secret. |
| **Redirect URL** | Auto-detected. Override only if running behind a complex proxy setup. |
| **Scopes** | Space-separated scopes. Default: `openid profile email`. |
| **Claim Mapping** | Map `email`, `username`, and `groups` to specific token claims. |

> **Note**: Setting `OIDC_*` environment variables locks those fields in the UI. See [CONFIGURATION.md](CONFIGURATION.md) for the full list of overrides.

### Access Control
Restrict access to specific users or groups:
- **Allowed Groups**: Only users in these groups can login. Requires the `groups` scope/claim.
- **Allowed Domains**: Restrict to specific email domains (e.g., `example.com`).
- **Allowed Emails**: Allow specific email addresses.

> **Administrator requirement**: SSO authentication does not grant instance
> administrator privileges by itself. Before removing the configured local
> administrator or relying on SSO-only access, map a trusted IdP group to the
> built-in `admin` role. Keep that administrator group in **Allowed Groups** so
> the login and authorization boundaries describe the same trusted population.
> An empty **Allowed Groups** list allows every IdP user to sign in, but does not
> make those users administrators.

### Group-to-Role Mapping

Automatically assign Pulse roles based on OIDC group membership. When a user logs in, Pulse checks their groups claim and assigns the corresponding roles. Mapping groups to the built-in `admin`, `operator`, and `viewer` roles is included with Community SSO. Creating custom roles and manually managing user assignments remain Pro RBAC features.

**Configuration:**
Group-role mappings are configured per SSO provider through the UI (or the
SSO provider API for automated setup). Go to **Settings → Security → Single
Sign-On**, edit the provider, and populate **Group Role Mappings** with
entries like:
- `oidc-admins` → `admin`
- `oidc-operators` → `operator`
- `oidc-viewers` → `viewer`

The mappings persist on the provider record as a `groupRoleMappings` JSON
field. Provider-level config (including this field) can be PUT through the
SSO provider API for automated setup.

`OIDC_GROUP_ROLE_MAPPINGS` populates the same field, but only for the legacy
single-provider OIDC configuration built from `OIDC_*` environment variables —
it has no effect on providers created through the UI or the SSO provider API.
See [CONFIGURATION.md](CONFIGURATION.md).

**How it works:**
- On each login, Pulse reads the user's groups from the configured groups claim.
- For each group that matches a mapping, the corresponding role is assigned.
- Multiple groups can map to multiple roles (user gets all matching roles).
- Role assignments are updated on every login to reflect current group membership.
- Role changes are logged to the audit log for compliance tracking.
- Instance-administration routes require the built-in `admin` role or another
  role with an explicit `admin` grant on all resources. The `operator` and
  `viewer` mappings never inherit administrator access on SSO-only instances.

**Example:**
If a user has groups `["oidc-admins", "developers"]` and you have mappings:
- `oidc-admins` → `admin`
- `developers` → `operator`

The user will be assigned both `admin` and `operator` roles.

> **Note**: Ensure your IdP includes the `groups` scope and that the groups claim is properly configured. Some providers use `groups`, others use `roles` or custom claims.

### Sessions and `offline_access`

A Pulse browser session and the IdP's access and refresh tokens have different
lifetimes. Requesting `offline_access` does not guarantee a 30- or 90-day Pulse
login, and it is not an access-revocation mechanism.

- Request `offline_access` only when you need token refresh and your IdP
  supports it. The IdP must actually issue a refresh token; the scope alone is
  not enough. SSO can work without a refresh token.
- Pulse creates the browser cookies with a **24-hour lifetime**. Valid requests
  extend the server-side session's sliding expiry; this does not extend the
  browser cookie's expiry. A successful token refresh also extends the
  server-side session, not the browser cookie.
- While a valid session is being used, Pulse can start a background refresh near
  the access token's expiry. This is request-driven, not continuous polling of
  the IdP, and the triggering request can finish before refresh completes.
- A rejected or failed **token exchange** invalidates that Pulse session.
  Failure to initialise the provider, or no matching enabled provider, instead
  skips refresh. Do not assume that a discovery failure, disabled provider or
  changed issuer/client ID has signed existing users out.
- Refresh tokens are encrypted when persisted. Treat the Pulse data directory,
  encryption key and backups as sensitive; never publish session files.

#### Changing or removing access

**Allowed Groups**, **Allowed Domains** and **Allowed Emails** are checked at
login; every non-empty restriction must pass. Group-role mappings are also
applied at login. Refreshing a token does **not** re-evaluate those restrictions
or group-role mappings. Removing someone from an IdP group therefore does not
prove that their existing Pulse session or role assignments have been revoked.

Disabling or deleting a provider, or changing its issuer/client ID, likewise
must not be treated as revoking existing Pulse sessions. Keep a working,
independent administrator login before changing SSO. For planned access removal:

1. Block future sign-ins for the affected identity at the IdP or its application
   assignment, without changing access for unrelated users.
2. Where Pro RBAC user administration is available, use the documented
   [Remove user access](RBAC.md#removing-user-access) operation for the exact
   provider-scoped identity. It removes the role assignment and revokes that
   identity's active Pulse sessions; it does not disable the IdP account, and a
   later authorised SSO login can recreate the identity. Do not remove your own
   current administrator identity.
3. Check the result through the existing administrator session and, where
   available, the affected user's ordinary browser session. A successful save,
   hidden login button or blocked new login is not proof that an old session
   lost access. Keep session cookies out of commands and reports.

If targeted user removal is unavailable, do not assume there is a universal
session-revocation control on your plan. For urgent containment, use your
existing network or authenticated reverse-proxy access boundary, preserving an
administrator recovery path; blocking only the IdP login is not enough. Do not
disable Pulse authentication, delete session/configuration files or restart the
service as a substitute for verified revocation.

## 📚 Provider Examples

### Authentik
- **Type**: OAuth2/OpenID (Confidential)
- **Redirect URI**: `https://pulse.example.com/api/oidc/<provider-id>/callback`
- **Signing Key**: Must use **RS256** (create a certificate/key pair if needed).
- **Issuer URL**: `https://auth.example.com/application/o/pulse/`

### Keycloak
- **Client ID**: `pulse`
- **Access Type**: Confidential
- **Valid Redirect URIs**: `https://pulse.example.com/api/oidc/<provider-id>/callback`
- **Issuer URL**: `https://keycloak.example.com/realms/myrealm`

### Microsoft Entra ID (formerly Azure AD)

Create the provider in Pulse first (**Settings → Security → Single Sign-On → Add Provider**) so it gets its ID — Pulse generates a UUID per provider and shows the resulting callback URL. You need that URL for the Entra redirect URI below.

**In the Entra admin center:**

1. **App registrations → New registration**: give it a name (e.g. `Pulse SSO`) and choose *Accounts in this organizational directory only (Single tenant)*.
2. **Authentication → Add a platform → Web**: set the redirect URI to `https://pulse.example.com/api/oidc/<provider-id>/callback` and tick **ID tokens**.
3. **Certificates & secrets → New client secret**: copy the secret **Value** (not the Secret ID) — it is only shown once.
4. **Token configuration → Add groups claim**: select **Groups assigned to the application**, expand **ID**, and choose **Group ID**.
5. **Enterprise applications → (your app) → Properties**: set **Assignment required?** to `Yes`, so only assigned users and groups can sign in.
6. **Enterprise applications → (your app) → Users and groups**: assign the security group (e.g. `Pulse-Admins`) and copy its **Object ID** — a GUID like `a1b2c3d4-e5f6-7890-abcd-123456789abc`.

**In Pulse:**

- **Issuer URL**: `https://login.microsoftonline.com/<tenant-id>/v2.0`
- **Client ID**: the app registration's *Application (client) ID*.
- **Client Secret**: the secret value from step 3.
- **Redirect URI**: `https://pulse.example.com/api/oidc/<provider-id>/callback`, matching the app registration exactly. The bare `/api/oidc/callback` is a v5 compatibility path that only serves the legacy env-configured provider — don't use it for a new provider.
- **Scopes**: exactly `openid profile email`. Do **not** add `groups`. Entra has no `groups` scope and fails the whole authorization request with `AADSTS650053`; group membership arrives in the ID token from the Token configuration step, not from a scope.
- **Groups Claim**: `groups`
- **Allowed Groups**: the group's Object ID (GUID), not its display name.
- **Group Role Mappings**: `<guid>=admin`. Keying on the Object ID means the mapping survives a group rename in Entra.

> **Warning — group overage**: if a user belongs to more groups than Entra will fit in a token, Entra omits the `groups` claim entirely and sends a `_claim_names` / `_claim_sources` overage marker pointing at Microsoft Graph instead. Pulse does not follow that marker, so it sees the user as having no groups — and because a configured group-role mapping is authoritative, that login **clears** the user's role assignments instead of leaving them alone. Selecting **Groups assigned to the application** rather than **Security groups** in Token configuration keeps the claim small and avoids the overage. If the token still carries every security group after that, check the app registration **Manifest**: `groupMembershipClaims` must be exactly `"ApplicationGroup"`. A value like `"SecurityGroup, ApplicationGroup"` (left over from an earlier Token configuration choice) keeps emitting all security groups no matter what **Assignment required** is set to, so edit the manifest to drop `SecurityGroup`.

> **Note**: Plain SSO login, **Allowed Groups** gating, and group mapping to built-in roles work on every plan. Creating custom roles and manually managing user assignments require Pro RBAC.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **`invalid_id_token`** | Compare the configured issuer with the IdP issuer locally; use the existing redacted error, not a token dump or newly enabled Debug logs. |
| **`unexpected signature algorithm "HS256"`** | Your IdP is signing with HS256. Configure it to use **RS256**. |
| **Redirect Loop** | Check `X-Forwarded-Proto` header (must be `https`) and cookie settings. |
| **Self-Signed Certs** | Set the **CA Bundle** field on the SSO provider to a host path readable by Pulse (e.g. `/etc/ssl/certs/oidc-ca.pem` mounted into the container). The field is stored on the provider record as `oidc.caBundle`; there is no `OIDC_CA_BUNDLE` env var. |

### Safe diagnosis

Start with the existing login error, its time, the provider type and whether the
failure is at the IdP, callback, login restriction or Pulse permission check.
Use the configured callback URL and claim names locally; do not paste a full
IdP response or token to diagnose a missing group. A working login does not
prove that the user has the intended Pulse role.

Review existing server and IdP logs privately. Do not enable Debug or restart
Pulse solely to fill in a report; debug logs can contain identity and claim
details. Share only a locally reviewed, redacted error and the relevant claim
names or consistent group aliases. Keep client secrets, authorization codes,
ID/access/refresh tokens, cookies, email addresses, full callback query strings,
HAR exports and “Copy as cURL” output private. Never disable TLS verification or
access restrictions as a diagnostic shortcut.
