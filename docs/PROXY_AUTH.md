# Proxy Authentication

Pulse supports proxy-based authentication for integration with SSO providers like Authentik, Authelia, Caddy, and others. This allows you to authenticate users via your existing reverse proxy authentication system while maintaining security.

> **When to use this**: If you already have an authentication proxy (Authentik, Authelia, etc.) protecting your services and want Pulse to trust that authentication instead of requiring its own login.

## Quick Start

1. Set `PROXY_AUTH_SECRET` to a random secret string
2. Configure your proxy to send this secret in the `X-Proxy-Secret` header
3. Set `PROXY_AUTH_USER_HEADER` to match your proxy's username header
4. (Optional) Configure role-based access control with `PROXY_AUTH_ROLE_HEADER`

## Configuration

Set the following environment variables to enable proxy authentication:

### Required Settings

```bash
# Shared secret between proxy and Pulse (required)
PROXY_AUTH_SECRET=your-secure-secret-here

# Header containing the authenticated username (optional but recommended)
PROXY_AUTH_USER_HEADER=X-Authentik-Username
```

### Optional Settings

```bash
# Header containing user roles/groups
PROXY_AUTH_ROLE_HEADER=X-Authentik-Groups

# Separator for multiple roles (default: |)
PROXY_AUTH_ROLE_SEPARATOR=|

# Role name that grants admin access (default: admin)
PROXY_AUTH_ADMIN_ROLE=admin

# URL to redirect users to for logout
PROXY_AUTH_LOGOUT_URL=/outpost.goauthentik.io/sign_out
```

## How It Works

1. **User visits Pulse** → Your proxy intercepts the request
2. **Proxy authenticates user** → Via its own login page/SSO
3. **Proxy adds headers** to the request:
   - `X-Proxy-Secret`: Shared secret (prevents spoofing)
   - Username header (e.g., `X-Authentik-Username`)
   - Roles header (e.g., `X-Authentik-Groups`)
4. **Pulse validates** the secret and trusts the user identity
5. **No Pulse login required** → User sees the dashboard immediately

## Example Configurations

### Authentik with Traefik

```yaml
# docker-compose.yml environment variables
environment:
  - PROXY_AUTH_SECRET=your-secure-secret-here
  - PROXY_AUTH_USER_HEADER=X-Authentik-Username
  - PROXY_AUTH_ROLE_HEADER=X-Authentik-Groups
  - PROXY_AUTH_ROLE_SEPARATOR=|
  - PROXY_AUTH_ADMIN_ROLE=admin
  - PROXY_AUTH_LOGOUT_URL=/outpost.goauthentik.io/sign_out
```

Traefik middleware configuration:

```yaml
http:
  middlewares:
    proxy-header-secret:
      headers:
        customRequestHeaders:
          X-Proxy-Secret: "your-secure-secret-here"
    
    authentik-auth:
      forwardAuth:
        address: http://authentik:9000/outpost.goauthentik.io/auth/traefik
        trustForwardHeader: true
        authResponseHeaders:
          - X-Authentik-Username
          - X-Authentik-Groups
          - X-Authentik-Email

  routers:
    pulse:
      rule: Host(`pulse.example.com`)
      entryPoints:
        - websecure
      middlewares:
        - authentik-auth
        - proxy-header-secret
      service: pulse-service
    
    pulse-auth:
      rule: Host(`pulse.example.com`) && PathPrefix(`/outpost.goauthentik.io/`)
      entryPoints:
        - websecure
      service: authentik-outpost
```

### Authelia Example

```yaml
# docker-compose.yml environment variables
environment:
  - PROXY_AUTH_SECRET=your-secure-secret-here
  - PROXY_AUTH_USER_HEADER=Remote-User
  - PROXY_AUTH_ROLE_HEADER=Remote-Groups
  - PROXY_AUTH_ROLE_SEPARATOR=,
  - PROXY_AUTH_ADMIN_ROLE=admins
  - PROXY_AUTH_LOGOUT_URL=/logout
```

Nginx configuration:
```nginx
location / {
    # Authelia authorization
    auth_request /authelia;
    auth_request_set $user $upstream_http_remote_user;
    auth_request_set $groups $upstream_http_remote_groups;
    
    # Pass headers to Pulse
    proxy_set_header X-Proxy-Secret "your-secure-secret-here";
    proxy_set_header Remote-User $user;
    proxy_set_header Remote-Groups $groups;
    proxy_pass_header X-RateLimit-Limit;
    proxy_pass_header X-RateLimit-Remaining;
    proxy_pass_header X-RateLimit-Reset;
    proxy_pass_header Retry-After;

    proxy_pass http://pulse:7655;
}
```

