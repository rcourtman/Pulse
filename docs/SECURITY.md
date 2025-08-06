# Pulse Security Guide

## Authentication Model

Pulse v4 uses an optional API token authentication system:

### Without API Token (Default)
- **Status**: UNPROTECTED
- All API endpoints are accessible without authentication
- Configuration export/import is allowed but data is encrypted with your passphrase
- Suitable for: Trusted local networks, homelab environments

### With API Token (Recommended)
- **Status**: PROTECTED  
- All configuration endpoints require the API token
- Export/import additionally requires the encryption passphrase
- Suitable for: Production environments, exposed instances

## Setting Up API Protection

### Method 1: Environment Variable
```bash
# Edit the systemd service
sudo systemctl edit pulse

# Add:
[Service]
Environment="API_TOKEN=your-secure-token-here"

# Restart
sudo systemctl restart pulse
```

### Method 2: Docker
```bash
docker run -d \
  -e API_TOKEN=your-secure-token \
  -p 7655:7655 \
  rcourtman/pulse:latest
```

## Best Practices

1. **Always set an API token** for any instance accessible outside localhost
2. **Use strong passphrases** for export/import operations
3. **Store exported configs securely** - they contain encrypted credentials
4. **Rotate API tokens periodically**
EOF < /dev/null
