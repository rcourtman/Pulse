# 🔐 OIDC Single Sign-On

Enable Single Sign-On (SSO) with providers like Authentik, Keycloak, Okta, and Microsoft Entra ID.

## 🚀 Quick Start

Keep a working, independent administrator login and its browser session while
setting up SSO. Saving a provider or seeing its login button is not proof of
successful sign-in or administrator access.

1. **Prepare the IdP application.** Obtain its issuer URL, client ID and client
   secret through your IdP's normal registration flow. Enter the secret only in
   Pulse's HTTPS settings form or private deployment configuration, never in a
   command, URL or report. Start with `openid profile email`; follow the
   [provider examples](#-provider-examples) for group claims and any additional
   supported scope, rather than adding `groups` universally.
2. **Configure access before saving.** In **Settings → Security → Single
   Sign-On**, add an OIDC provider and enter those details. Select **Show access
   restrictions & role mapping** and set the intended login restrictions and
   **Group Role Mappings**, including a trusted group mapped to `admin` if
   administrators will use SSO. Check the [access rules](#access-control) and
   [role mappings](#group-to-role-mapping) first; blank login restrictions do
   not limit sign-ins to administrators. New UI providers are enabled on save.
3. **Save, then register the actual callback.** Copy **Callback / Redirect
   URL** from the saved provider card into the IdP's redirect allowlist. Pulse
   generates the provider ID when it is saved; do not register a guessed ID or
   the literal `<provider-id>` placeholder. Use the exact public HTTPS URL,
   not a backend address or wildcard. If the displayed URL is wrong, correct
   the [HTTPS/proxy configuration](REVERSE_PROXY.md#before-configuring-the-proxy)
   before signing in, without disabling certificate or origin checks.
4. **Verify without replacing the working login.** In a separate browser
   session, sign in as an intended administrator and check administrator controls
   in Security settings. Where you already have suitable users, also check that
   an intended non-admin cannot use administrator controls and that an excluded
   user is refused. Do not change
   settings to test privileges. A successful provider **Test** checks discovery,
   not the callback, user claims or resulting permissions. Keep the independent
   administrator path until these checks pass; do not clear restrictions to
   make a failed login work. Use [safe diagnosis](#safe-diagnosis).

Only after verifying SSO, optionally hide the local username/password form with
`PULSE_AUTH_HIDE_LOCAL_LOGIN=true` through your normal deployment configuration.
Appending `?show_local=true` to your Pulse URL shows that form again; it does
not create or restore local credentials, bypass the proxy or revoke SSO sessions.

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
- **Allowed Groups**: Only users with a matching group in the configured claim can login. Configure the IdP to include that claim; a `groups` scope is not universally supported.
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

> **Note**: Pulse reads the configured groups claim, not a scope name. Some
> providers use `groups`, others use `roles` or custom claims. Request an extra
> scope only when that IdP supports and requires it to emit the claim. Microsoft
> Entra ID has no `groups` scope; use its token configuration below. The scope
> alone neither supplies group membership nor grants a Pulse role.

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
- **Redirect URI**: the exact **Callback / Redirect URL** copied from the saved Pulse provider card.
- **Signing Key**: Must use **RS256** (create a certificate/key pair if needed).
- **Issuer URL**: `https://auth.example.com/application/o/pulse/`

### Keycloak
- **Client ID**: `pulse`
- **Access Type**: Confidential
- **Valid Redirect URIs**: the exact **Callback / Redirect URL** copied from the saved Pulse provider card, without a wildcard.
- **Issuer URL**: `https://keycloak.example.com/realms/myrealm`

### Microsoft Entra ID (formerly Azure AD)

Prepare the Entra app registration and client credentials first, then save the
Pulse provider with its access restrictions as in [Quick Start](#-quick-start).
Finally register the callback copied from that saved provider card. Do not use
dummy client details or a guessed provider ID to break the setup order.

**In the Entra admin center:**

1. **App registrations → New registration**: give it a name (e.g. `Pulse SSO`) and choose *Accounts in this organizational directory only (Single tenant)*.
2. **Certificates & secrets → New client secret**: copy the secret **Value** (not the Secret ID) — it is only shown once. Keep it private for Pulse's HTTPS form.
3. **Token configuration → Add groups claim**: select **Groups assigned to the application**, expand **ID**, and choose **Group ID**.
4. **Enterprise applications → (your app) → Properties**: set **Assignment required?** to `Yes`, so only assigned users and groups can sign in.
5. **Enterprise applications → (your app) → Users and groups**: assign the security group (e.g. `Pulse-Admins`) and copy its **Object ID** — a GUID like `a1b2c3d4-e5f6-7890-abcd-123456789abc`.

**In Pulse:**

- **Issuer URL**: `https://login.microsoftonline.com/<tenant-id>/v2.0`
- **Client ID**: the app registration's *Application (client) ID*.
- **Client Secret**: the secret value from step 2.
- **Redirect URL**: normally leave the override empty; save to obtain the actual callback from the provider card. The bare `/api/oidc/callback` is a v5 compatibility path. It is mapped to the legacy (v5-migrated or env-configured) provider; if that provider is not enabled, Pulse instead serves the provider whose login started from it (recovered from the pending login state). Don't use it as the redirect URI for a new provider.
- **Scopes**: exactly `openid profile email`. Do **not** add `groups`. Entra has no `groups` scope and fails the whole authorization request with `AADSTS650053`; group membership arrives in the ID token from the Token configuration step, not from a scope.
- **Groups Claim**: `groups`
- **Allowed Groups**: the group's Object ID (GUID), not its display name.
- **Group Role Mappings**: `<guid>=admin`. Keying on the Object ID means the mapping survives a group rename in Entra.

**Complete the redirect registration before signing in:** In Entra,
**Authentication → Add a platform → Web**, paste the exact **Callback / Redirect
URL** from the saved Pulse card and tick **ID tokens**. It must match exactly;
do not use a wildcard, guessed ID or compatibility callback. Then follow the
separate-session login and permission checks in Quick Start.

> **Warning — group overage**: if a user belongs to more groups than Entra will fit in a token, Entra omits the `groups` claim entirely and sends a `_claim_names` / `_claim_sources` overage marker pointing at Microsoft Graph instead. Pulse does not follow that marker, so it sees the user as having no groups — and because a configured group-role mapping is authoritative, that login **clears** the user's role assignments instead of leaving them alone. Selecting **Groups assigned to the application** rather than **Security groups** in Token configuration keeps the claim small and avoids the overage. If the token still carries every security group after that, check the app registration **Manifest**: `groupMembershipClaims` must be exactly `"ApplicationGroup"`. A value like `"SecurityGroup, ApplicationGroup"` (left over from an earlier Token configuration choice) keeps emitting all security groups no matter what **Assignment required** is set to, so edit the manifest to drop `SecurityGroup`.

> **Note**: Plain SSO login, **Allowed Groups** gating, and group mapping to built-in roles work on every plan. Creating custom roles and manually managing user assignments require Pro RBAC.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **`invalid_id_token`** | Compare the configured issuer with the IdP issuer locally; use the existing redacted error, not a token dump or newly enabled Debug logs. |
| **`unexpected signature algorithm "HS256"`** | Your IdP is signing with HS256. Configure it to use **RS256**. |
| **Redirect Loop** | Check `X-Forwarded-Proto` header (must be `https`) and cookie settings. |
| **Self-Signed Certs** | The SSO provider form in Settings has no CA Bundle field. Set `oidc.caBundle` on the provider through the SSO provider API (`/api/security/sso/providers`) to a host path readable by Pulse (e.g. `/etc/ssl/certs/oidc-ca.pem` mounted into the container). For a provider configured with the legacy `OIDC_ENABLED` environment variables, set `OIDC_CA_BUNDLE` instead (see [Configuration](CONFIGURATION.md)). |

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