### Caddy with Forward Auth

```caddyfile
pulse.example.com {
    forward_auth authelia:9091 {
        uri /api/verify?rd=https://auth.example.com
        copy_headers Remote-User Remote-Groups
    }
    
    header_downstream X-Proxy-Secret "your-secure-secret-here"
    
    reverse_proxy pulse:7655
}
```

### Nginx Proxy Manager

In NPM's Advanced tab for your Pulse proxy host:

```nginx
# Custom Nginx Configuration
proxy_set_header X-Proxy-Secret "your-secure-secret-here";
proxy_set_header X-Authentik-Username $http_x_authentik_username;
proxy_set_header X-Authentik-Groups $http_x_authentik_groups;
```

## Security Considerations

1. **Use a strong secret**: Generate a secure random string for `PROXY_AUTH_SECRET`
2. **HTTPS only**: Always use HTTPS between the proxy and Pulse in production
3. **Network isolation**: Ensure Pulse is not directly accessible, only through the proxy
4. **Header validation**: Pulse validates all headers and the proxy secret on every request
5. **Preserve rate-limit headers**: Do not strip `X-RateLimit-*` or `Retry-After`. Clients rely on them when Pulse throttles requests.

## Combining with Other Auth Methods

Proxy authentication can work alongside other authentication methods:

- If `PROXY_AUTH_SECRET` is set, proxy auth takes precedence
- API tokens (`API_TOKENS` or legacy `API_TOKEN`) still work for programmatic access
- Basic auth (`PULSE_AUTH_USER`/`PULSE_AUTH_PASS`) can be used as fallback

## Troubleshooting

### Users can't access Pulse (401 Unauthorized)

1. **Check the secret header**:
   ```bash
   # Test with curl
   curl -H "X-Proxy-Secret: your-secret" \
        -H "X-Authentik-Username: testuser" \
        http://pulse:7655/api/state
   ```

2. **Verify headers are being sent**:
   - Temporarily raise logging to debug via **Settings → System → Logging** (or set `LOG_LEVEL=debug` and restart). Remember to return to `info` when finished.
   - Check Pulse logs: `docker logs pulse` or `journalctl -u pulse`
   - Look for "Invalid proxy secret" or "Proxy auth user header not found"

3. **Common issues**:
   - Typo in `PROXY_AUTH_SECRET` 
   - Header names are case-sensitive in configuration
   - Proxy not forwarding headers correctly

### Admin features not available

Check if user is recognized as admin:
```bash
curl -H "X-Proxy-Secret: your-secret" \
     -H "X-Authentik-Username: admin" \
     -H "X-Authentik-Groups: users|admin" \
     http://pulse:7655/api/security/status | jq '.proxyAuthIsAdmin'
```

- Ensure the roles header contains the admin role
- Verify `PROXY_AUTH_ADMIN_ROLE` matches your configuration  
- Check the role separator matches your proxy's format (default: `|`)

### Logout doesn't work

- Verify `PROXY_AUTH_LOGOUT_URL` points to your proxy's logout endpoint
- Ensure the logout URL is accessible from the user's browser
- For Authentik: `/outpost.goauthentik.io/sign_out`
- For Authelia: `/logout` or custom path

### Testing your configuration

Test proxy auth without a reverse proxy:
```bash
# Should return 401
curl http://localhost:7655/api/state

# Should return 200 with state data
curl -H "X-Proxy-Secret: your-secret-here" \
     -H "X-Your-User-Header: testuser" \
     http://localhost:7655/api/state
```

## FAQ

**Q: Do I still need to set up Pulse authentication?**  
A: No, when proxy auth is configured, Pulse trusts your proxy's authentication. Users won't see Pulse's login page.

**Q: Can I use this with Cloudflare Access or Tailscale?**  
A: Yes, any service that can add custom headers after authentication will work.

**Q: What happens if someone bypasses my proxy?**  
A: They can't authenticate. Without the correct `X-Proxy-Secret` header, all requests are rejected with 401.

**Q: Can I have some users with read-only access?**  
A: Currently, Pulse has admin and non-admin roles. Non-admin users have read-only access to monitoring data.

**Q: Is the username displayed in Pulse?**  
A: Yes, the authenticated username appears in the top-right corner of the UI.

**Q: Can I use both proxy auth and API tokens?**  
A: Yes! API tokens still work for automation/scripts. Proxy auth is for human users via the web UI.
