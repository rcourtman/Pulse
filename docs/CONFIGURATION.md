# Pulse Configuration Guide

## Key Features

- **🔒 Auto-Hashing Security** (v4.5.0+): Plain text credentials in environment variables are automatically hashed
- **📁 Separated Configuration**: Authentication (.env), settings (system.json), and node credentials (nodes.enc) 
- **🚀 Zero-Touch Deployment**: Configure via environment variables, skip UI setup entirely
- **🔐 Enterprise Security**: Credentials encrypted at rest, hashed in memory

## Configuration File Structure

Pulse uses three separate configuration files, each with a specific purpose. This separation ensures security, clarity, and proper access control.

### File Locations
All configuration files are stored in `/etc/pulse/` (or `/data/` in Docker containers).

```
/etc/pulse/
├── .env          # Authentication credentials ONLY
├── system.json   # Application settings (ports, intervals, etc.)
├── nodes.enc     # Encrypted node credentials
├── alerts.json   # Alert thresholds and rules
└── webhooks.json # Webhook configurations
```

---

## 📁 `.env` - Authentication & Security

**Purpose:** Contains authentication credentials and security settings ONLY.

**Format:** Environment variables (KEY=VALUE)

**Contents:**
```bash
# User authentication
PULSE_AUTH_USER='admin'              # Admin username
PULSE_AUTH_PASS='$2a$12$...'        # Bcrypt hashed password (keep quotes!)
API_TOKEN=abc123...                  # API token (plain text, not hashed)

# Security settings
DISABLE_AUTH=true                    # Disable authentication entirely (for proxy auth)
ENABLE_AUDIT_LOG=true                # Enable security audit logging
```

**Important Notes:**
- Password hash MUST be in single quotes to prevent shell expansion
- API tokens are stored in plain text (48-64 hex characters)
- This file should have restricted permissions (600)
- Never commit this file to version control
- ProxmoxVE installations may pre-configure API_TOKEN
- Changes to this file are applied immediately without restart (v4.3.9+)
- **DO NOT** put port configuration here - use system.json or systemd overrides

---

## 📁 `system.json` - Application Settings

**Purpose:** Contains all application behavior settings and configuration.

**Format:** JSON

**Contents:**
```json
{
  "pollingInterval": 10,          // Fixed at 10 seconds to match Proxmox update cycle
  "connectionTimeout": 10,        // Seconds before node connection timeout
  "autoUpdateEnabled": false,     // Enable automatic updates
  "updateChannel": "stable",      // Update channel: stable, rc, beta
  "autoUpdateTime": "03:00",      // Time for automatic updates (24hr format)
  "allowedOrigins": "",           // CORS allowed origins (empty = same-origin only)
  "backendPort": 7655,            // Backend API port
  "frontendPort": 7655,           // Frontend UI port (same as backend in embedded mode)
  "discoveryEnabled": true,       // Enable/disable network discovery for Proxmox/PBS servers
  "discoverySubnet": "auto"       // Subnet to scan ("auto" or CIDR like "192.168.1.0/24")
}
```

**Important Notes:**
- User-editable via Settings UI
- Can be safely backed up without exposing secrets
- Missing file results in defaults being used
- Changes take effect immediately (no restart required)
- API tokens are no longer managed in system.json (moved to .env in v4.3.9+)

---

## 📁 `nodes.enc` - Encrypted Node Credentials

**Purpose:** Stores encrypted credentials for Proxmox VE and PBS nodes.

**Format:** Encrypted JSON (AES-256-GCM)

**Structure (when decrypted):**
```json
{
  "pveInstances": [
    {
      "name": "pve-node1",
      "url": "https://192.168.1.10:8006",
      "username": "root@pam",
      "password": "encrypted_password_here",
      "token": "optional_api_token"
    }
  ],
  "pbsInstances": [
    {
      "name": "backup-server",
      "url": "https://192.168.1.20:8007",
      "username": "admin@pbs",
      "password": "encrypted_password_here"
    }
  ]
}
```

**Important Notes:**
- Encrypted at rest using system-generated key
- Credentials never exposed in UI (only "•••••" shown)
- Export/import requires authentication
- Automatic re-encryption on each save

---

## Configuration Priority

Settings are loaded in this order (later overrides earlier):

1. **Built-in defaults** - Hardcoded application defaults
2. **system.json file** - Settings configured via UI
3. **Environment variables** - Override both defaults and system.json

### Environment Variables

#### Configuration Variables (override system.json)
These env vars override system.json values. When set, the UI will show a warning and disable the affected fields:

