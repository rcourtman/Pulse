# Pulse Security

## Mandatory Authentication

**Starting with v4.5.0, authentication setup is prompted for all new Pulse installations.** This protects your Proxmox API credentials from unauthorized access.

### First-Run Security Setup
When you first access Pulse, you'll be guided through a mandatory security setup:
- Create your admin username and password
- Automatic API token generation for automation
- Settings are applied immediately without restart
- **Your existing nodes and settings are preserved**

## Smart Security Context

### Public Access Detection
Pulse automatically detects when it's being accessed from public networks:
- **Private Networks**: Local/RFC1918 addresses (192.168.x.x, 10.x.x.x, etc.)
- **Public Networks**: Any non-private IP address
- **Stronger Warnings**: Red alerts when accessed from public IPs without authentication

### Trusted Networks Configuration (Deprecated)
**Note: Authentication is now mandatory regardless of network location.**

Legacy configuration (no longer applicable):
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

## Credential Security

### Encrypted at Rest (AES-256-GCM)
- **Node Credentials**: Passwords and API tokens (`/etc/pulse/nodes.enc`)
- **Email Settings**: SMTP passwords (`/etc/pulse/email.enc`)  
- **Webhook Data**: URLs and auth headers (`/etc/pulse/webhooks.enc`) - v4.1.9+
- **Encryption Key**: Auto-generated (`/etc/pulse/.encryption.key`)

### Security Features
- **Logs**: Token values masked with `***` in all outputs
- **API**: Frontend receives only `hasToken: true`, never actual values
- **Export**: Requires API_TOKEN authentication to extract credentials
- **Migration**: Use passphrase-protected export/import (see [Migration Guide](MIGRATION.md))
- **Auto-Migration**: Unencrypted configs automatically migrate to encrypted format

## Export/Import Protection

By default, configuration export/import is blocked for security. You have two options:

### Option 1: Set API Token (Recommended)
```bash
# Using systemd (secure)
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="API_TOKEN=your-48-char-hex-token"

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

**Note:** For production deployments, consider using Docker secrets or systemd environment variables instead of .env files for sensitive data.

## Security Features

### Core Protection
- **Encryption**: All credentials encrypted at rest (AES-256-GCM)
- **Export Protection**: Exports always encrypted with passphrase
- **Minimum Passphrase**: 12 characters required for exports
- **Security Tab**: Check status in Settings → Security

### Enterprise Security (When Authentication Enabled)
- **Password Security**: 
  - Bcrypt hashing with cost factor 12 (60-character hash)
  - Passwords NEVER stored in plain text
  - Automatic hashing on security setup
  - **CRITICAL**: Bcrypt hashes MUST be exactly 60 characters
- **API Token Security**:
  - 64-character hex tokens (32 bytes of entropy)
  - SHA3-256 hashed before storage (64-char hash)
  - Raw token shown only once during generation
  - Tokens NEVER stored in plain text
  - Live reloading when .env file changes
  - API-only mode supported (no password auth required)
- **CSRF Protection**: All state-changing operations require CSRF tokens
- **Rate Limiting**: 
  - Authentication endpoints: 10 attempts/minute per IP
  - General API: 500 requests/minute per IP
  - Real-time endpoints exempt for functionality
- **Account Lockout Protection**:
  - Locks after 5 failed login attempts
  - 15-minute automatic lockout duration
  - Clear feedback showing remaining attempts
  - Time remaining displayed when locked
  - Manual reset available via API for administrators
- **Session Management**: 
  - Secure HttpOnly cookies
  - 24-hour session expiry
  - Session invalidation on password change
- **Security Headers**: 
  - Content-Security-Policy with strict directives
  - X-Frame-Options: DENY (prevents clickjacking)
  - X-Content-Type-Options: nosniff
  - X-XSS-Protection: 1; mode=block
  - Referrer-Policy: strict-origin-when-cross-origin
  - Permissions-Policy restricting sensitive APIs
- **Audit Logging**: All authentication events logged with IP addresses

### What's Encrypted in Exports
- Node credentials (passwords, API tokens)
- PBS credentials
- Email settings passwords
- Webhook URLs and authentication headers (v4.1.9+)

### What's NOT Encrypted
- Node hostnames and IPs
- Threshold settings
- General configuration
- Alert rules and schedules

## Authentication

Pulse supports multiple authentication methods that can be used independently or together:

### Password Authentication

#### Quick Security Setup (Recommended)
The easiest way to enable authentication is through the web UI:
1. Go to Settings → Security
2. Click "Enable Security Now"
3. Enter username and password
4. Save the generated API token (shown only once!)
5. Security is enabled immediately (no restart needed)

This automatically:
- Generates a secure random password
- Hashes it with bcrypt (cost factor 12)
- Creates secure API token (SHA3-256 hashed, raw token shown once)
- For systemd: Configures systemd with hashed credentials
- For Docker: Saves to `/data/.env` with hashed credentials (properly quoted to prevent shell expansion)
- Restarts service/container with authentication enabled

#### Manual Setup (Advanced)
```bash
# Using systemd (password will be hashed automatically)
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="PULSE_AUTH_USER=admin"
Environment="PULSE_AUTH_PASS=$2a$12$..."  # Use bcrypt hash, not plain text!

