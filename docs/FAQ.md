# FAQ

## Installation

### What's the easiest way to install?
```bash
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```

### System requirements?
- 1 vCPU, 512MB RAM (1GB recommended), 1GB disk
- Network access to Proxmox API

## Configuration

### How do I add a node?
**Auto-discovery (Easiest)**: Settings → Nodes → Click "Setup Script" on discovered node → Run on Proxmox
**Manual**: Settings → Nodes → Add Node → Enter credentials → Save

### How do I disable network discovery?
Settings → System → Network Settings → Toggle "Enable Discovery" off → Save
Or set environment variable `DISCOVERY_ENABLED=false`

### How do I change the port?
Systemd: `sudo systemctl edit pulse-backend`, add `Environment="FRONTEND_PORT=8080"`, restart
Docker: Use `-e FRONTEND_PORT=8080` in your run command

### Why can't I change settings in the UI?
If a setting is disabled with an amber warning, it's being overridden by an environment variable. 
Remove the env var (check `sudo systemctl show pulse-backend | grep Environment`) and restart to enable UI configuration.

### What permissions needed?
- PVE: `PVEAuditor` minimum
- PBS: `DatastoreReader` minimum

### API tokens vs passwords?
API tokens are more secure. Create in Proxmox: Datacenter → Permissions → API Tokens

### Where are settings stored?
See [Configuration Guide](CONFIGURATION.md) for details

### How do I backup my configuration?
Settings → Security → Backup & Restore → Export Backup
- If logged in with password: Just enter your password or a custom passphrase
- If using API token only: Provide the API token when prompted
- Includes all settings, nodes, credentials (encrypted), and custom console URLs

### Can Pulse detect Proxmox clusters?
Yes! When you add one cluster node, Pulse automatically discovers and monitors all nodes

## Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006/8007)
- Verify credentials
- Check logs: `journalctl -u pulse -f`

### Connection refused?
- Check port 7655 is open
- Verify Pulse is running: `systemctl status pulse`

### PBS connection issues?
- PBS requires HTTPS (not HTTP) - use `https://your-pbs:8007`
- Default PBS port is 8007 (not 8006)
- Check firewall allows port 8007

### Invalid credentials?
- Check username includes realm (@pam, @pve)
- Verify API token not expired
- Confirm user has required permissions

### CORS errors in browser?
- By default, Pulse only allows same-origin requests
- Set `ALLOWED_ORIGINS` environment variable for cross-origin access
- Example: `ALLOWED_ORIGINS=https://app.example.com`
- Never use `*` in production

### Authentication issues?
- Password auth: Check `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` environment variables
- API token: Verify `API_TOKEN` is set correctly
- Session expired: Log in again via web UI
- Account locked: Wait 15 minutes after 5 failed attempts

### High memory usage?
Reduce `metricsRetentionDays` in settings and restart

## Features

### Multiple clusters?
Yes, add multiple nodes in Settings

### PBS push mode?
No, PBS push mode is not currently supported. PBS monitoring requires network connectivity from Pulse to the PBS server.

### Webhook providers?
Discord, Slack, Gotify, Telegram, ntfy.sh, Teams, generic JSON

### Works with reverse proxy?
Yes, ensure WebSocket support is enabled

## Updates

### How to update?
- **ProxmoxVE LXC**: Type `update` in the LXC console
- **Docker**: Pull latest image, recreate container  
- **Manual/systemd**: Run the install script again: `curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | sudo bash`

### Why can't I update from the UI?
For security reasons, Pulse cannot self-update. The UI will notify you when updates are available and show the appropriate update command for your deployment type.

### Will updates break config?
No, configuration is preserved