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

**Hiding the form is not SSO-only enforcement.** Local password authentication
remains enabled: valid existing local credentials can still sign in, including
through the local login endpoint. The visibility setting does not reject those
requests or revoke local sessions. Keep the independent administrator recovery
path; do not remove credentials or disable authentication to simulate an
OIDC-only policy. Login restrictions and group-role mappings below apply to SSO
sign-ins, not to the separate local password login.

## ⚙️ Configuration

| Setting | Description |
| :--- | :--- |
| **Issuer URL** | The OIDC provider's issuer URL. Must match the `iss` claim in tokens. |
| **Client ID** | The application ID from your provider. |
| **Client Secret** | The application secret. |
| **Redirect URL** | Auto-detected. Override only if running behind a complex proxy setup. |
| **Scopes** | Space-separated scopes. Default: `openid profile email`. Enabled providers with custom scopes must include the exact `openid` token. |
| **Claim Mapping** | Map `email`, `username`, and `groups` to specific token claims. |

When creating an enabled OIDC provider, enabling a disabled provider, or changing custom **Scopes** on an enabled provider, Pulse requires the exact `openid` token and rejects the write (`Include openid in Scopes, or clear Scopes to use the defaults. Nothing was saved.`) if it is omitted. Leaving **Scopes** blank uses the defaults. Disabled drafts and unchanged scopes on already-enabled providers remain editable.

