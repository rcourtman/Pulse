# Pulse Security Guide

## Overview

Pulse v4 implements a comprehensive security system with multiple layers of protection for your monitoring data and configuration.

## Security Features

### 1. API Token Authentication
- Optional but recommended for production environments
- When set, all configuration endpoints require authentication
- Protects sensitive operations like export/import

### 2. Configuration Export/Import Protection
- **Secure by Default**: Export/import is blocked unless API token is configured
- **Homelab Mode**: Can be explicitly enabled with `ALLOW_UNPROTECTED_EXPORT=true`
- **Encryption**: All exported data is encrypted with AES-256-GCM using your passphrase
- **Minimum Security**: Passphrases must be at least 12 characters

### 3. Rate Limiting
- Prevents brute force attacks on sensitive endpoints
- Configured at 5 attempts per minute for export/import operations
- Automatically tracks and blocks abusive IPs

### 4. Audit Logging
- All export/import attempts are logged with IP addresses
- Tracks both successful and failed authentication attempts
- Helps identify potential security threats

## Configuration

### Setting Up API Protection

#### Method 1: Environment Variable (Systemd)
```bash
# Edit the systemd service
sudo systemctl edit pulse-backend

# Add:
[Service]
Environment="API_TOKEN=your-secure-token-here"

# Restart
sudo systemctl restart pulse-backend
```

#### Method 2: Docker
```bash
docker run -d \
  -e API_TOKEN=your-secure-token \
  -p 7655:7655 \
  rcourtman/pulse:latest
```

### Homelab Configuration

For trusted homelab environments where you want to allow export/import without API authentication:

#### Systemd
```bash
sudo systemctl edit pulse-backend

# Add:
[Service]
Environment="ALLOW_UNPROTECTED_EXPORT=true"

sudo systemctl restart pulse-backend
```

#### Docker
```bash
docker run -d \
  -e ALLOW_UNPROTECTED_EXPORT=true \
  -p 7655:7655 \
  rcourtman/pulse:latest
```

## Security Status

The frontend Settings page includes a Security tab that shows:
- Whether API token is configured
- Export/import protection status
- Configuration guidance

Access it at: Settings → Security

## Export/Import Security

### How It Works
1. **Without API Token + Without ALLOW_UNPROTECTED_EXPORT**: Export/import is completely blocked
2. **With API Token**: Export/import requires authentication
3. **With ALLOW_UNPROTECTED_EXPORT=true**: Export/import allowed but data still encrypted

### Encryption Details
- Algorithm: AES-256-GCM
- Key Derivation: PBKDF2 with SHA-256
- Iterations: 100,000
- Salt: Randomly generated per export
- Minimum passphrase length: 12 characters

### What's Encrypted
- Node credentials (passwords, API tokens)
- PBS credentials
- All sensitive configuration data

### What's NOT Encrypted
- Node hostnames and IPs
- Threshold settings
- Non-sensitive configuration

## Best Practices

### Production Environments
1. **Always set an API token** - This is your first line of defense
2. **Use strong passphrases** - At least 16 characters for export/import
3. **Monitor logs** - Check for unauthorized access attempts
4. **Restrict network access** - Use firewalls to limit who can reach Pulse
5. **Use HTTPS proxy** - Put Pulse behind nginx/traefik with SSL

### Homelab Environments
1. **Consider your network** - If exposed to internet, use API token
2. **Strong passphrases still matter** - Exports contain encrypted credentials
3. **Regular backups** - Export configurations periodically
4. **Update regularly** - Keep Pulse updated for security patches

## Troubleshooting

### Export/Import Blocked
**Error**: "Export requires API_TOKEN to be set"
**Solution**: Either set an API token or enable `ALLOW_UNPROTECTED_EXPORT=true` for homelab use

### Rate Limit Exceeded
**Error**: "Rate limit exceeded. Please try again later"
**Solution**: Wait 1 minute before retrying. Check for scripts making too many requests.

### Invalid Passphrase Length
**Error**: "Passphrase must be at least 12 characters long"
**Solution**: Use a longer, more secure passphrase

