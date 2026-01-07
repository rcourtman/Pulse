# Proxmox Backup Server (PBS) Integration

This guide explains how to connect Pulse to your Proxmox Backup Server for comprehensive backup monitoring.

## Two Ways to Monitor PBS Backups

Pulse can monitor PBS backups in two ways:

### 1. Direct PBS Connection (Recommended)

Connect directly to your PBS server for full monitoring capabilities:

**Benefits:**
- ✅ Deduplication factor and storage efficiency stats
- ✅ PBS server health monitoring (CPU, memory, uptime)
- ✅ Datastore usage and namespace hierarchy
- ✅ Sync, verify, prune, and GC job status
- ✅ Backup owner information
- ✅ Faster queries (no PVE proxy overhead)

### 2. PVE Passthrough (Automatic)

If your PVE cluster has PBS storage configured, Pulse automatically fetches backup data through the PVE API.

**Limitations:**
- ❌ No deduplication stats
- ❌ No PBS server health data
- ❌ No job monitoring
- ❌ Can be slow for encrypted PBS storage
- ❌ Limited metadata per backup

**Recommendation:** If you see a banner in the Backups page suggesting you add PBS directly, following this guide will significantly improve your monitoring experience.

---

## Setting Up Direct PBS Connection

### Method 1: Unified Agent Install (Recommended for Bare Metal)

Install the unified agent directly on your PBS server for automatic setup:

```bash
# Run on your PBS server
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  sudo bash -s -- --url http://<pulse-ip>:7655 --token <api-token> --enable-proxmox --proxmox-type pbs
```

The agent will:
1. Detect it's running on a PBS server
2. Create a `pulse-monitor@pbs` user with read-only access
3. Generate an API token
4. Register the PBS node with Pulse automatically

### Method 2: Password Setup (Best for Docker PBS) ⭐

**Perfect for PBS running in Docker containers** where you can't run the agent.

1. Go to **Settings → Nodes → Add PBS Node**
2. Enter your PBS server's URL (e.g., `https://192.168.1.50:8007`)
3. Select **Username & Password** authentication
4. Enter admin credentials (e.g., `root@pam` with password)
5. Click **Save**

Pulse will automatically:
- Connect to your PBS server
- Create a `pulse-monitor@pbs` monitoring user
- Generate an API token with Audit permissions
- Store the token (not your password) for ongoing monitoring

> **Note:** This requires admin credentials initially, but Pulse converts them to a limited-permission token immediately.

### Method 3: One-Click Setup Script

From Pulse's Settings page:
1. Go to **Settings → Nodes**
2. Click **Add PBS Node**
3. Select **API Only** tab
4. Enter your PBS server's URL
5. Click copy to get the setup command
6. Run the command on your PBS server

### Method 4: Manual Token Creation

If you prefer manual setup:

```bash
# SSH into your PBS server

# 1. Create a dedicated monitoring user
proxmox-backup-manager user create pulse-monitor@pbs --comment "Pulse monitoring"

# 2. Grant read-only access (Audit role)
proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs

# 3. Generate an API token (save the output!)
proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token
```

Copy the token value and enter it in Pulse:
- **Token ID:** `pulse-monitor@pbs!pulse-token`
- **Token Value:** The UUID shown after running the command

---

## PBS Permissions

The Pulse monitoring user needs minimal permissions:

| Role | Path | Purpose |
|------|------|---------|
| `Audit` | `/` | Read-only access to all datastores, backups, and server status |

The `Audit` role provides:
- List datastores and their usage
- View backup groups and snapshots
- Read server status (CPU, memory, uptime)
- View job history and status

It does **not** allow:
- Creating, modifying, or deleting backups
- Running backup/restore operations
- Changing server configuration

---

## Multiple PBS Servers

If you have multiple PBS servers, add each one separately in Settings. Pulse will:
- Monitor each server independently
- Show backups from all servers in the unified Backups view
- Deduplicate if the same backup appears via both PVE passthrough and direct PBS

---

## Troubleshooting

### "Connection Failed" Error

1. **Check URL:** Ensure the PBS URL is correct (default port is 8007)
   - Format: `https://pbs.example.com:8007`

2. **Verify token:** Test authentication:
   ```bash
   curl -sk -H "Authorization: PBSAPIToken=pulse-monitor@pbs!pulse-token:YOUR_TOKEN" \
     https://your-pbs:8007/api2/json/version
   ```

3. **Network access:** Ensure Pulse can reach PBS on port 8007

4. **SSL verification:** If using self-signed certificates, disable SSL verification in the node settings

### Slow Backup Loading

If you notice slow loading for PBS storage accessed via PVE:
- This often happens with encrypted PBS datastores
- The fix is to add PBS directly (this guide)
- Direct PBS connections bypass the slow PVE content listing

### Duplicate Backups

If you see the same backup twice:
- This shouldn't happen—Pulse deduplicates by VMID and timestamp
- If it does occur, the direct PBS version takes priority
- Check console for debug logs: `localStorage.setItem('debug-pmg', 'true')`

---

## Data Source Indicator

In the Backups view, PBS backups show a data source indicator:

- **"PBS"** badge alone = Direct PBS connection (full data)
- **"PBS via PVE"** = Passthrough via PVE storage (limited data)

Adding your PBS server directly will remove the "via PVE" indicator and unlock full monitoring capabilities.

---

## Related Documentation

- [Unified Agent Setup](UNIFIED_AGENT.md) - Installing agents on PBS/PVE/PMG hosts
- [Configuration Reference](CONFIGURATION.md) - Environment variables including PBS settings
- [Troubleshooting](TROUBLESHOOTING.md) - General troubleshooting guide
