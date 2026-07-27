# 🔐 OIDC Single Sign-On

Enable Single Sign-On (SSO) with providers like Authentik, Keycloak, Okta, and Azure AD.

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

### Group-to-Role Mapping (Pro and Above)

Automatically assign Pulse roles based on OIDC group membership. When a user logs in, Pulse checks their groups claim and assigns the corresponding roles.

**Configuration:**
Group-role mappings are configured per SSO provider through the UI (or the
SSO provider API for automated setup) — there is no environment-variable
override. Go to **Settings → Security → Single Sign-On**, edit the provider,
and populate **Group Role Mappings** with entries like:
- `oidc-admins` → `admin`
- `oidc-operators` → `operator`
- `oidc-viewers` → `viewer`

The mappings persist on the provider record as a `groupRoleMappings` JSON
field. Provider-level config (including this field) can be PUT through the
SSO provider API for automated setup.

**How it works:**
- On each login, Pulse reads the user's groups from the configured groups claim.
- For each group that matches a mapping, the corresponding role is assigned.
- Multiple groups can map to multiple roles (user gets all matching roles).
- Role assignments are updated on every login to reflect current group membership.
- Role changes are logged to the audit log for compliance tracking.

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

### Microsoft Entra ID (Azure AD)

#### 1. Entra ID Configuration

1. **Create App Registration:**
   * In Microsoft Entra Admin Center, navigate to **Identity > Applications > App registrations**.
   * Click **New registration**.
   * Enter a name (e.g. `Pulse SSO`) and select **Accounts in this organizational directory only** (Single tenant).
   * Click **Register**.

2. **Configure Authentication:**
   * Go to **Authentication** > **Add a platform** > **Web**.
   * Set **Redirect URI** to: `https://pulse.yourdomain.com/api/oidc/callback`
   * Click **Configure**.

3. **Configure Group Claims:**
   * Go to **Token configuration** > **Add groups claim**.
   * Select **Security groups** or **Groups assigned to the application**.
   * Under **ID**, select **Group ID**.
   * Click **Add**.

4. **Restrict Access & Assign Groups (Enterprise Application):**
   * Go to **Identity > Applications > Enterprise applications** and select your Pulse application.
   * Under **Properties**, set **Assignment required?** to **Yes** *(restricts SSO login strictly to assigned users/groups)*.
   * Under **Users and groups**, click **Add user/group** and assign your designated admin group (e.g., `Pulse-Admins`).
   * Copy the group's **Object ID** (GUID, e.g. `a1b2c3d4-e5f6-7890-abcd-123456789abc`).

#### 2. Pulse Environment Variables / Configuration

Configure Pulse with the following settings:

| Parameter | Value | Description / Note |
| :--- | :--- | :--- |
| **Issuer URL** | `https://login.microsoftonline.com/<TENANT_ID>/v2.0` | Replace `<TENANT_ID>` with your Entra ID Directory (tenant) ID. |
| **Client ID** | `<CLIENT_ID>` | Application (client) ID from the App Registration overview. |
| **Client Secret** | `<CLIENT_SECRET>` | Generated under **Certificates & secrets**. |
| **Redirect URI** | `https://pulse.yourdomain.com/api/oidc/callback` | Must match the Redirect URI configured in Entra ID. |
| **Scopes** | `openid profile email` | **Do NOT include `groups`**. Entra ID handles group claims via token configuration, not OIDC scopes. Requesting `groups` will cause an `AADSTS650053` error. |
| **Groups Claim** | `groups` | JSON key in the ID Token where Entra ID returns group Object IDs. |
| **Allowed Groups** | `<GROUP_OBJECT_ID>` | Entra Group Object ID (e.g. `a1b2c3d4-e5f6-7890-abcd-123456789abc`). |
| **Group Role Mappings** | `<GROUP_OBJECT_ID>=admin` | Maps the group Object ID to the `admin` role in Pulse. |

> **Note on Best Practices:**
> * **Overage Prevention:** Selecting *Groups assigned to the application* in Entra ID ensures only relevant group Object IDs are included in the ID Token, preventing JWT bloat.
> * **Group Renames:** Role mappings use the persistent **Group Object ID (GUID)**, making authorization resilient against group display name changes.

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
