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

### Long-Lived Sessions with `offline_access`
For persistent sessions that don't require frequent re-authentication:

1. **Add `offline_access` scope**: Include `offline_access` in your OIDC scopes (e.g., `openid profile email offline_access`).
2. **Configure your IdP**: Ensure your identity provider issues refresh tokens when `offline_access` is requested.

**How it works:**
- When you login with `offline_access`, Pulse stores the refresh token alongside your session.
- When your access token expires, Pulse automatically refreshes it using the stored refresh token.
- Your session remains valid as long as the refresh token is valid (typically 30-90 days depending on your IdP).
- If the IdP revokes access (user disabled, token revoked), Pulse detects this on the next refresh attempt and logs you out.

**Security considerations:**
- Refresh tokens are stored encrypted at rest.
- If the IdP configuration changes, existing sessions with mismatched issuers are automatically invalidated.
- Failed refresh attempts immediately invalidate the session.

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
| **`invalid_id_token`** | Issuer URL mismatch. Check logs (`LOG_LEVEL=debug`) to see the expected vs. received issuer. |
| **`unexpected signature algorithm "HS256"`** | Your IdP is signing with HS256. Configure it to use **RS256**. |
| **Redirect Loop** | Check `X-Forwarded-Proto` header (must be `https`) and cookie settings. |
| **Self-Signed Certs** | Set the **CA Bundle** field on the SSO provider to a host path readable by Pulse (e.g. `/etc/ssl/certs/oidc-ca.pem` mounted into the container). The field is stored on the provider record as `oidc.caBundle`; there is no `OIDC_CA_BUNDLE` env var. |

### Debugging
Enable debug logs to trace the OIDC flow:
```bash
export LOG_LEVEL=debug
# Restart Pulse
```
Logs will show discovery, token exchange, and claim parsing details.
