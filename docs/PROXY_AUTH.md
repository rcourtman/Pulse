# 🛡️ Proxy Authentication

Authenticate users via your existing reverse proxy (Authentik, Authelia, Cloudflare Zero Trust, etc.).

## 🚀 Quick Start

1.  **Generate Secret**: Create a strong random string.
2.  **Configure Pulse**: Set these in your deployment's private configuration,
    not a shell command, shared Compose file or repository:
    ```dotenv
    PROXY_AUTH_SECRET=your-random-secret
    PROXY_AUTH_USER_HEADER=X-Authentik-Username
    ```
3.  **Configure Proxy**: Set the proxy to send `X-Proxy-Secret` and the user header.

## ⚙️ Configuration

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PROXY_AUTH_SECRET` | **Required**. Shared secret to verify requests. | - |
| `PROXY_AUTH_USER_HEADER` | **Required**. Header containing the username. | - |
| `PROXY_AUTH_ROLE_HEADER` | Header containing user groups/roles. | - |
| `PROXY_AUTH_ROLE_SEPARATOR` | Separator for multiple roles in the header. | `\|` |
| `PROXY_AUTH_ADMIN_ROLE` | Role name that grants admin access. | `admin` |
| `PROXY_AUTH_LOGOUT_URL` | URL to redirect to after logout. | - |

Setting `PROXY_AUTH_ROLE_HEADER` turns on role gating. From then on admin access fails closed unless the role header is present and contains the admin role — `PROXY_AUTH_ADMIN_ROLE` if you set it, otherwise the `admin` default. A missing or blank role header still authenticates the user, but the user is treated as non-admin.

If you intentionally want every proxy-authenticated user to be an admin, leave `PROXY_AUTH_ROLE_HEADER` unset and protect Pulse entirely at the proxy/IdP layer.

Running Pulse 5.x? Role gating behaves differently there and needs two extra steps — see [Pulse 5.x (end-of-life)](#pulse-5x-end-of-life).

## ⚠️ Header Trust Boundary

Pulse trusts these headers completely — they *are* the identity and the privilege decision. Two deployment requirements make that safe, and both are yours to enforce:

1. **Your proxy must _replace_ these headers, never append to them.** On every request the proxy has to discard any client-supplied copy of `X-Proxy-Secret`, your user header, and your role header, then set its own. Pulse reads the **first** value of a repeated header, so a client-supplied `X-Proxy-Roles: admin` that arrives ahead of your proxy's value wins — and because the proxy supplies the shared secret itself, the client never needs to know it. The examples below use nginx `proxy_set_header` and Traefik `customRequestHeaders`, which replace; be careful with anything that adds a header rather than setting it.
2. **Pulse must not be reachable except through the proxy.** Anyone who can connect directly and knows `PROXY_AUTH_SECRET` can assert any username and any role. Bind Pulse to the proxy's network or to localhost.

Neither of these can be enforced from inside Pulse: a forged header and a genuine one are indistinguishable once they arrive.

## 📦 Examples

The secret values below are placeholders, not defaults. Replace them with the
same strong unique secret in Pulse and the proxy, using private configuration
files or your deployment's secret mechanism. Keep the rendered configuration
and container environment private too; do not share `docker inspect` or
resolved Compose output.

### Authentik (with Traefik)
**docker-compose.yml**:
```yaml
environment:
  - PROXY_AUTH_SECRET=secure-secret
  - PROXY_AUTH_USER_HEADER=X-Authentik-Username
```

**Traefik Middleware**:
```yaml
headers:
  customRequestHeaders:
    X-Proxy-Secret: "secure-secret"
```

### Authelia (Nginx)
```nginx
location / {
    auth_request /authelia;
    proxy_set_header X-Proxy-Secret "secure-secret";
    proxy_set_header Remote-User $upstream_http_remote_user;
    proxy_pass http://pulse:7655;
}
```

### Cloudflare Tunnel
1.  **Zero Trust Dashboard**: Applications → Add Application.
2.  **Settings**: HTTP Settings → HTTP Headers.
3.  **Add Header**: `X-Proxy-Secret` = `your-secret`.
4.  **Pulse Config**: `PROXY_AUTH_USER_HEADER=Cf-Access-Authenticated-User-Email`.

## 🔧 Troubleshooting

| Issue | Check |
| :--- | :--- |
| **401 Unauthorized** | Verify `X-Proxy-Secret` matches `PROXY_AUTH_SECRET`. Check if headers are being stripped by intermediate proxies. |
| **Not Admin** | Check the role header actually reaches Pulse and contains the admin role **exactly** — matching is case-sensitive and compares whole roles, so `Admins` and `authentik Admins` do not match the `admin` default. Set `PROXY_AUTH_ADMIN_ROLE` to your IdP's admin group name. Pulse logs a warning at startup when it falls back to the default. |
| **Logout Fails** | Ensure `PROXY_AUTH_LOGOUT_URL` is set to your IdP's logout endpoint. |

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
  curl --disable --silent --show-error --output /dev/null \
    --write-out 'HTTP %{http_code}\n' --header "@$header_file" \
    http://127.0.0.1:7655/api/system/settings
)
```

The loopback address applies only on the Pulse host with a loopback listener.
Otherwise use its existing private backend address over HTTPS with certificate
verification enabled. Do not send forged test identities through a public proxy
that should replace them. Keep `--disable` first to ignore local curl defaults;
do not enable verbose/trace output, `--insecure` or redirect following.

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