- `DISCOVERY_ENABLED` - Enable/disable network discovery (default: true)
- `DISCOVERY_SUBNET` - Custom network to scan (default: auto-scans common networks)
- `CONNECTION_TIMEOUT` - API timeout in seconds (default: 10)
- `ALLOWED_ORIGINS` - CORS origins (default: same-origin only)
- `LOG_LEVEL` - Log verbosity: debug/info/warn/error (default: info)

#### Authentication Variables (from .env file)
These should be set in the .env file for security:

- `PULSE_AUTH_USER`, `PULSE_AUTH_PASS` - Basic authentication
- `API_TOKEN` - API token for authentication
- `DISABLE_AUTH` - Set to `true` to disable authentication entirely (useful for reverse proxy auth)

#### Port Configuration
Port configuration should be done via one of these methods:

1. **systemd override** (Recommended for production):
   ```bash
   sudo systemctl edit pulse
   # Add: Environment="FRONTEND_PORT=8080"
   ```

2. **system.json** (For persistent configuration):
   ```json
   {"frontendPort": 8080}
   ```

3. **Environment variable** (For Docker/testing):
   - `FRONTEND_PORT` - Port to listen on (default: 7655)
   - `PORT` - Legacy port variable (use FRONTEND_PORT instead)

#### TLS/HTTPS Configuration
- `HTTPS_ENABLED` - Enable HTTPS (true/false)
- `TLS_CERT_FILE`, `TLS_KEY_FILE` - Paths to TLS certificate files

> **⚠️ UI Override Warning**: When configuration env vars are set (like `ALLOWED_ORIGINS`), the corresponding UI fields will be disabled with a warning message. Remove the env var and restart to enable UI configuration.

---

## Automated Setup (Skip UI)

For automated deployments (CI/CD, infrastructure as code, ProxmoxVE scripts), you can configure Pulse authentication via environment variables, completely bypassing the UI setup screen.

### Simple Automated Setup

**Option 1: API Token Authentication**
```bash
# Start Pulse with API token - setup screen is skipped
API_TOKEN=your-secure-api-token ./pulse

# The token is hashed and stored securely
# Use this same token for all API calls
curl -H "X-API-Token: your-secure-api-token" http://localhost:7655/api/nodes
```

**Option 2: Basic Authentication**
```bash
# Start Pulse with username/password - setup screen is skipped
PULSE_AUTH_USER=admin \
PULSE_AUTH_PASS=your-secure-password \
./pulse

# Password is bcrypt hashed and stored securely
# Use these credentials for UI login or API calls
```

**Option 3: Both (API + Basic Auth)**
```bash
# Configure both authentication methods
API_TOKEN=your-api-token \
PULSE_AUTH_USER=admin \
PULSE_AUTH_PASS=your-password \
./pulse
```

### Security Notes

- **Automatic hashing**: Plain text credentials are automatically hashed when provided via environment variables
  - API tokens → SHA3-256 hash
  - Passwords → bcrypt hash (cost 12)
- **Pre-hashed credentials supported**: Advanced users can provide pre-hashed values:
  - API tokens: 64-character hex string (SHA3-256 hash)
  - Passwords: bcrypt hash starting with `$2a$`, `$2b$`, or `$2y$` (60 characters)
- **No plain text in memory**: All credentials are hashed before use
- Once configured, the setup screen is automatically skipped
- Credentials work immediately - no additional setup required

### Example: Docker Automated Deployment

```bash
#!/bin/bash
# Generate secure token
API_TOKEN=$(openssl rand -hex 32)

# Deploy with authentication pre-configured
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -e API_TOKEN="$API_TOKEN" \
  -v pulse-data:/data \
  rcourtman/pulse:latest

echo "Pulse deployed! Use API token: $API_TOKEN"

# Immediately use the API - no setup needed
curl -H "X-API-Token: $API_TOKEN" http://localhost:7655/api/nodes
```

### ProxmoxVE Helper Script

The ProxmoxVE community scripts already use this approach:

```bash
# They generate a token and set it directly
API_TOKEN=generated-token-here /opt/pulse/bin/pulse
```

---

## Security Best Practices

1. **File Permissions**
   ```bash
   chmod 600 /etc/pulse/.env        # Only readable by owner
   chmod 644 /etc/pulse/system.json # Readable by all, writable by owner
   chmod 600 /etc/pulse/nodes.enc   # Only readable by owner
   ```

2. **Backup Strategy**
   - `.env` - Backup separately and securely (contains auth)
   - `system.json` - Safe to include in regular backups
   - `nodes.enc` - Backup with .env (contains encrypted credentials)

3. **Version Control**
   - **NEVER** commit `.env` or `nodes.enc`
   - `system.json` can be committed if it doesn't contain sensitive data
   - Use `.gitignore` to exclude sensitive files
