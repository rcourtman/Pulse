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
| **Not Admin** | Verify `PROXY_AUTH_ROLE_HEADER` is set and contains `PROXY_AUTH_ADMIN_ROLE`. |
| **Logout Fails** | Ensure `PROXY_AUTH_LOGOUT_URL` is set to your IdP's logout endpoint. |

### Verify Headers
Use `curl` to simulate a proxy request:
```bash
curl -H "X-Proxy-Secret: your-secret" \
     -H "X-Authentik-Username: admin" \
     http://localhost:7655/api/state
```
