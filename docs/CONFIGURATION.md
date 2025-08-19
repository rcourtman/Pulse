# Pulse Configuration Guide

## Configuration File Structure

Pulse uses three separate configuration files, each with a specific purpose. This separation ensures security, clarity, and proper access control.

### File Locations
All configuration files are stored in `/etc/pulse/` (or `/data/` in Docker containers).

```
/etc/pulse/
├── .env          # Authentication credentials
├── system.json   # Application settings
└── nodes.enc     # Encrypted node credentials
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
ENABLE_AUDIT_LOG=true                # Enable security audit logging
```

**Important Notes:**
- Password hash MUST be in single quotes to prevent shell expansion
- API tokens are stored in plain text (48-64 hex characters)
- This file should have restricted permissions (600)
- Never commit this file to version control
- ProxmoxVE installations may pre-configure API_TOKEN
- Changes to this file are applied immediately without restart (v4.3.9+)

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
  "frontendPort": 7655            // Frontend UI port (same as backend in embedded mode)
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

- `DISCOVERY_SUBNET` - Network to scan (e.g., "192.168.1.0/24")
- `CONNECTION_TIMEOUT` - API timeout in seconds (default: 10)
- `ALLOWED_ORIGINS` - CORS origins (default: same-origin only)
- `LOG_LEVEL` - Log verbosity: debug/info/warn/error (default: info)

#### Network & Security Variables (always from env)
These are only configurable via environment variables for security:

- `PULSE_AUTH_USER`, `PULSE_AUTH_PASS` - Basic authentication
- `API_TOKEN` - API token for authentication
- `FRONTEND_PORT` - Port to listen on (default: 7655)
- `HTTPS_ENABLED` - Enable HTTPS (true/false)
- `TLS_CERT_FILE`, `TLS_KEY_FILE` - Paths to TLS certificate files

> **⚠️ UI Override Warning**: When configuration env vars are set (like `ALLOWED_ORIGINS`), the corresponding UI fields will be disabled with a warning message. Remove the env var and restart to enable UI configuration.

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
