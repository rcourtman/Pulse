# 🚚 Migrating Pulse

This guide covers migrating Pulse to a new host using the built-in encrypted export/import workflow.

## 🚀 Quick Migration Guide

### ❌ DON'T: Copy Files
Never copy `/etc/pulse` (or `/data` in Docker/Kubernetes) manually. Encryption keys and credentials can break.

### ✅ DO: Use Export/Import

#### 1. Export (Old Server)
1.  Go to **Settings → System → Backups**.
2.  Click **Create Backup**.
3.  Enter a strong passphrase and download the encrypted backup.

#### 2. Import (New Server)
1.  Install a fresh Pulse instance.
2.  Go to **Settings → System → Backups**.
3.  Click **Restore Configuration** and upload your file.
4.  Enter the passphrase.

## 📦 What Gets Migrated

| Included ✅ | Not Included ❌ |
| :--- | :--- |
| Nodes & credentials | Historical metrics history (`metrics.db`) |
| Alerts & overrides | Browser sessions and local cookies |
| Notifications (email, webhooks, Apprise) | Local login username/password (`.env`) |
| System settings (`system.json`) | Update history/backup folders |
| API token records | |
| OIDC config | |
| Guest metadata/notes | |

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

Because local login credentials are stored in `.env` (not part of exports), you must:

1.  **Re-create Admin User**: If not using `.env` overrides, create your admin account on the new instance.
2.  **Confirm API access**:
    *   If you created API tokens in the UI, those token records are included in the export and should continue working.
    *   If you used `.env`-based `API_TOKENS`/`API_TOKEN`, reconfigure them on the new host.
3.  **Update Agents**:
    *   **Unified Agent**: Update the `--token` flag in your service definition.
    *   **Docker**: Update `PULSE_TOKEN` in your container config.
    *   *Tip: You can use the "Install New Agent" wizard to generate updated install commands.*

## 🔒 Security

*   **Encryption**: Exports are encrypted with passphrase-based encryption (PBKDF2 + AES-GCM).
*   **Storage**: Safe to store in cloud backups or password managers.
*   **Passphrase**: Use a strong, unique passphrase (min 12 chars).

## 🔧 Troubleshooting

*   **"Invalid passphrase"**: Ensure exact match (case-sensitive).
*   **Missing Nodes**: Verify export date.
*   **Connection Errors**: Update node IPs in Settings if they changed.
*   **Logging**: Adjust `LOG_LEVEL`/`LOG_FORMAT` via environment variables if needed.
