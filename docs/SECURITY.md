# Pulse Security

## Security Warning System (v4.3.2+)

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

- **Encryption**: Exports are always encrypted (AES-256-GCM)
- **Rate Limiting**: 5 attempts per minute on export/import
- **Minimum Passphrase**: 12 characters required
- **Security Tab**: Check status in Settings → Security

### What's Encrypted in Exports
- Node credentials (passwords, API tokens)
- PBS credentials
- Email settings passwords

### What's NOT Encrypted
- Node hostnames and IPs
- Threshold settings
- General configuration

## Registration Tokens

Secure your Pulse instance by requiring tokens for node auto-registration.

### Token Management (v4.0+)
Access via **Settings → Security → Registration Tokens**

#### Features
- Generate time-limited registration tokens
- Set maximum usage count per token
- Restrict tokens to specific node types (PVE/PBS)
- Add descriptions for token identification
- Revoke tokens immediately when needed

#### Configuration Options
```bash
# Require tokens for all registrations (recommended for production)
Environment="REQUIRE_REGISTRATION_TOKEN=true"

# Allow registration without tokens (default - homelab friendly)
Environment="ALLOW_UNPROTECTED_AUTO_REGISTER=true"

# Default token validity (seconds)
Environment="REGISTRATION_TOKEN_DEFAULT_VALIDITY=1800"

# Default max uses per token
Environment="REGISTRATION_TOKEN_DEFAULT_MAX_USES=1"
```

#### Usage Flow
1. **Admin**: Generate token in Settings → Security → Registration Tokens
2. **Admin**: Copy token (format: `PULSE-REG-xxxxxxxxxxxx`)
3. **Node Setup**: Include token in setup script or auto-register request
4. **System**: Validates token and decrements usage count
5. **System**: Auto-expires token after validity period

#### Setup Script Integration
```bash
# Include token when running setup script
PULSE_REG_TOKEN=PULSE-REG-xxxxxxxxxxxx ./setup.sh

# Or in auto-register API call
curl -X POST "https://pulse-server:7655/api/auto-register" \
  -H "X-Registration-Token: PULSE-REG-xxxxxxxxxxxx" \
  -d "$NODE_DATA"
```

### Security Modes

#### Homelab Mode (Default)
- Registration tokens optional
- Nodes can register without authentication
- Suitable for trusted networks
- Enable with: `ALLOW_UNPROTECTED_AUTO_REGISTER=true`

#### Production Mode
- All registrations require valid token
- Tokens expire after set time
- Usage limits enforced
- Enable with: `REQUIRE_REGISTRATION_TOKEN=true`

## Troubleshooting

**Export blocked?** Set API_TOKEN or ALLOW_UNPROTECTED_EXPORT=true
**Rate limited?** Wait 1 minute and try again
**Registration failing?** Check if REQUIRE_REGISTRATION_TOKEN is enabled
**Token not working?** Verify it hasn't expired or exceeded usage limit