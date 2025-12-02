# 🚚 Migrating Pulse

**Updated for Pulse v4.24.0+**

## 🚀 Quick Migration Guide

### ❌ DON'T: Copy Files
Never copy `/etc/pulse` or `/var/lib/pulse` manually. Encryption keys and credentials will break.

### ✅ DO: Use Export/Import

#### 1. Export (Old Server)
1.  Go to **Settings → Configuration Management**.
2.  Click **Export Configuration**.
3.  Enter a strong passphrase and save the `.enc` file.

#### 2. Import (New Server)
1.  Install a fresh Pulse instance.
2.  Go to **Settings → Configuration Management**.
3.  Click **Import Configuration** and upload your file.
4.  Enter the passphrase.

## 📦 What Gets Migrated

| Included ✅ | Not Included ❌ |
| :--- | :--- |
| Nodes & Credentials | Historical Metrics |
| Alert Settings | Alert History |
| Email & Webhooks | Auth Settings (Passwords/Tokens) |
| System Settings | Update Rollback History |
| Guest Metadata | |

## 🔄 Common Scenarios

### Moving to New Hardware
Export from old → Install new → Import.

### Docker ↔ Systemd ↔ Kubernetes
The export file works across all installation methods. You can migrate from Docker to Kubernetes or vice versa seamlessly.

### Disaster Recovery
1.  Install Pulse using Docker or your preferred method (see [INSTALL.md](INSTALL.md)).
2.  Import your latest backup.
3.  Restored in < 5 minutes.

## 📋 Post-Migration Checklist

Because authentication secrets are excluded from exports, you must:

1.  **Re-create Admin User**: If not using `.env` overrides, create your admin account on the new instance.
2.  **Re-issue API Tokens**:
    *   Go to **Settings → Security**.
    *   Generate new tokens for your agents and scripts.
3.  **Update Agents**:
    *   **Unified Agent**: Update the `--token` flag in your service definition.
    *   **Docker**: Update `PULSE_TOKEN` in your container config.
    *   *Tip: You can use the "Install New Agent" wizard to generate updated install commands.*

## 🔒 Security

*   **Encryption**: Exports are encrypted with PBKDF2 (100k iterations).
*   **Storage**: Safe to store in cloud backups or password managers.
*   **Passphrase**: Use a strong, unique passphrase (min 12 chars).

## 🔧 Troubleshooting

*   **"Invalid passphrase"**: Ensure exact match (case-sensitive).
*   **Missing Nodes**: Verify export date.
*   **Connection Errors**: Update node IPs in Settings if they changed.
*   **Logging**: Re-configure log levels in **Settings → System → Logging** if needed.