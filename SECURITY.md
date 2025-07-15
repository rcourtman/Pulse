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

# CSRF Protection is automatic in private mode
# No configuration needed - it's always enabled for security

# Trust proxy configuration (for reverse proxy setups)
# Set to '1' for single proxy, 'true' for all proxies
TRUST_PROXY=false

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

### CSRF Protection
In Private mode, Pulse implements CSRF protection using a double-submit cookie pattern:
- Tokens are automatically generated for authenticated sessions
- Required for all state-changing operations (POST, PUT, DELETE)
- Tokens included in response headers and login responses
- API integrations using Basic Auth are exempt

### Brute Force Protection
- Account lockout after failed attempts
- Configurable attempt limits and lockout duration
- Rate limiting on authentication endpoints

### Audit Logging
When enabled (`AUDIT_LOG=true`), Pulse logs:
- Authentication attempts (success/failure)
- Configuration changes
- Service restarts
- Security mode changes
- CSRF validation failures
- Failed authorization attempts

## Best Practices

### For Home Labs / Trusted Networks
1. **Public mode is fine** if you trust everyone on your network
2. **Use network isolation** - Keep Pulse on a management VLAN
3. **Firewall rules** - Restrict access to port 7655 from untrusted networks

### For Any Other Environment
1. **Always use Private mode** - Authentication should be required
2. **Strong passwords** - Use a password manager
3. **Configure trust proxy** - Set `TRUST_PROXY=1` when behind a reverse proxy
4. **HTTPS required** - Use a reverse proxy with SSL certificates:

```nginx
# nginx example with proper headers
server {
    listen 443 ssl;
    server_name pulse.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:7655;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

Remember to set `TRUST_PROXY=1` in your Pulse configuration when using this setup.

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
1. Use Private mode with CSRF protection
2. Set up HTTPS via reverse proxy
3. Configure `TRUST_PROXY` setting
4. Consider using a VPN instead of direct exposure

**Q: What is CSRF protection?**  
A: CSRF (Cross-Site Request Forgery) protection prevents malicious websites from making unauthorized requests on behalf of authenticated users. Pulse uses a double-submit cookie pattern that requires a secret token for all state-changing operations.

**Q: When should I configure TRUST_PROXY?**  
A: Set `TRUST_PROXY=1` when Pulse is behind a single reverse proxy (most common). This ensures correct client IP logging, HTTPS detection, and proper session security. For multiple proxies, set the number of proxies or specific IPs to trust.

## Security Hardening Checklist

For production use:
- [ ] Set `SECURITY_MODE=private`
- [ ] Configure strong `ADMIN_PASSWORD`
- [ ] Generate random `SESSION_SECRET` (64+ characters)
- [ ] Enable `AUDIT_LOG=true`
- [ ] Use HTTPS via reverse proxy
- [ ] Configure `TRUST_PROXY` appropriately
- [ ] Verify CSRF protection is enabled
- [ ] Configure firewall rules
- [ ] Set appropriate `SESSION_TIMEOUT_HOURS`
- [ ] Monitor audit logs
- [ ] Review and adjust rate limits if needed
- [ ] Test authentication and CSRF protection

## Support

For security-related questions:
- Review this guide
- Check [GitHub Issues](https://github.com/rcourtman/Pulse/issues)
- For security vulnerabilities, please report privately via GitHub Security tab

## API Token Permissions and Security

### Understanding the Permission Requirements

Pulse requires specific permissions to monitor your Proxmox infrastructure:

#### Basic Monitoring (VMs, Containers, Nodes)
- **Required**: `PVEAuditor` role on `/`
- **Risk Level**: Low - This is a read-only role

#### Backup Monitoring
- **Required**: `PVEDatastoreAdmin` role on `/storage` or specific storages
- **Risk Level**: Medium to High - Includes write permissions

### The PVEDatastoreAdmin Security Concern

**Important Security Notice**: Due to Proxmox VE API limitations, viewing backup content requires `PVEDatastoreAdmin`, which grants these permissions:

**What Pulse Actually Uses:**
- ✅ List backup files (`Datastore.Audit`)
- ✅ View backup metadata

**What PVEDatastoreAdmin Allows:**
- ⚠️ Delete backup files (`Datastore.Allocate`)
- ⚠️ Create/modify datastores
- ⚠️ Upload ISOs and templates
- ⚠️ Consume storage space

This is a [known Proxmox limitation](https://forum.proxmox.com/threads/listing-storage-content-requires-write-permissions.122211/) where read-only access to backup content isn't possible.

### Risk Mitigation Strategies

#### 1. Minimize Permission Scope
Instead of granting permissions on `/storage` (all storages):
```bash
# Better: Grant only to specific backup storages
pveum acl modify /storage/backup-storage --users pulse@pam --roles PVEDatastoreAdmin
pveum acl modify /storage/nfs-backup --users pulse@pam --roles PVEDatastoreAdmin
```

#### 2. Use Dedicated Backup Storage
Create storage exclusively for backups:
- Reduces risk if token is compromised
- Limits potential damage to backup data only
- No risk to VM disks or ISO storage

#### 3. Prefer PBS for New Deployments
Proxmox Backup Server has proper read-only permissions:
```bash
# PBS only needs DatastoreAudit (truly read-only)
proxmox-backup-manager acl update /datastore DatastoreAudit --auth-id 'pulse@pbs!monitoring'
```

#### 4. Network Security
- Run Pulse on an isolated management network
- Use firewall rules to restrict Pulse's outbound connections
- Ensure Pulse cannot be accessed from untrusted networks

#### 5. Token Security Best Practices
- **Regular Rotation**: Rotate API tokens every 90 days
- **Audit Logs**: Monitor `/var/log/pveproxy/access.log` for token usage
- **Unique Tokens**: Use dedicated tokens for Pulse, not shared with other tools
- **Secure Storage**: Store tokens encrypted, never in plain text

#### 6. Alternative Approaches
If the write permissions are unacceptable for your security policy:
- Monitor only VMs/containers (skip backup monitoring)
- Use PBS exclusively for backups (proper read-only permissions)
- Consider using Proxmox's built-in backup job notifications instead

### Monitoring Token Usage

Check API token activity:
```bash
# View recent API token usage
grep "pulse@pam!token" /var/log/pveproxy/access.log | tail -20

# Monitor for suspicious activity (deletes, uploads)
grep "pulse@pam!token" /var/log/pveproxy/access.log | grep -E "(DELETE|POST|PUT)"
```

### Security Incident Response

If you suspect token compromise:
1. **Immediately revoke the token**: `pveum user token remove pulse@pam token-name`
2. **Check logs** for unauthorized actions
3. **Verify backup integrity**
4. **Create new token with minimal required permissions**
5. **Update Pulse configuration**

Remember: While Pulse only reads data, the required permissions allow write access due to Proxmox API design. Always follow the principle of least privilege and implement appropriate compensating controls.