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
API_TOKEN=abc123...                  # API authentication token

# Security settings
ENABLE_AUDIT_LOG=true                # Enable security audit logging
```

**Important Notes:**
- Password hash MUST be in single quotes to prevent shell expansion
- This file should have restricted permissions (600)
- Never commit this file to version control
- ProxmoxVE installations may pre-configure API_TOKEN

---

## 📁 `system.json` - Application Settings

**Purpose:** Contains all application behavior settings and configuration.

**Format:** JSON

**Contents:**
```json
{
  "pollingInterval": 5,           // Seconds between node polls (2-60)
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

## Environment Variable Priority

For backwards compatibility, some settings can be overridden via environment variables:

1. **Authentication variables (from .env)** - Always highest priority
   - `PULSE_AUTH_USER`, `PULSE_AUTH_PASS`, `API_TOKEN`

2. **System settings (from system.json)** - Normal priority
   - If system.json exists, it takes precedence
   - If missing, environment variables are checked

3. **Legacy environment variables** - Lowest priority (deprecated)
   - `POLLING_INTERVAL` - Only used if system.json doesn't exist
   - `CONNECTION_TIMEOUT` - Can override system.json value
   - `ALLOWED_ORIGINS` - Can override system.json value

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
