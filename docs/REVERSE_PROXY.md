# Reverse Proxy Configuration

Pulse uses WebSockets for real-time updates. Your reverse proxy **MUST** support WebSocket connections or Pulse will not work correctly.

## Important Requirements

1. **WebSocket Support Required** - Enable WebSocket proxying
2. **Proxy Headers** - Forward original host and IP headers
3. **Timeouts** - Increase timeouts for long-lived connections
4. **Buffer Sizes** - Increase for large state updates (64KB recommended)

## Authentication with Reverse Proxy

If you're using authentication at the reverse proxy level (Authentik, Authelia, etc.), you can disable Pulse's built-in authentication to avoid double login prompts:

```bash
# In your .env file or environment
DISABLE_AUTH=true
```

When `DISABLE_AUTH=true` is set:
- Pulse's built-in authentication is completely bypassed
- All endpoints become accessible without authentication
- The reverse proxy handles all authentication and authorization
- A warning is logged on startup to confirm auth is disabled

⚠️ **Warning**: Only use `DISABLE_AUTH=true` if your reverse proxy provides authentication. Never expose Pulse directly to the internet with authentication disabled.

## Nginx

```nginx
server {
    listen 80;
    server_name pulse.example.com;
    
    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name pulse.example.com;
    
    # SSL configuration
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    # Proxy settings
    location / {
        proxy_pass http://localhost:7655;
        proxy_http_version 1.1;
        
        # Required for WebSocket
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts for WebSocket
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
        
        # Disable buffering for real-time updates
        proxy_buffering off;
        
        # Increase buffer sizes for large messages
        proxy_buffer_size 64k;
        proxy_buffers 8 64k;
        proxy_busy_buffers_size 128k;
    }
    
    # API endpoints (optional, same config as above)
    location /api/ {
        proxy_pass http://localhost:7655/api/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
        proxy_buffering off;
    }
}
```

## Caddy v2

Caddy automatically handles WebSocket upgrades when reverse proxying.

```caddy
pulse.example.com {
    reverse_proxy localhost:7655
}
```

For more control:

```caddy
pulse.example.com {
    reverse_proxy localhost:7655 {
        # Headers automatically handled by Caddy
        header_up Host {host}
        header_up X-Real-IP {remote}
        header_up X-Forwarded-For {remote}
        header_up X-Forwarded-Proto {scheme}
        
        # Increase timeouts for WebSocket
        transport http {
            dial_timeout 30s
            response_header_timeout 30s
            read_timeout 0
        }
    }
}
```

## Apache

```apache
<VirtualHost *:443>
    ServerName pulse.example.com
    
    SSLEngine on
    SSLCertificateFile /path/to/cert.pem
    SSLCertificateKeyFile /path/to/key.pem
    
    # Enable necessary modules:
    # a2enmod proxy proxy_http proxy_wstunnel headers
    
    # WebSocket proxy
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://localhost:7655/$1" [P,L]
    
    # Regular HTTP proxy
    ProxyPass / http://localhost:7655/
    ProxyPassReverse / http://localhost:7655/
    
    # Preserve host headers
    ProxyPreserveHost On
    
    # Forward real IP
    RequestHeader set X-Real-IP "%{REMOTE_ADDR}s"
    RequestHeader set X-Forwarded-For "%{REMOTE_ADDR}s"
    RequestHeader set X-Forwarded-Proto "https"
    
    # Disable buffering
    ProxyIOBufferSize 65536
</VirtualHost>
```

## Traefik

```yaml
# docker-compose.yml
services:
  pulse:
    image: rcourtman/pulse:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.pulse.rule=Host(`pulse.example.com`)"
      - "traefik.http.routers.pulse.tls=true"
      - "traefik.http.services.pulse.loadbalancer.server.port=7655"
      # WebSocket support is automatic in Traefik 2.x+
```

Or using Traefik file configuration:

```yaml
# traefik-dynamic.yml
http:
  routers:
    pulse:
      rule: "Host(`pulse.example.com`)"
      service: pulse
      tls: {}
      
  services:
    pulse:
      loadBalancer:
        servers:
          - url: "http://localhost:7655"
```

## HAProxy

```haproxy
frontend https
    bind *:443 ssl crt /path/to/cert.pem
    
    # ACL for Pulse
    acl host_pulse hdr(host) -i pulse.example.com
    
    # WebSocket detection
    acl is_websocket hdr(Upgrade) -i websocket
    
    # Use backend
    use_backend pulse if host_pulse

backend pulse
    # Health check
    option httpchk GET /api/health
    
    # WebSocket support
    option http-server-close
    option forwardfor
    
    # Timeouts for WebSocket
    timeout client 3600s
    timeout server 3600s
    timeout tunnel 3600s
    
    # Backend server
    server pulse1 localhost:7655 check
```

## Cloudflare Tunnel

If using Cloudflare Tunnel (cloudflared):

```yaml
# config.yml
tunnel: YOUR_TUNNEL_ID
credentials-file: /path/to/credentials.json

ingress:
  - hostname: pulse.example.com
    service: http://localhost:7655
    originRequest:
      # Enable WebSocket
      noTLSVerify: false
      connectTimeout: 30s
      # No additional config needed - WebSockets work by default
  - service: http_status:404
```

## Testing WebSocket Connection

After configuring your reverse proxy, test that WebSockets work:

```bash
# Test basic connectivity
curl https://pulse.example.com/api/health

# Test WebSocket upgrade (should return 101 Switching Protocols)
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  https://pulse.example.com/api/ws
```

In browser console (F12):
```javascript
// Test WebSocket connection
const ws = new WebSocket('wss://pulse.example.com/api/ws');
ws.onopen = () => console.log('WebSocket connected!');
ws.onmessage = (e) => console.log('Received:', e.data);
ws.onerror = (e) => console.error('WebSocket error:', e);
```

## Common Issues

### "Connection Lost" or no real-time updates
- WebSocket upgrade not configured correctly
- Check proxy passes `Upgrade` and `Connection` headers
- Verify timeouts are increased for long connections

### CORS errors
- Pulse handles CORS internally
- Don't add additional CORS headers in proxy
- If needed, set `ALLOWED_ORIGINS` in Pulse configuration

### 502 Bad Gateway
- Pulse not running on expected port (default 7655)
- Check with: `curl http://localhost:7655/api/health`
- Verify Pulse service: `systemctl status pulse` (use `pulse-backend` if you're on a legacy unit)

### WebSocket closes immediately
- Timeout too short in proxy configuration
- Increase `proxy_read_timeout` (Nginx) or equivalent
- Set to at least 3600s (1 hour) or more

## Security Recommendations

1. **Always use HTTPS** for production deployments
2. **Set proper headers** to prevent clickjacking:
   ```nginx
   add_header X-Frame-Options "SAMEORIGIN";
   add_header X-Content-Type-Options "nosniff";
   ```
3. **Rate limiting** for API endpoints:
   ```nginx
   limit_req_zone $binary_remote_addr zone=pulse:10m rate=30r/s;
   limit_req zone=pulse burst=50 nodelay;
   ```
4. **Hide proxy version**:
   ```nginx
   proxy_hide_header X-Powered-By;
   server_tokens off;
   ```

## Support

If WebSockets still don't work after following this guide:
1. Check browser console for errors (F12)
2. Verify Pulse logs: `journalctl -u pulse -f`
3. Test without proxy first: `http://your-server:7655`
4. Report issues: https://github.com/rcourtman/Pulse/issues
