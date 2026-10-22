# 🔄 Reverse Proxy Setup

Pulse uses WebSockets for real-time updates. Your proxy **MUST** support WebSockets.

## Before configuring the proxy

Use HTTPS at the public proxy and keep Pulse's backend private. The examples
below are configuration fragments, not complete TLS or authentication setups.
Keep Pulse authentication enabled; TLS termination does not authenticate users.
For identity supplied by an IdP/proxy, follow the separate
[proxy authentication guide](PROXY_AUTH.md), including its Header Trust Boundary.

A same-origin reverse proxy needs no CORS exception. Keep the UI and API on the
same public origin rather than adding `*` to repair a login or routing error.
For a separate trusted browser app, follow the
[exact-origin CORS checks](TROUBLESHOOTING.md#cors-errors).

**Forwarded HTTPS headers are trusted only from a configured immediate peer.**
Set `PULSE_TRUSTED_PROXY_CIDRS` in Pulse's deployment configuration to the proxy
address **as seen by Pulse**, not the browser's address or the public hostname.
For a proxy on the same host connecting to `127.0.0.1:7655`, use:

```dotenv
PULSE_TRUSTED_PROXY_CIDRS=127.0.0.1/32
```

Use `::1/128` instead if that connection actually uses IPv6 loopback. A Docker
proxy normally reaches Pulse from its private container/network address, not
loopback; trust only that proxy's fixed address or a narrowly isolated proxy
network. Do not trust a whole LAN or all Docker networks to make a warning go
away. Wildcard ranges `0.0.0.0/0` and `::/0` are rejected at startup.

At the public edge, replace client-supplied forwarded scheme, host and client-IP
headers with values established by the proxy; do not append an untrusted value.
With multiple proxy hops, configure the upstream trust boundary on each hop
and trust only the final peer in Pulse. `PULSE_TRUSTED_NETWORKS` is not a
substitute for proxy trust, and proxy trust is not an authentication bypass.
See [configuration overrides](CONFIGURATION.md#common-overrides-environment-variables).

## ⚡ Quick Configs

### Nginx

Inside your HTTPS `server` block, for a single public-edge proxy:

```nginx
location / {
    proxy_pass http://127.0.0.1:7655;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Scheme "";
    proxy_set_header Forwarded "";

    # Critical for WebSockets
    proxy_read_timeout 86400; # 24h
}
```

### Caddy

Caddy handles the WebSocket upgrade and sets forwarded client-IP, host and
scheme headers automatically. The removals below discard other client-supplied
forwarding metadata. Pulse still needs the matching trusted-peer setting above:

```caddy
pulse.example.com {
    reverse_proxy 127.0.0.1:7655 {
        header_up -Forwarded
        header_up -X-Forwarded-Scheme
        header_up -X-Forwarded-Port
    }
}
```

### Traefik (Docker Compose)

These labels assume an existing HTTPS entrypoint named `websecure` and a
configured certificate. Keep Pulse on the private proxy network; do not publish
its backend port publicly. Apply trusted-peer configuration to Pulse separately.
Keep Traefik's `forwardedHeaders.insecure` disabled; upstream forwarded headers
need their own narrow trusted-IP configuration, not blanket client trust.

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.pulse.rule=Host(`pulse.example.com`)"
  - "traefik.http.routers.pulse.entrypoints=websecure"
  - "traefik.http.routers.pulse.tls=true"
  - "traefik.http.routers.pulse.service=pulse"
  - "traefik.http.services.pulse.loadbalancer.server.port=7655"
```

### Apache

Inside a TLS-enabled virtual host, with the required proxy, rewrite and headers
modules enabled:

```apache
ProxyPreserveHost On
ProxyAddHeaders On
RequestHeader unset X-Forwarded-For
RequestHeader unset X-Forwarded-Host
RequestHeader unset X-Forwarded-Port
RequestHeader unset X-Real-IP
RequestHeader set X-Forwarded-Proto "https"
RequestHeader unset X-Forwarded-Scheme
RequestHeader unset Forwarded
RewriteEngine On
RewriteCond %{HTTP:Upgrade} websocket [NC]
RewriteCond %{HTTP:Connection} upgrade [NC]
RewriteRule ^/?(.*) "ws://127.0.0.1:7655/$1" [P,L]

ProxyPass / http://127.0.0.1:7655/
ProxyPassReverse / http://127.0.0.1:7655/
```

---

## ⚠️ Common Issues

### "HTTPS: HTTP only" in Security Posture

If your reverse proxy terminates SSL but Pulse shows "HTTPS: HTTP only" in Settings → Security:

Pulse's secure-cookie and request-origin logic recognises direct TLS, or
`X-Forwarded-Proto: https` from an immediate peer in
`PULSE_TRUSTED_PROXY_CIDRS`; a header from any other peer is ignored for those
decisions, even when the browser used HTTPS. Adding the header alone is not sufficient.
The Security Posture HTTPS
indicator is different: it is true when the request has direct TLS or any
`X-Forwarded-Proto: https` header, whatever the peer. A passing indicator
therefore does not prove that proxy trust is configured correctly, and
"HTTP only" means neither TLS nor that header reached Pulse.

Check the proxy's actual backend connection address and the effective Pulse
setting first, then check that the TLS proxy sends exactly `https`. For Nginx,
keep `proxy_set_header X-Forwarded-Proto $scheme` inside the HTTPS server's
location; do not label a plain-HTTP request as HTTPS. Caddy normally needs no
header override. Any Caddy `header_up` override belongs **inside** its
`reverse_proxy` block, not directly in the site block.

After applying configuration through your normal deployment path, sign in
through the real HTTPS URL and recheck Security Posture and live dashboard
updates. If using SSO, check its redirect/callback on that same URL. A successful
`/api/health` request alone does not verify authentication, WebSockets or proxy
trust. Do not disable authentication, certificate verification or origin checks,
or expose the backend, to test this. Share only redacted configuration and
HTTP/error status, not cookies, proxy secrets or a full network export.

### Dashboard stays on "Connecting to Pulse" or stops updating

A successful login, `/api/health` returning `200`, or Docker reporting `healthy`
does not prove that the dashboard received fresh monitoring state. A WebSocket
`101` upgrade or `welcome` message establishes a connection, not state delivery.

Use observations you already have; there is no need to recreate the outage or
collect a full network export:

- **The page or connection failed before the WebSocket upgrade:** retain the
  existing HTTP status or redacted error. A rejected upgrade may involve the
  proxy, session or origin checks; it is not automatically a proxy-header fault.
  Compare the relevant configuration above without weakening authentication,
  certificate verification or origin restrictions.
- **The upgrade succeeds, but only `welcome` arrives or readings stop changing:**
  retain that distinction and the original time and timezone. A green healthcheck
  does not clear it. More proxy timeout or header changes do not establish a
  repair for a connection that already upgraded.
- **It already happened in several browsers or through an existing private
  direct connection:** include that comparison; do not ask someone to repeat it.
  Do not publish Pulse's backend port or bypass HTTPS to create a comparison.
  If a restart already restored the UI, keep that result and its time separate
  from the failure; recovery after restart does not establish its cause.

For [Getting Help](TROUBLESHOOTING.md#-getting-help), use the affected Pulse
version/image, observed duration, whether login and the upgrade succeeded, and
whether monitoring state arrived, where already known. Unknown is valid. Keep
cookies, tokens, full WebSocket payloads, HAR files, profiles and goroutine dumps
private; use only a locally reviewed, redacted error or summary. A stack showing
one waiting goroutine does not identify the lock holder or prove a deadlock.

Keep any working administrator session. Do not repeatedly reload, sign out,
restart Pulse, enable Debug, reset configuration or run Diagnostics just to
complete a report. If a guest is unresponsive or backup freeze/thaw is uncertain,
follow [Backup safety](VM_DISK_MONITORING.md#backup-safety) first; a healthy
container does not establish guest or filesystem recovery.

### Other Issues

- **"Connection lost" (Workloads) or "Disconnected", "Reconnecting...", "Sync reconnecting" or "Backend healthy" in the header status badge**: the live data stream is not connected, which can happen when the WebSocket upgrade fails. Distinguish a rejected upgrade from [a connection without fresh state](#dashboard-stays-on-connecting-to-pulse-or-stops-updating) before changing proxy headers.
- **502 Bad Gateway**: Check Pulse's actual listener and the proxy's upstream address, port and network reachability; this does not by itself prove Pulse is stopped. Container `localhost` refers to that container, not another service.
- **CORS Errors**: Do not add CORS headers in the proxy; Pulse handles them. Set **Settings → System → Network → Allowed Origins** or use `ALLOWED_ORIGINS` if needed.
- **OIDC redirects fail**: Ensure `X-Forwarded-Proto` is set (see above).
- **Wrong client IPs**: Check the immediate peer and header replacement above. Do not broaden proxy trust or change authentication to repair attribution.
