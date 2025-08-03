# Pulse Security Guide

## Overview

Pulse is designed as an internal monitoring tool for Proxmox environments with flexible security options to match your deployment needs, from simple homelabs to enterprise deployments.

## Security Levels

### Level 0: Quick Start (Default)
- Credentials stored inline in `/etc/pulse/pulse.yml`
- Works immediately, no extra setup required
- **Pulse will warn you** if the file has overly permissive permissions

### Level 1: Basic Security (Recommended)
Simply restrict file permissions:
```bash
sudo chmod 600 /etc/pulse/pulse.yml
sudo chown pulse:pulse /etc/pulse/pulse.yml
```
This ensures only the pulse user can read the configuration file.

### Level 2: Environment Variables
Replace sensitive values with environment variable references:

```yaml
nodes:
  pve:
    - name: homelab
      host: https://proxmox.example.com:8006
      user: pulse-monitor@pam
      token_name: noprivsep
      token_value: ${PROXMOX_TOKEN}  # Reference env variable
```

Then set the environment variable:
```bash
# For systemd service
sudo systemctl edit pulse-backend
# Add:
[Service]
Environment="PROXMOX_TOKEN=YOUR-TOKEN-HERE-XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"

# For Docker
docker run -e PROXMOX_TOKEN=your-token-here pulse
```

### Level 3: File References
Store each credential in a separate file:

```yaml
nodes:
  pve:
    - name: homelab
      token_value: file:///etc/pulse/secrets/proxmox.token
```

Setup:
```bash
# Create secrets directory
sudo mkdir -p /etc/pulse/secrets
sudo chmod 700 /etc/pulse/secrets

# Create token file
echo -n "YOUR-TOKEN-HERE-XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX" | sudo tee /etc/pulse/secrets/proxmox.token
sudo chmod 600 /etc/pulse/secrets/proxmox.token
sudo chown pulse:pulse /etc/pulse/secrets/proxmox.token
```

## Security Warnings

Pulse will automatically warn you about:
- Config files with overly permissive permissions (readable by others)
- Credentials stored inline when the file is world-readable
- Secret files with incorrect permissions

Example warnings:
```
WRN Config file has overly permissive permissions. Recommended: chmod 600 /etc/pulse/pulse.yml
WRN The following credentials are stored inline in a world-readable file: ["pve.homelab.token_value"]
INF 💡 Security tip: You can reference credentials more securely:
INF   - Environment variable: token_value: ${PROXMOX_TOKEN}
INF   - File reference: token_value: file:///etc/pulse/secrets/proxmox.token
INF   - Or simply: chmod 600 /etc/pulse/pulse.yml
```

## Best Practices

1. **For Homelab Users**: Level 1 (restricted file permissions) provides good security with zero complexity
2. **For Docker/K8s**: Use environment variables (Level 2) for easy secret management
3. **For Production**: Use file references (Level 3) with proper secret management tools

## Examples

### Mixed Approach
You can mix different methods based on your needs:

```yaml
nodes:
  pve:
    - name: production
      token_value: ${PROD_TOKEN}  # High security for production
    - name: homelab
      token_value: file:///etc/pulse/secrets/homelab.token  # Moderate security
    - name: test
      token_value: test-token-12345  # Low security for test environment
```

### Docker Compose Example
```yaml
version: '3.8'
services:
  pulse:
    image: pulse:latest
    environment:
      - PROXMOX_TOKEN=${PROXMOX_TOKEN}
      - PROXMOX2_TOKEN=${PROXMOX2_TOKEN}
    volumes:
      - ./pulse.yml:/etc/pulse/pulse.yml:ro
```

### Kubernetes Secret Example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pulse-tokens
stringData:
  proxmox-token: "YOUR-TOKEN-HERE-XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: pulse
        env:
        - name: PROXMOX_TOKEN
          valueFrom:
            secretKeyRef:
              name: pulse-tokens
              key: proxmox-token
```

## Migration

To migrate existing inline credentials:

1. **Quick & Secure**: Just chmod 600 your config file
2. **Environment Variables**: Replace values with ${VAR_NAME} and set the variables
3. **File References**: Move tokens to separate files and update config

The system remains backward compatible - existing configurations continue to work with security warnings to guide improvements.