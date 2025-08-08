# Pulse Security

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

## Troubleshooting

**Export blocked?** Set API_TOKEN or ALLOW_UNPROTECTED_EXPORT=true
**Rate limited?** Wait 1 minute and try again