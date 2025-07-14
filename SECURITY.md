# Pulse Security Guide

## Overview

Pulse is designed as an internal monitoring tool for Proxmox environments. It provides two simple security modes to match your deployment needs.

## Security Modes

Pulse supports two security modes configured via the `SECURITY_MODE` environment variable:

### 1. Public Mode
- **Use Case**: Trusted internal networks, home labs, development environments
- **Behavior**: No authentication required - anyone can access Pulse
- **Configuration**: `SECURITY_MODE=public`
- **Warning**: Only use on fully trusted networks

### 2. Private Mode (Default)
- **Use Case**: Any environment where access control is needed
- **Behavior**: Authentication required - must login to access Pulse
- **Configuration**: `SECURITY_MODE=private` (or not set)
- **Recommended**: Use this mode unless you fully trust your network

## Quick Start Security Setup

### For Trusted Home Networks

If you completely trust your network (e.g., home lab with no outside access):

```env
SECURITY_MODE=public
```

### For Everything Else (Recommended)

```env
SECURITY_MODE=private
ADMIN_PASSWORD=your-secure-password-here
SESSION_SECRET=your-random-64-char-secret
```

**First run** - If no admin password is set, a temporary one will be generated and displayed in the console.

## Authentication

When running in Private mode:

- **Username**: `admin`
- **Password**: Set via `ADMIN_PASSWORD` environment variable
- **Session timeout**: 24 hours (configurable)
- **Basic Auth**: Supported for automation (e.g., curl scripts)

## Environment Variables

### Essential Security Settings
```env
# Security mode: public or private
SECURITY_MODE=private

# Admin password (required for private mode)
ADMIN_PASSWORD=your-secure-password

# Session secret (required for private mode, 64+ characters)
SESSION_SECRET=your-random-session-secret

# Session timeout in hours (default: 24)
SESSION_TIMEOUT_HOURS=24

# Enable audit logging
AUDIT_LOG=true
```

### Advanced Security Settings
```env
# Password hashing rounds (higher = more secure but slower)
BCRYPT_ROUNDS=10

# Login attempt limits
MAX_LOGIN_ATTEMPTS=5
LOCKOUT_DURATION=900000  # 15 minutes in ms
```

## Security Features

### Automatic Security Headers
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY` (unless iframe embedding is enabled)
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy` with strict directives

### Session Security
- HTTPOnly cookies (no JavaScript access)
- Secure cookies in production
- SameSite protection
- Configurable session timeout

### Brute Force Protection
- Account lockout after failed attempts
- Configurable attempt limits and lockout duration

### Audit Logging
When enabled (`AUDIT_LOG=true`), Pulse logs:
- Authentication attempts (success/failure)
- Configuration changes
- Service restarts
- Security mode changes

## Best Practices

### For Home Labs / Trusted Networks
1. **Public mode is fine** if you trust everyone on your network
2. **Use network isolation** - Keep Pulse on a management VLAN
3. **Firewall rules** - Restrict access to port 7655 from untrusted networks

### For Any Other Environment
1. **Always use Private mode** - Authentication should be required
2. **Strong passwords** - Use a password manager
3. **HTTPS recommended** - Use a reverse proxy with SSL certificates:

```nginx
# nginx example
server {
    listen 443 ssl;
    server_name pulse.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:7655;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Common Scenarios

### Scenario 1: Home Lab (Trusted Network)
```env
SECURITY_MODE=public
# That's it! No auth needed on trusted network
```

### Scenario 2: Home with Port Forwarding
```env
SECURITY_MODE=private
ADMIN_PASSWORD=strong-password-here
SESSION_SECRET=random-64-character-string
AUDIT_LOG=true
```

### Scenario 3: Small Business / Shared Environment
```env
SECURITY_MODE=private
ADMIN_PASSWORD=very-strong-password
SESSION_SECRET=random-64-character-string
SESSION_TIMEOUT_HOURS=8  # Shorter sessions
AUDIT_LOG=true
MAX_LOGIN_ATTEMPTS=3     # Stricter limits
```

## FAQ

**Q: I forgot my admin password. How do I reset it?**  
A: Stop Pulse, remove `ADMIN_PASSWORD` from `.env`, restart Pulse. A new temporary password will be shown in the console.

**Q: Can I use Pulse with my existing authentication system?**  
A: Yes! Place Pulse behind a reverse proxy that handles authentication (OAuth2 proxy, Authelia, Authentik, etc.).

**Q: Is Public mode really safe for home use?**  
A: If your network is isolated and you trust all devices/users on it, yes. If you have IoT devices, guests, or any untrusted devices, use Private mode.

**Q: Why is there no user management?**  
A: Pulse is designed to be simple. For multi-user setups, use a reverse proxy with your preferred auth system.

**Q: Can I access Pulse remotely?**  
A: Yes, but we recommend:
1. Use Private mode
2. Set up HTTPS via reverse proxy
3. Consider using a VPN instead of direct exposure

## Security Hardening Checklist

For production use:
- [ ] Set `SECURITY_MODE=private`
- [ ] Configure strong `ADMIN_PASSWORD`
- [ ] Generate random `SESSION_SECRET`
- [ ] Enable `AUDIT_LOG=true`
- [ ] Use HTTPS (reverse proxy)
- [ ] Configure firewall rules
- [ ] Set appropriate `SESSION_TIMEOUT_HOURS`
- [ ] Monitor audit logs

## Support

For security-related questions:
- Review this guide
- Check [GitHub Issues](https://github.com/rcourtman/Pulse/issues)
- For security vulnerabilities, please report privately via GitHub Security tab

Remember: Pulse is a monitoring tool that only reads data from Proxmox. The main security concern is preventing unauthorized users from viewing your infrastructure details.