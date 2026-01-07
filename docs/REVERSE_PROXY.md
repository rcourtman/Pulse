# 🔄 Reverse Proxy Setup

Pulse uses WebSockets for real-time updates. Your proxy **MUST** support WebSockets.

## ⚡ Quick Configs

### Nginx
```nginx
location / {
    proxy_pass http://localhost:7655;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Critical for WebSockets
    proxy_read_timeout 86400; # 24h
}
```

### Caddy
```caddy
pulse.example.com {
    reverse_proxy localhost:7655
}
```

### Traefik (Docker Compose)
```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.pulse.rule=Host(`pulse.example.com`)"
  - "traefik.http.services.pulse.loadbalancer.server.port=7655"
```

### Apache
```apache
RewriteEngine On
RewriteCond %{HTTP:Upgrade} websocket [NC]
RewriteCond %{HTTP:Connection} upgrade [NC]
RewriteRule ^/?(.*) "ws://localhost:7655/$1" [P,L]

ProxyPass / http://localhost:7655/
ProxyPassReverse / http://localhost:7655/
```

---

## ⚠️ Common Issues

### "HTTPS: HTTP only" in Security Posture

If your reverse proxy terminates SSL but Pulse shows "HTTPS: HTTP only" in Settings → Security:

**Cause**: Pulse detects HTTPS in two ways:
1. Direct TLS connection (`req.TLS != nil`)
2. The `X-Forwarded-Proto: https` header

If your proxy terminates SSL but doesn't forward this header, Pulse sees plain HTTP.

**Fix**: Add the `X-Forwarded-Proto` header in your proxy config:

```nginx
# Nginx
proxy_set_header X-Forwarded-Proto $scheme;
```

```caddy
# Caddy (automatic, but explicit override if needed)
header_up X-Forwarded-Proto {scheme}
```

```apache
# Apache
RequestHeader set X-Forwarded-Proto "https"
```

### Other Issues

- **"Connection Lost"**: WebSocket upgrade failed. Check `Upgrade` and `Connection` headers.
- **502 Bad Gateway**: Pulse is not running on port 7655.
- **CORS Errors**: Do not add CORS headers in the proxy; Pulse handles them. Set **Settings → System → Network → Allowed Origins** or use `ALLOWED_ORIGINS` if needed.
- **OIDC redirects fail**: Ensure `X-Forwarded-Proto` is set (see above).
- **Wrong client IPs**: Set `PULSE_TRUSTED_PROXY_CIDRS` to your proxy IP/CIDR so `X-Forwarded-For` is trusted.
