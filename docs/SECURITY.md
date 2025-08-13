# Pulse Security

## Smart Security Context (v4.3.2+)

### Public Access Detection
Pulse automatically detects when it's being accessed from public networks:
- **Private Networks**: Local/RFC1918 addresses (192.168.x.x, 10.x.x.x, etc.)
- **Public Networks**: Any non-private IP address
- **Stronger Warnings**: Red alerts when accessed from public IPs without authentication

### Trusted Networks Configuration
Define networks that don't require authentication:
```bash
# Environment variable (comma-separated CIDR blocks)
PULSE_TRUSTED_NETWORKS=192.168.1.0/24,10.0.0.0/24

# Or in systemd
sudo systemctl edit pulse-backend
[Service]
Environment="PULSE_TRUSTED_NETWORKS=192.168.1.0/24,10.0.0.0/24"
```

When configured:
- Access from trusted networks: No auth required
- Access from outside: Authentication enforced
- Useful for: Mixed home/remote access scenarios

## Security Warning System

Pulse now includes a non-intrusive security warning system that helps you understand your security posture:

### Security Score
Your instance receives a score from 0-5 based on:
- ✅ Credentials encrypted at rest (always enabled)
- ✅ Export/import protection
- ⚠️ Authentication enabled
- ⚠️ HTTPS connection
- ⚠️ Audit logging

### Dismissing Warnings
If you're comfortable with your security setup, you can dismiss warnings:
- **For 1 day** - Reminder tomorrow
- **For 1 week** - Reminder next week  
- **Forever** - Won't show again

To permanently disable all security warnings:
```bash
# Environment variable
PULSE_DISABLE_SECURITY_WARNINGS=true
```

## Credential Security

- **Storage**: Encrypted at rest using AES-256-GCM (`/etc/pulse/nodes.enc`)
- **Logs**: Token values masked with `***` in all outputs
- **API**: Frontend receives only `hasToken: true`, never actual values
- **Export**: Requires API_TOKEN authentication to extract credentials
- **Migration**: Use passphrase-protected export/import (see [Migration Guide](MIGRATION.md))

## Export/Import Protection

By default, configuration export/import is blocked for security. You have two options:

### Option 1: Set API Token (Recommended)
```bash
# Using systemd (secure)
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="API_TOKEN=your-secure-token-here"

# Then restart:
sudo systemctl restart pulse-backend

# Docker
docker run -e API_TOKEN=your-token rcourtman/pulse:latest
```

### Option 2: Allow Unprotected Export (Homelab)
```bash
# Using systemd
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="ALLOW_UNPROTECTED_EXPORT=true"

# Docker
docker run -e ALLOW_UNPROTECTED_EXPORT=true rcourtman/pulse:latest
```

**Note:** Never put API tokens or passwords in .env files! Use systemd environment variables or Docker secrets for sensitive data.

## Security Features

### Core Protection
- **Encryption**: All credentials encrypted at rest (AES-256-GCM)
- **Export Protection**: Exports always encrypted with passphrase
- **Minimum Passphrase**: 12 characters required for exports
- **Security Tab**: Check status in Settings → Security

### Enterprise Security (When Authentication Enabled)
- **CSRF Protection**: All state-changing operations require CSRF tokens
- **Rate Limiting**: 
  - General API: 500 requests/minute
  - Authentication: 10 attempts/minute
  - Export/Import: 5 attempts/minute
- **Account Lockout**: Locks after 5 failed login attempts (15 minute cooldown)
- **Session Management**: 
  - Secure HttpOnly cookies
  - 24-hour session expiry
  - Session invalidation on password change
- **Password Security**: bcrypt hashing with cost 12
- **Security Headers**: 
  - Content-Security-Policy
  - X-Frame-Options: DENY
  - X-Content-Type-Options: nosniff
  - X-XSS-Protection
- **Audit Logging**: All authentication events logged

### What's Encrypted in Exports
- Node credentials (passwords, API tokens)
- PBS credentials
- Email settings passwords

### What's NOT Encrypted
- Node hostnames and IPs
- Threshold settings
- General configuration

## Authentication

Pulse supports multiple authentication methods that can be used independently or together:

### Password Authentication
Protect your Pulse instance with a password. Passwords are automatically hashed with bcrypt for security.

```bash
# Using systemd
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="PULSE_PASSWORD=your-secure-password"

# Docker
docker run -e PULSE_PASSWORD=your-password rcourtman/pulse:latest
```

#### Features
- Web UI login required when password is set
- Change password from Settings → Security  
- Passwords hashed with bcrypt (cost 12)
- Session-based authentication with secure HttpOnly cookies
- 24-hour session expiry
- CSRF protection for all state-changing operations
- Session invalidation on password change

### API Token Authentication  
For programmatic access and automation. Tokens can be generated and managed via the web UI.

```bash
# Using systemd
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="API_TOKEN=your-secure-token"

# Docker
docker run -e API_TOKEN=your-token rcourtman/pulse:latest
```

#### Token Management (Settings → Security → API Token)
- Generate new tokens via web UI when authenticated
- View existing token anytime (authenticated users only)
- Regenerate tokens without disrupting service
- Delete tokens to disable API access

#### Usage
```bash
# Include token in X-API-Token header
curl -H "X-API-Token: your-token" http://localhost:7655/api/health
```

### Auto-Registration Security

#### Default Mode (Homelab Friendly)
- Nodes can auto-register without authentication
- Suitable for trusted networks
- Setup scripts work without additional configuration

#### Secure Mode
- Require API token for all operations
- Protects auto-registration endpoint
- Enable by setting API_TOKEN environment variable

## Troubleshooting

**Export blocked?** Set API_TOKEN or ALLOW_UNPROTECTED_EXPORT=true
**Rate limited?** Wait 1 minute and try again
**Can't login?** Check PULSE_PASSWORD environment variable
**API access denied?** Verify API_TOKEN is correct