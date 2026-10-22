# 🛡️ Proxy Authentication

Authenticate users via your existing reverse proxy (Authentik, Authelia, Cloudflare Zero Trust, etc.).

## 🚀 Quick Start

Proxy authentication delegates identity to your **existing authenticating proxy**;
a shared header is not an identity-provider login. Complete the
[header trust boundary](#-header-trust-boundary) before enabling it. Keep the
existing administrator recovery path until the new login and access checks pass.

1. **Decide who is an administrator.** Leaving `PROXY_AUTH_ROLE_HEADER` unset
   makes **every proxy-authenticated user an administrator**. For mixed-access
   users, configure a role header and the exact intended IdP admin group before
   enabling proxy login. Do not remove role gating to resolve a denial.
2. **Prepare private configuration.** Generate a strong unique shared secret
   using your trusted secret-management tool. Store it in Pulse and the proxy's
   private deployment configuration, not command arguments, shared Compose files
   or a repository. For example, these are mappings, not working defaults:

   ```dotenv
   PROXY_AUTH_SECRET=<unique-private-shared-secret>
   PROXY_AUTH_USER_HEADER=X-Authentik-Username
   PROXY_AUTH_ROLE_HEADER=X-Authentik-Groups
   PROXY_AUTH_ROLE_SEPARATOR=|
   PROXY_AUTH_ADMIN_ROLE=<exact-idp-admin-group>
   ```

   Match the header names and separator to what your existing authenticator
   actually emits. The `|` example is not correct for an IdP that emits
   comma-separated groups. Never use the placeholder strings as credentials
   or invent an admin group to make a user pass.
3. **Complete the proxy boundary.** Require successful IdP authentication before
   forwarding to Pulse. Replace client-supplied identity, role and secret headers
   with trusted values; deny access if the authenticator is unavailable. A
   headers-only middleware does not authenticate anyone. Restrict the backend
   to the existing private proxy path, including any alternate published port.
4. **Verify through the normal browser path.** Check an intended administrator,
   an authenticated non-admin and a signed-out session separately. Signing in
   does not prove the intended privilege; a non-admin must not gain settings
   write access. A signed-out session must be challenged or refused by the
   authenticating proxy, not admitted as a Pulse user. Use existing test accounts
   and read-only observations, not settings changes or a new public backend.

Apply deployment changes during a suitable maintenance window, preserving the
current image, data, credentials and unrelated settings. A Docker environment
change needs recreation/redeployment; a restart alone does not apply it. Follow
[environment precedence](CONFIGURATION.md#common-overrides-environment-variables).

## ⚙️ Configuration

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PROXY_AUTH_SECRET` | **Required**. Shared secret to verify requests. | - |
| `PROXY_AUTH_USER_HEADER` | **Required**. Header containing the username. | - |
| `PROXY_AUTH_ROLE_HEADER` | Header containing user groups/roles. | - |
| `PROXY_AUTH_ROLE_SEPARATOR` | Separator for multiple roles in the header. | `\|` |
| `PROXY_AUTH_ADMIN_ROLE` | Role name that grants admin access. | `admin` |
| `PROXY_AUTH_LOGOUT_URL` | Browser destination for **Logout**; not an access-revocation operation. See [sign-out](#sign-out-and-access-removal). | - |

Setting `PROXY_AUTH_ROLE_HEADER` turns on role gating. From then on admin access fails closed unless the role header is present and contains the admin role — `PROXY_AUTH_ADMIN_ROLE` if you set it, otherwise the `admin` default. A missing or blank role header still authenticates the user, but the user is treated as non-admin.

If you intentionally want every proxy-authenticated user to be an admin, leave `PROXY_AUTH_ROLE_HEADER` unset and protect Pulse entirely at the proxy/IdP layer.

Running Pulse 5.x? Role gating behaves differently there and needs two extra steps — see [Pulse 5.x (end-of-life)](#pulse-5x-end-of-life).

## ⚠ Header Trust Boundary

Pulse trusts these headers completely — they *are* the identity and the privilege decision. Two deployment requirements make that safe, and both are yours to enforce:

1. **Your proxy must _replace_ these headers, never append to them.** On every request the proxy has to discard any client-supplied copy of `X-Proxy-Secret`, your user header, and your role header, then set its own. Pulse reads the **first** value of a repeated header, so a client-supplied `X-Proxy-Roles: admin` that arrives ahead of your proxy's value wins — and because the proxy supplies the shared secret itself, the client never needs to know it. Replacing only the secret is insufficient: the username and configured role header must also come from the successful authenticator, never from the client. Validate this at the proxy; a direct Pulse probe cannot prove it.
2. **Pulse must not be reachable except through the proxy.** Anyone who can connect directly and knows `PROXY_AUTH_SECRET` can assert any username and any role. Bind Pulse to the proxy's network or to localhost.

Neither of these can be enforced from inside Pulse: a forged header and a genuine one are indistinguishable once they arrive.

## 📦 Examples

These are integration requirements for an already configured authenticator,
not complete provider deployment recipes. Follow your installed provider's
version-specific documentation for its authentication middleware and trusted
identity output. Keep its existing access policy and backend isolation; adding
`X-Proxy-Secret` alone is not a secure setup.

### Authentik (with Traefik)

The Pulse router must use the existing Authentik authentication middleware before
trusted identity reaches Pulse. Map the authenticated username and groups to the
same headers configured in Pulse, with the actual group separator. Strip inbound
copies before authentication and replace them with the trusted result. A Traefik
`headers.customRequestHeaders` entry that sets only `X-Proxy-Secret` does not
configure Authentik, protect the route or sanitise the user/group headers. Do not
add a static username or admin group as a shortcut.

### Authelia (Nginx)

Use the existing Authelia authorisation subrequest and explicitly capture its
successful identity and group response for the upstream request. Merely adding
`auth_request` and referencing `$upstream_http_remote_user` is not a complete
identity mapping: the authenticated subrequest's output must be assigned with
`auth_request_set` and used for the headers sent to Pulse. Replace all configured
identity, role and secret headers; a failed or unavailable subrequest must not
forward an authenticated request. Match Pulse's role separator to the actual
Authelia group output, not the example separator above.

### Cloudflare Tunnel

A tunnel is a transport, not an Access policy. Require the existing Cloudflare
Access application and its authenticated identity on every route to Pulse;
protect alternate hostnames and keep direct backend access private. Pulse's proxy
authentication does not validate a Cloudflare Access JWT: it checks the shared
secret and configured identity/role headers. The trusted Access/connector path
must establish identity and replace those headers before forwarding. Do not
assume a client-supplied `Cf-Access-Authenticated-User-Email` or a static secret
header proves a login. If no trusted role mapping is available, do not silently
fall back to all-users-admin access.

## 🔧 Troubleshooting

| Issue | Check |
| :--- | :--- |
| **401 Unauthorized** | Verify `X-Proxy-Secret` matches `PROXY_AUTH_SECRET`. Check if headers are being stripped by intermediate proxies. |
| **Not Admin** | Check the role header actually reaches Pulse and contains the admin role **exactly** — matching is case-sensitive and compares whole roles, so `Admins` and `authentik Admins` do not match the `admin` default. Set `PROXY_AUTH_ADMIN_ROLE` to your IdP's admin group name. Pulse logs a warning at startup when it falls back to the default. |
| **Logout returns straight to Pulse** | Check the authenticator's actual sign-out behaviour and [sign-out boundary](#sign-out-and-access-removal); setting a redirect URL alone does not revoke access. |

### Sign-out and access removal

**A logout redirect is not proof that access ended.** Pulse's **Logout** button
has two different paths:

- With `PROXY_AUTH_LOGOUT_URL` set, the browser navigates to that URL **instead
  of calling Pulse's `/api/logout`**. Pulse does not first clear its own session
  through that endpoint, or perform a server-side IdP logout. Use the existing
  authenticator's supported logout destination, not a Pulse home page or an
  arbitrary redirect. That destination's behaviour depends on your provider
  and its installed configuration.
- Without that URL, Pulse requests its normal logout and clears local UI auth
  state. This does **not** sign out of the authenticating proxy or IdP. If the
  proxy still supplies valid identity headers, it can admit the next request
  immediately, without showing Pulse's login form.

Neither path is a universal **sign out all sessions** operation or revocation
of API tokens and agent credentials. Closing a tab, seeing a logged-out page
or reaching the configured destination does not establish that another Pulse
tab, browser session or device lost access.

For a planned sign-out check, keep an independent administrator recovery
session. Use the authenticator's normal sign-out flow, then revisit the
protected Pulse URL in the **same browser session** without signing in again:
it should require authentication or refuse access. A new private window tests
a different session and cannot prove the old one ended. If Pulse immediately
opens again, do not hand that browser or device to another person on the
assumption that it is signed out. Use your established device lock and
authenticator recovery procedure; do not repeatedly log out just to diagnose it.

To remove a person's access, use the existing proxy/IdP assignment and session
revocation controls, preserving unrelated users and administrator recovery.
Verify the affected session through the protected route where safely available;
blocking only future IdP logins does not prove that existing proxy sessions
were revoked. Keep the [backend isolation](#-header-trust-boundary) intact.
Do not disable Pulse authentication, expose its backend, delete configuration
or rotate the shared proxy secret as a substitute for removing one identity.
Keep cookies, shared secrets and complete logout URLs with query strings out
of commands and reports; use only a redacted observation when requesting help.

### Verify Headers
First use a normal signed-in browser through your proxy. Check that the intended
user has the expected access; do not extract its cookies or use **Copy as cURL**.
The shared proxy secret can assert any identity, so a direct test belongs only
on a trusted backend/proxy host or inside its private network. Do not expose the
backend port or change firewall rules to make this test possible.

If an authorised administrator needs to isolate Pulse's role check, prepare a
temporary private header file in an editor. In that editor, enter the following,
substituting your configured user/role header names, real shared secret and a
test user's **non-admin** role (not the admin role). Do not add an API token or
session cookie: those would test a different credential path.

```text
X-Proxy-Secret: <your-real-shared-secret>
X-Authentik-Username: testuser
X-Proxy-Roles: none
```

Run this read-only probe (curl 7.76 or later). It sends no settings changes and
prints only the HTTP status, not settings, cookies or credential-bearing logs:

```bash
(
  set -eu
  umask 077
  probe_dir=$(mktemp -d)
  header_file="$probe_dir/proxy-headers"
  trap 'test ! -e "$header_file" || rm -- "$header_file"; rmdir -- "$probe_dir"' EXIT
  touch "$header_file"
  vi "$header_file"
  if status=$(curl --disable --silent --show-error --output /dev/null \
    --proto '=http' --noproxy 127.0.0.1 \
    --connect-timeout 5 --max-time 10 \
    --write-out '%{http_code}' --header "@$header_file" \
    http://127.0.0.1:7655/api/system/settings); then
    printf 'HTTP %s\n' "$status"
  else
    curl_exit=$?
    printf 'Proxy probe unavailable (curl exit %s); no role result. Stop here.\n' "$curl_exit" >&2
    exit "$curl_exit"
  fi
)
```

The probe permits five seconds to connect and ten seconds in total, with no
retry. A failed or timed-out transport prints no HTTP role result; retain the
error and stop, rather than removing the deadline or repeating the request.
The temporary header file is removed on exit, including an editor or curl
failure. Cleanup does not revoke the shared secret.

The loopback address applies only on the Pulse host with a loopback listener.
`--noproxy 127.0.0.1` explicitly bypasses proxy environment settings for this
direct request. `--disable` ignores curl's configuration file, **not** those
environment settings: without the bypass, a proxy could receive the shared
secret and its response could be mistaken for Pulse's role result.

Otherwise use the existing private backend address over HTTPS with certificate
verification enabled. Replace `--proto '=http'` with `--proto '=https'` and the
`--noproxy` value with that exact backend hostname or IP; do not send the shared
secret through an environment proxy. Keep backend isolation intact. Do not send
forged test identities through a public proxy that should replace them. Keep
`--disable` first to ignore local curl defaults; do not enable verbose/trace
output, `--insecure` or redirect following.

With role gating configured on Pulse 6.2.2 or later, expect **403** for a valid
non-admin identity. Repeat the same read-only probe with the role line omitted,
then blank: both should still be **403**. **200** means admin access is allowed;
check whether role gating is configured. **401** means authentication failed,
not that the non-admin role was checked. A redirect, connection/TLS error or
`HTTP 000` is not a successful denial test. Curl deliberately does not fail on
403 here because it is the expected result; assess the printed status separately
from transport success. Stop on an unexpected result rather than granting more
access. This tests Pulse's direct header handling, **not** your proxy's header
replacement or end-to-end access boundary. Share only the status and redacted
configuration names, never the header file or secret.

## Pulse 5.x (end-of-life)

The 5.x line is end-of-life and does not receive fixes. **Upgrade to 6.2.2 or later**, which is where proxy-auth role gating behaves as documented above.

If you cannot upgrade immediately, 5.x needs two configuration changes before role gating actually restricts anyone. **Both are required** — either one alone leaves administrator access open:

1. **Set `PROXY_AUTH_ADMIN_ROLE` explicitly.** 5.x never applies the documented `admin` default. While it is empty, 5.x skips role checking altogether and treats every proxy-authenticated user as an administrator, whatever their role header says.
2. **Send a non-empty role header on every authenticated request.** 5.x only evaluates roles when that header carries a value; an absent or empty header leaves the user an administrator. A user with no groups in your identity provider commonly produces exactly that, so have the proxy send a placeholder such as `none` rather than nothing. Any non-empty value that does not contain your admin role correctly resolves to non-admin.

Use the private-file probe above with a non-empty non-admin role: it should
return **403**. A direct 5.x request with the role header omitted can still
return **200** despite the explicit admin role; that is the old behaviour, not
proof that the workaround failed. The mitigation depends on the real proxy
always supplying a non-empty role, including for users with no groups. Verify
that end-to-end with such a user; do not expose or bypass the proxy to rely on
this workaround. A successful direct test alone does not establish that every
proxied request carries the role. Upgrade rather than treating the workaround
as equivalent to the current fail-closed behaviour.

6.2.2 and later fix both behaviors: role gating activates on the presence of the role header alone, and an absent or blank header resolves to non-admin.
