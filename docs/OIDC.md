# 🔐 OIDC Single Sign-On

Enable Single Sign-On (SSO) with providers like Authentik, Keycloak, Okta, and Azure AD.

## 🚀 Quick Start

1.  **Configure Provider**: Create an OIDC application in your IdP.
    - **Redirect URI**: `https://<your-pulse-domain>/api/oidc/callback`
    - **Scopes**: `openid`, `profile`, `email`
2.  **Enable in Pulse**: Go to **Settings → Security → Single Sign-On**.
3.  **Enter Details**:
    - **Issuer URL**: The base URL of your IdP (e.g., `https://auth.example.com/application/o/pulse/`).
    - **Client ID & Secret**: From your IdP.
4.  **Save**: The login page will now show a "Continue with Single Sign-On" button.

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

### Group-to-Role Mapping (Pro)

Automatically assign Pulse roles based on OIDC group membership. When a user logs in, Pulse checks their groups claim and assigns the corresponding roles.

**Configuration via UI:**
Go to **Settings → Security → Single Sign-On → Group Role Mappings** and add mappings like:
- `oidc-admins` → `admin`
- `oidc-operators` → `operator`
- `oidc-viewers` → `viewer`

**Configuration via Environment:**
```bash
# Format: group1=role1,group2=role2
OIDC_GROUP_ROLE_MAPPINGS="oidc-admins=admin,oidc-operators=operator"
```

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
- **Redirect URI**: `https://pulse.example.com/api/oidc/callback`
- **Signing Key**: Must use **RS256** (create a certificate/key pair if needed).
- **Issuer URL**: `https://auth.example.com/application/o/pulse/`

### Keycloak
- **Client ID**: `pulse`
- **Access Type**: Confidential
- **Valid Redirect URIs**: `https://pulse.example.com/api/oidc/callback`
- **Issuer URL**: `https://keycloak.example.com/realms/myrealm`

### Azure AD
- **Redirect URI**: `https://pulse.example.com/api/oidc/callback` (Web)
- **Issuer URL**: `https://login.microsoftonline.com/<tenant-id>/v2.0`
- **Note**: Enable "ID tokens" in Authentication settings.

## 🔧 Troubleshooting

| Issue | Solution |
| :--- | :--- |
| **`invalid_id_token`** | Issuer URL mismatch. Check logs (`LOG_LEVEL=debug`) to see the expected vs. received issuer. |
| **`unexpected signature algorithm "HS256"`** | Your IdP is signing with HS256. Configure it to use **RS256**. |
| **Redirect Loop** | Check `X-Forwarded-Proto` header (must be `https`) and cookie settings. |
| **Self-Signed Certs** | Mount your CA bundle to `/etc/ssl/certs/oidc-ca.pem` and set `OIDC_CA_BUNDLE`. |

### Debugging
Enable debug logs to trace the OIDC flow:
```bash
export LOG_LEVEL=debug
# Restart Pulse
```
Logs will show discovery, token exchange, and claim parsing details.