# Docker (credentials persist in volume via .env file)
# IMPORTANT: Always quote bcrypt hashes to prevent shell expansion!
docker run -e PULSE_AUTH_USER=admin -e PULSE_AUTH_PASS='$2a$12$...' rcourtman/pulse:latest
# Or use Quick Security Setup and restart container
```

**Important**: Always use hashed passwords in configuration. Use the Quick Security Setup or generate bcrypt hashes manually.

#### Features
- Web UI login required when authentication enabled
- Change/remove password from Settings → Security  
- Passwords ALWAYS hashed with bcrypt (cost 12)
- Session-based authentication with secure HttpOnly cookies
- 24-hour session expiry
- CSRF protection for all state-changing operations
- Session invalidation on password change

### API Token Authentication  
For programmatic access and automation. API tokens are SHA3-256 hashed for security.

#### Token Setup via Quick Security
The Quick Security Setup automatically:
- Generates a cryptographically secure token
- Hashes it with SHA3-256
- Stores only the 64-character hash

#### Manual Token Setup
```bash
# Using systemd (use SHA3-256 hash, not plain text!)
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="API_TOKEN=<64-char-sha3-256-hash>"

# Docker
docker run -e API_TOKEN=<64-char-sha3-256-hash> rcourtman/pulse:latest
```

**Security Note**: API tokens are automatically hashed with SHA3-256. Never store plain text tokens in configuration.

#### Token Management (Settings → Security → API Token)
- Generate new tokens via web UI when authenticated
- View existing token anytime (authenticated users only)
- Regenerate tokens without disrupting service
- Delete tokens to disable API access
- All tokens stored as SHA3-256 hashes

#### Usage
```bash
# Include the ORIGINAL token (not hash) in X-API-Token header
curl -H "X-API-Token: your-original-token" http://localhost:7655/api/health

# Or in query parameter for export/import
curl "http://localhost:7655/api/export?token=your-original-token"
```

### Auto-Registration Security

#### Default Mode
- All access requires authentication
- Nodes can auto-register with the API token
- Setup scripts work without additional configuration

#### Secure Mode
- Require API token for all operations
- Protects auto-registration endpoint
- Enable by setting API_TOKEN environment variable

## CORS (Cross-Origin Resource Sharing)

By default, Pulse only allows same-origin requests (no CORS headers). This is the most secure configuration.

### Configuring CORS for External Access

If you need to access Pulse API from a different domain:

```bash
# Docker
docker run -e ALLOWED_ORIGINS="https://app.example.com" rcourtman/pulse:latest

# systemd
sudo systemctl edit pulse-backend
[Service]
Environment="ALLOWED_ORIGINS=https://app.example.com"

# Multiple origins (comma-separated)
ALLOWED_ORIGINS="https://app.example.com,https://dashboard.example.com"

# Development mode (allows localhost)
PULSE_DEV=true
```

**Security Note**: Never use `ALLOWED_ORIGINS=*` in production as it allows any website to access your API.

## Security Best Practices

### Credential Storage
- ✅ **DO**: Use Quick Security Setup for automatic hashing
- ✅ **DO**: Store only bcrypt hashes for passwords
- ✅ **DO**: Store only SHA3-256 hashes for API tokens
- ❌ **DON'T**: Store plain text passwords in config files
- ❌ **DON'T**: Store plain text API tokens in config files
- ❌ **DON'T**: Log credentials or include them in backups

### Authentication Setup
- ✅ **DO**: Use strong, unique passwords (16+ characters)
- ✅ **DO**: Rotate API tokens periodically
- ✅ **DO**: Use HTTPS in production environments
- ❌ **DON'T**: Share API tokens between users/services
- ❌ **DON'T**: Embed credentials in client-side code

### Verification
Run the security verification script to ensure no plain text credentials:
```bash
/opt/pulse/testing-tools/security-verification.sh
```

This checks:
- No hardcoded credentials in code
- No credentials exposed in logs
- All passwords/tokens properly hashed
- Secure file permissions
- No credential leaks in API responses

## Account Lockout and Recovery

### Lockout Behavior
- After **5 failed login attempts**, the account is locked for **15 minutes**
- Lockout applies to both username and IP address
- Login form shows remaining attempts after each failure
- Clear message when locked with time remaining

### Automatic Recovery
- Lockouts automatically expire after 15 minutes
- No action needed - just wait for the timer to expire
- Successful login clears all failed attempt counters

### Manual Recovery (Admin)
Administrators with API access can manually reset lockouts:

```bash
# Reset lockout for a specific username
curl -X POST http://localhost:7655/api/security/reset-lockout \
  -H "X-API-Token: your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"identifier":"username"}'

# Reset lockout for an IP address
curl -X POST http://localhost:7655/api/security/reset-lockout \
  -H "X-API-Token: your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"identifier":"192.168.1.100"}'
```

## Troubleshooting

**Account locked?** Wait 15 minutes or contact admin for manual reset
**Export blocked?** You're on a public network - login with password, set API_TOKEN, or set ALLOW_UNPROTECTED_EXPORT=true
**Rate limited?** Wait 1 minute and try again
**Can't login?** Check PULSE_AUTH_USER and PULSE_AUTH_PASS environment variables
**API access denied?** Verify API_TOKEN is correct (use original token, not hash)
**CORS errors?** Configure ALLOWED_ORIGINS for your domain
**Forgot password?** Start fresh - delete your Pulse data and restart (takes 2 minutes)