> **Note**: `OIDC_*` does not override saved providers. Read [configuration sources](#configuration-sources) before editing a deployment-managed provider.

### Access Control
At sign-in, every non-empty list must match. Entries within one list are alternatives:
- **Allowed Groups**: Match a group in the configured claim. The IdP must supply it; a `groups` scope is not universally supported. An empty group list skips only that check, and any domain and email restrictions still apply.
- **Allowed Domains**: Match the email domain (e.g., `example.com`).
- **Allowed Emails**: Match an email address. This does not bypass the other lists.

> **Administrator access:** Passing the sign-in rules does not grant instance
> administrator privileges by itself. For SSO administrators, map a trusted
> IdP group to the built-in `admin` role in **Group Role Mappings**.
> [Check admin access in a separate browser session](#-quick-start) and keep
> your independent administrator login even after SSO works.

An address the IdP reports as unverified (`email_verified: false`) matches neither **Allowed Domains** nor **Allowed Emails**. Sign-in fails with `email_unverified` until the user verifies it at the IdP. An IdP that omits `email_verified` is not affected.

In **Allowed Groups**, separate entries with commas or new lines. Spaces inside a
group name are preserved. Wrap any exact directory group name containing commas
or `=` in double quotes (for example, `"CN=Pulse Admins,OU=Groups,DC=example,DC=com"`)
so punctuation inside the name is not split. Leaving a restriction field blank
clears it on save. Separators alone or unmatched quotes are rejected before saving.

### Group-to-Role Mapping

Automatically assign Pulse roles based on OIDC group membership. When a user logs in, Pulse checks their groups claim and assigns the corresponding roles. Mapping groups to the built-in `admin`, `operator`, and `viewer` roles is included with Community SSO. Creating custom roles and manually managing user assignments remain Pro RBAC features.

**Configuration:**
Group-role mappings are configured per SSO provider through the UI (or the
SSO provider API for automated setup). Go to **Settings → Security → Single
Sign-On**, edit the provider, and populate **Group Role Mappings** with
`group=roleId` pairs separated by commas or new lines, such as:
- `oidc-admins` → `admin`
- `oidc-operators` → `operator`
- `oidc-viewers` → `viewer`

Wrap exact directory group names containing commas or `=` in double quotes (for
example, `"Ops, East"=viewer`). Duplicate groups or incomplete `group=roleId`
pairs are rejected before sending a save. Leaving the field blank clears saved
mappings.

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
- If a configured mapping references a role that no longer exists, Pulse replaces the user's saved roles with only the mapped roles that still exist rather than leaving earlier grants in place. If Pulse cannot replace the saved roles, sign-in is refused (`session_failed`) instead of admitting the user with stale roles.
- Clearing all **Group Role Mappings** on a provider withdraws roles that an earlier mapping saved on the user's next sign-in, while preserving roles assigned manually by an administrator.
- If an administrator edits or disables a provider while a sign-in callback is in flight, Pulse refuses the stale callback (`invalid_state`) rather than rewriting roles or opening a session under the older policy.
- Successful non-empty assignments get an audit event. Clearing roles does not.
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
independent administrator login before changing SSO.

Each SSO provider binds to its original verified issuer (the OIDC **Issuer
URL** or SAML **IdP Entity ID**), so existing users keep their saved identities
across upgrades. Editing an existing provider to point at a different issuer,
or changing or deleting a provider whose original issuer cannot be resolved,
causes future logins on that provider ID to use separate issuer-scoped
principals rather than inheriting saved roles from the previous identity
provider. Legacy assignments keyed by email address are never migrated from an
unverified OIDC address (`email_verified: false`).

For planned access removal:

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

## SAML 2.0 Single Sign-On

SAML 2.0 is included on every plan alongside OIDC and supports multiple saved providers under **Settings → Security → Single Sign-On**. Keep a working, independent administrator login open throughout setup.

### SAML setup and endpoints

1. **Choose the IdP metadata source.** Add a SAML provider and supply **IdP Metadata URL** (preferred), raw IdP metadata XML, or manual **IdP SSO URL**, **IdP Entity ID**, and **IdP Certificate (PEM)**.
2. **Configure access and attributes before saving.** Set **Username Attribute** (defaults to `NameID`), **Email Attribute**, and **Groups Attribute** (Pulse matches either the SAML attribute `Name` or `FriendlyName`). Open **Show access restrictions & role mapping** to set **Allowed Groups**, **Allowed Domains**, **Allowed Emails**, and **Group Role Mappings**, including a trusted admin group mapped to `admin` if administrators will sign in through SAML.
3. **Save, then register Pulse in the IdP.** Saving generates the provider ID and exposes the **SP Metadata** (`/api/saml/{id}/metadata`, `GET`, `application/xml`) and **ACS URL** (`/api/saml/{id}/acs`, `POST`) on the saved provider card. Copy those exact HTTPS URLs into the IdP rather than guessing a provider ID. Pulse's default SP Entity ID is `<public-url>/saml/{id}` unless overridden (`spEntityId`), and SP-initiated sign-in starts at `/api/saml/{id}/login` (`GET` redirects to the IdP, and `POST` returns `authorizationUrl`).
4. **Understand what SAML Test checks.** **Check SAML metadata** (with a metadata URL or XML) verifies that the metadata parses and contains an IdP Entity ID and usable SSO URL, and reports whether an X.509 signing certificate was read. With manual settings (**Check the IdP SSO URL format only**), Pulse checks only URL syntax. Neither verifies assertion signatures end-to-end or completes a sign-in: verify administrator, non-admin, and excluded users in a separate browser session.

Optional signed authentication requests (`signRequests`) require both an SP certificate and an RSA private key (PKCS#1 or PKCS#8 PEM, inline or file path).

### SAML browser binding, sessions, and Single Logout

- **SP-initiated login binding:** Each SP-initiated `AuthnRequest` is valid for **10 minutes** (up to 1,024 outstanding entries per provider) and is bound to a browser-only HttpOnly login cookie (`__Host-pulse_saml_login` over HTTPS, `pulse_saml_login` over HTTP). Pulse accepts each signed IdP response at most once, only from the browser that started the login, and returns to the local path recorded when login began.
- **SameSite repost on ACS:** Browsers withhold `SameSite=Lax` cookies on cross-site `POST` requests from the IdP to `/api/saml/{id}/acs`. When a SAML response arrives without the login cookie on an SP-initiated provider, Pulse returns a one-time same-origin repost form so the browser re-submits the assertion with its cookie. If cookies are blocked or no login started in that browser, ACS refuses the login with `saml_validation_failed`.
- **Keep IdP-initiated SSO off by default:** **Allow IdP-initiated SSO** (`allowIdpInitiated`) is disabled by default because unsolicited assertions answer no `AuthnRequest` and bypass `InResponseTo` and browser binding. Enable it only when an IdP workflow specifically requires it.
- **Sessions and access boundaries:** SAML creates the same 24-hour browser session cookie as OIDC, binds each user's stable principal to the assertion's verified `Issuer` / IdP Entity ID and `NameID` (an assertion without `NameID` fails with `missing_subject`), enforces all non-empty **Allowed Groups**, **Allowed Domains**, and **Allowed Emails** rules, writes `saml_role_assignment` audit events only when at least one role is mapped, and rejects in-flight callbacks with `invalid_state` if the provider is edited or disabled before completion.
- **Single Logout (SLO):** SP-initiated logout (`POST /api/saml/{id}/logout`) requires an active browser session, clears the local Pulse session first, and redirects to the IdP's SLO endpoint when present in metadata (or to `/?logout=success` otherwise). `/api/saml/{id}/slo` validates only a signed `LogoutResponse` answering an outstanding SP-initiated `LogoutRequest` from that session. **IdP-initiated Single Logout** (`SAMLRequest` sent to `/slo`) is not supported and returns HTTP 400 (`IdP-initiated logout not supported`) without clearing sessions.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **`invalid_id_token`** | Compare the configured issuer with the IdP issuer locally; use the existing redacted error, not a token dump or newly enabled Debug logs. |
| **`unexpected signature algorithm "HS256"`** | Your IdP is signing with HS256. Configure it to use **RS256**. |
| **Redirect Loop** | [Check the callback address](#check-the-callback-address) before changing proxy or cookie settings. |
| **Self-Signed Certs** | The SSO provider form in Settings has no CA Bundle field. Set `oidc.caBundle` on the provider through the SSO provider API (`/api/security/sso/providers`) to a host path readable by Pulse (e.g. `/etc/ssl/certs/oidc-ca.pem` mounted into the container). For a provider configured with the legacy `OIDC_ENABLED` environment variables, set `OIDC_CA_BUNDLE` instead (see [Configuration](CONFIGURATION.md)). |
| **`saml_validation_failed`** | The SAML response failed signature/assertion checks, expired past the 10-minute login window, was replayed, or arrived without the browser's login cookie (including when cookies or JavaScript are blocked on the same-origin ACS repost). Start a fresh SP-initiated login from Pulse in the same browser and check the IdP signing certificate and clock skew locally. |
| **`missing_subject`** | The SAML assertion omitted `NameID`. Configure the IdP to send a stable `NameID` subject for the user. |
| **`provider_not_initialized`** | Pulse could not initialize the SAML service from its saved metadata URL/XML or SP signing credentials. Check metadata reachability, XML validity, and any SP certificate/key paths privately. |

### Safe diagnosis

Back at Pulse sign-in after SSO? Your session is unconfirmed. **Retry sign-in
check**, if shown, only reads the sign-in options. Keep your working admin
session open and do not resave the provider. Use the existing login error,
time and provider type to check the IdP, callback or login restriction. If
Pulse opens without the expected controls, check the role.

Check existing logs, callback URLs and claim names locally. Do not paste a
full IdP response. Do not enable Debug or restart for a report. Debug logs
can expose identities and claims. Share only a locally reviewed, redacted
error, claim names or consistent group aliases. Keep client secrets,
authorization codes, ID/access/refresh tokens, cookies, email addresses, full
callback query strings, HAR exports and “Copy as cURL” output private. Never
disable TLS verification or access restrictions.

### A provider change could not be confirmed

A connection failure or server error after **Save**, an enable/disable change
or **Delete** can happen after Pulse applied it. Do not repeat the change or
add another provider to clear the warning. Keep your working, independent
administrator session open.

In **Settings → Security → Single Sign-On**, use **Check saved providers**
when offered. This discards the open draft, including any typed secret, and
reads the provider list without repeating the change or running a connection
test. On versions without that button, reopen the panel for a fresh read before
considering another write. Do not bypass the warning through the API, edit
configuration files or restart Pulse to clear it.

Compare the returned list with what you intended. If the provider is present,
use that card's **Callback / Redirect URL**, not a duplicate provider. Open
**Edit** without saving to review its login restrictions and role mappings. If
opening **Edit** shows **Could not open settings. Try Edit again.** beside that
provider's card, the read failed without submitting any change. Select **Edit**
again to retry only that read.
The read shows a snapshot at that time, not proof that an earlier request cannot
still finish. A failed read means unknown, not empty or unchanged.

**If a provider you know existed is unexpectedly missing, stop without adding
or deleting anything.** A configuration-load failure can look like an empty
list. Keep the existing data and administrator access. For help, use the
original action, time, Pulse version and relevant, locally redacted existing
error. Do not repeat the action to obtain evidence.

A returned provider or masked secret field does not confirm which client
secret is stored. It also does not prove working sign-in, intended permissions
or revoked sessions. Once the intended settings are confirmed, use the
[separate-session checks in Quick Start](#-quick-start) without signing out of
your working administrator session. Keep secrets, cookies and full responses
private.

### Configuration sources

Use the source that already owns the affected provider, not a replacement:

- **Saved provider:** edit its existing entry in **Settings → Security → Single
  Sign-On**. The `OIDC_*` variables do not override a saved provider, including
  its group-role mappings and CA bundle. Setting `OIDC_ENABLED=false` does not
  disable a saved provider.
- **Deployment-managed provider:** `OIDC_ENABLED=true`, with the required issuer
  and client configuration, can supply the legacy provider. An edit, toggle or
  deletion returns **Provider is managed by OIDC environment variables**
  (`provider_managed_by_environment`, HTTP 409). An editable-looking form does
  not mean a save can override that source. Use the existing private deployment
  configuration instead. The [variable list](CONFIGURATION.md) applies to this
  provider, not to every provider in Settings.
- **Migrated provider:** an older saved OIDC configuration can also have the ID
  `legacy-oidc`. If that ID already has a saved provider, the persisted record
  takes precedence over the environment provider. Its name or ID alone does
  not establish that it is deployment-managed. Do not delete it, add a duplicate
  or remove configuration files to make environment settings take effect.

Change only the active private configuration source during a suitable
maintenance window. Preserve the provider identity, registered callback, access
restrictions, credentials and data. Keep an independent administrator recovery
path. Apply a managed environment change through the existing deployment:
Docker needs recreation/redeployment, not just a restart. Never put the client
secret in a command or report. Verify sign-in and permissions in a separate
browser session as in [Quick Start](#-quick-start).

Changing either source does not revoke existing sessions. Follow
[Changing or removing access](#changing-or-removing-access) for that separate job.

### `invalid_client` after saving an OIDC provider

If the existing Pulse log shows `invalid_client` with **OIDC code exchange
failed**, the identity provider rejected Pulse's client authentication. That
does not prove Pulse lost the secret. Compare the saved **Client ID** with the
app registration. In the identity provider, check whether the app's existing
secret is still active. The masked field in Pulse does not show which secret is
saved.

Keep your working administrator session open. If Pulse could not confirm the
save, [check the saved provider](#a-provider-change-could-not-be-confirmed)
before writing anything. If the failure began after a save, keep the time,
the setting you changed (or that nothing changed) and the locally redacted
error. Do not repeat the save, rotate the secret or sign out to diagnose it.

### Check the callback address

Keep your working administrator session open. On your usual HTTPS Pulse
address, open **Settings → Security → Single Sign-On** and compare the saved
provider's **Callback / Redirect URL** with the redirect URL registered in
the identity provider. Match the whole address, including the scheme, host,
port if shown and provider-specific path. If only the registration is wrong,
correct it in the identity provider. There is no need to save the Pulse
provider again.

If Pulse shows the wrong address, open the existing provider without saving
and check **Redirect URL**. A saved override takes precedence over **Public
URL** in **Settings → System → Network**. If there is no override, check
**Public URL** and the [trusted proxy setup](REVERSE_PROXY.md#before-configuring-the-proxy).
Adding `X-Forwarded-Proto: https` alone does not make Pulse trust the proxy.
Do not expose the backend or disable TLS or origin checks. After correcting
the address, register the exact callback shown on the card before trying a
fresh sign-in in a separate browser session.

If the addresses already match, use the existing
[sign-in error to find the failing step](#safe-diagnosis). A redirect loop
alone does not tell you which setting is wrong.
