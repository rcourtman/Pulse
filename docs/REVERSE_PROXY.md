# Reverse Proxy Configuration Guide

This guide explains how to properly configure Pulse behind various reverse proxies.

## Overview

Pulse supports deployment behind reverse proxies with proper configuration. The key requirements are:

1. **Trust Proxy Settings** - Configure Pulse to trust X-Forwarded headers
2. **Header Forwarding** - Ensure your proxy forwards necessary headers
3. **WebSocket Support** - Enable WebSocket proxying for real-time updates
4. **HTTPS Termination** - Handle SSL/TLS at the proxy level

## Configuration via Settings UI (Recommended)

**No manual file editing required!** Configure proxy settings through the Pulse UI:

1. Click the **Settings** button (⚙️ gear icon) in Pulse
2. Navigate to the **System/Advanced** tab
3. Find the **Reverse Proxy Configuration** section
4. Select your **Trust Proxy** setting:
   - **Disabled (Direct connection)** - No reverse proxy
   - **Behind 1 proxy** - Single proxy like Nginx, Caddy, or Apache
   - **Behind 2 proxies** - CDN + proxy (e.g., Cloudflare + Nginx)
   - **Trust all proxies** - ⚠️ Only use on fully trusted networks
   - **Custom** - Specify exact IPs or subnets to trust
5. Save your changes

The UI automatically handles all configuration - no need to edit .env files!

## Manual Configuration (Alternative)

If you prefer environment variables or need to configure via Docker/deployment scripts:

```env
# Trust proxy settings
TRUST_PROXY=1  # For single proxy (most common)
# or
TRUST_PROXY=2  # For CDN + proxy setup
# or
TRUST_PROXY=10.0.0.0/8,172.16.0.0/12  # Custom IPs/subnets
```

## Nginx Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name pulse.example.com;

    # SSL configuration
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # Security headers
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;

    location / {
        proxy_pass http://localhost:7655;
        
        # Required headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Timeouts for long-running connections
        proxy_read_timeout 86400;
        
        # Optional: Increase buffer sizes for large responses
        proxy_buffer_size 128k;
        proxy_buffers 4 256k;
        proxy_busy_buffers_size 256k;
    }
}
```

## Caddy Configuration

Caddy automatically handles most proxy requirements. Basic configuration:

```caddyfile
pulse.example.com {
    reverse_proxy localhost:7655
}
```

For advanced configuration with headers:

```caddyfile
pulse.example.com {
    reverse_proxy localhost:7655 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }
}
```

## Apache Configuration

```apache
<VirtualHost *:443>
    ServerName pulse.example.com
    
    SSLEngine on
    SSLCertificateFile /path/to/cert.pem
    SSLCertificateKeyFile /path/to/key.pem
    
    ProxyPreserveHost On
    ProxyRequests Off
    
    # Main proxy
    ProxyPass / http://localhost:7655/
    ProxyPassReverse / http://localhost:7655/
    
    # WebSocket proxy
    RewriteEngine on
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://localhost:7655/$1" [P,L]
    
    # Headers
    RequestHeader set X-Forwarded-Proto "https"
    RequestHeader set X-Forwarded-Port "443"
</VirtualHost>
```

## Traefik Configuration

Docker Compose example:

```yaml
version: '3'

services:
  traefik:
    image: traefik:v3.0
    command:
      - "--providers.docker=true"
      - "--entrypoints.websecure.address=:443"
    ports:
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  pulse:
    image: geekdojo/pulse:latest
    environment:
      - TRUST_PROXY=1
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.pulse.rule=Host(`pulse.example.com`)"
      - "traefik.http.routers.pulse.entrypoints=websecure"
      - "traefik.http.routers.pulse.tls=true"
      - "traefik.http.services.pulse.loadbalancer.server.port=7655"
```

## HAProxy Configuration

```haproxy
frontend https_front
    bind *:443 ssl crt /etc/haproxy/certs/pulse.pem
    option forwardfor
    
    # Add headers
    http-request set-header X-Forwarded-Proto https
    
    default_backend pulse_back

backend pulse_back
    server pulse1 localhost:7655 check
    
    # WebSocket support
    option http-server-close
    timeout tunnel 1h
```

## Common Issues and Solutions

### Issue: "Failed to save configuration: Invalid CSRF token"

**Cause**: The CSRF protection is rejecting requests because headers aren't being forwarded properly.

**Solution**: 
1. Ensure `TRUST_PROXY` is configured correctly
2. Verify your proxy is forwarding all required headers
3. Check that cookies are being passed through

### Issue: Real-time updates not working

**Cause**: WebSocket connections are being blocked or not proxied correctly.

**Solution**: Ensure your proxy configuration includes WebSocket support (see examples above).

### Issue: Redirect loops or incorrect URLs

**Cause**: The application doesn't know its public URL when behind a proxy.

**Solution**: Set the `PULSE_PUBLIC_URL` environment variable to your public URL.

### Issue: Session cookies not working

**Cause**: Cookie security settings conflict with proxy setup.

**Solution**: 
- For HTTPS proxies: Ensure `X-Forwarded-Proto: https` header is set
- For iframe embedding: Configure `ALLOW_EMBEDDING=true`

## Security Considerations

1. **Always use HTTPS** at the proxy level for production deployments
2. **Limit trusted proxies** - Don't use `TRUST_PROXY=true` in production
3. **Set security headers** at the proxy level for defense in depth
4. **Use strong SSL/TLS settings** (TLS 1.2+ only, strong ciphers)
5. **Consider rate limiting** at the proxy level

## Testing Your Configuration

After setting up your reverse proxy:

1. **Test basic access**: Navigate to your Pulse URL
2. **Test authentication**: Try logging in (if using private mode)
3. **Test configuration saving**: Make a configuration change and save
4. **Test real-time updates**: Open two browser tabs and verify changes appear in both
5. **Test WebSocket connection**: Check browser console for WebSocket errors

### Debug Headers

To verify headers are being forwarded correctly, you can temporarily enable debug logging:

```env
DEBUG=pulse:*
```

Then check the logs for incoming request headers.

## Example Docker Compose with Nginx

Complete example with Nginx proxy:

```yaml
version: '3'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./certs:/etc/nginx/certs
    depends_on:
      - pulse

  pulse:
    image: geekdojo/pulse:latest
    environment:
      - TRUST_PROXY=1
      - PROXMOX_HOST=192.168.1.100
      - PROXMOX_TOKEN_ID=monitor@pve!pulse
      - PROXMOX_TOKEN_SECRET=your-secret-here
    volumes:
      - pulse-data:/opt/pulse/data
      - pulse-config:/config

volumes:
  pulse-data:
  pulse-config:
```

## Support

If you're having issues with reverse proxy configuration:

1. Check the [troubleshooting section](#common-issues-and-solutions)
2. Enable debug logging to see request details
3. Verify your proxy is forwarding headers correctly
4. Open an issue with your proxy configuration (sanitized)