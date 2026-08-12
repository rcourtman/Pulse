# 🛡️ Proxy Authentication

Authenticate users via your existing reverse proxy (Authentik, Authelia, Cloudflare Zero Trust, etc.).

## 🚀 Quick Start

1.  **Generate Secret**: Create a strong random string.
2.  **Configure Pulse**:
    ```bash
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
Use `curl` to simulate a proxy request:
```bash
curl -H "X-Proxy-Secret: your-secret" \
     -H "X-Authentik-Username: admin" \
     http://localhost:7655/api/state
```

## Pulse 5.x (end-of-life)

The 5.x line is end-of-life and does not receive fixes. **Upgrade to 6.2.2 or later**, which is where proxy-auth role gating behaves as documented above.

If you cannot upgrade immediately, 5.x needs two configuration changes before role gating actually restricts anyone. **Both are required** — either one alone leaves administrator access open:

1. **Set `PROXY_AUTH_ADMIN_ROLE` explicitly.** 5.x never applies the documented `admin` default. While it is empty, 5.x skips role checking altogether and treats every proxy-authenticated user as an administrator, whatever their role header says.
2. **Send a non-empty role header on every authenticated request.** 5.x only evaluates roles when that header carries a value; an absent or empty header leaves the user an administrator. A user with no groups in your identity provider commonly produces exactly that, so have the proxy send a placeholder such as `none` rather than nothing. Any non-empty value that does not contain your admin role correctly resolves to non-admin.

Verify both with a request carrying a non-admin role — it should be refused:

```bash
curl -si -H "X-Proxy-Secret: your-secret" \
     -H "X-Authentik-Username: testuser" \
     -H "X-Proxy-Roles: none" \
     http://localhost:7655/api/system/settings | head -1
```

Expected: `HTTP/1.1 403 Forbidden`. Repeat it with the role header omitted entirely — that must also be refused. If either returns `200`, admin access is still open.

6.2.2 and later fix both behaviors: role gating activates on the presence of the role header alone, and an absent or blank header resolves to non-admin.
