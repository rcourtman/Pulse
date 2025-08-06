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
Settings → Nodes → Add Node → Enter credentials → Save

### What permissions needed?
- PVE: `PVEAuditor` minimum
- PBS: `DatastoreReader` minimum

### API tokens vs passwords?
API tokens are more secure. Create in Proxmox: Datacenter → Permissions → API Tokens

## Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006/8007)
- Verify credentials
- Check logs: `journalctl -u pulse -f`

### Connection refused?
- Check port 7655 is open
- Verify Pulse is running: `systemctl status pulse`

### Invalid credentials?
- Check username includes realm (@pam, @pve)
- Verify API token not expired
- Confirm user has required permissions

### High memory usage?
Reduce `metricsRetentionDays` in settings and restart

## Features

### Multiple clusters?
Yes, add multiple nodes in Settings

### PBS push mode?
Yes, use PBS agent for isolated servers. See [PBS Agent docs](PBS-AGENT.md)

### Webhook providers?
Discord, Slack, Gotify, Telegram, ntfy.sh, Teams, generic JSON

### Works with reverse proxy?
Yes, ensure WebSocket support is enabled

## Updates

### How to update?
- **Docker**: Pull latest image, recreate container
- **Manual**: Settings → System → Check for Updates

### Will updates break config?
No, configuration is preserved