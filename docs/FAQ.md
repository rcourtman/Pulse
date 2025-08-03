# Frequently Asked Questions

## General

### What is Pulse?
Pulse is a real-time monitoring tool for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS). It provides a modern web interface with alerts, webhooks, and comprehensive monitoring capabilities.

### Is Pulse free?
Yes, Pulse is completely free and open source under the MIT license.

### What versions of Proxmox are supported?
- Proxmox VE 7.0 and later
- Proxmox Backup Server 2.0 and later

## Installation

### What's the easiest way to install Pulse?
The automated LXC container script is the easiest method:
```bash
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```

### Can I run Pulse in Docker?
Yes! Pulse has official Docker images:
```bash
docker run -d -p 7655:7655 -v pulse_config:/etc/pulse -v pulse_data:/data rcourtman/pulse:latest
```

### What are the minimum system requirements?
- CPU: 1 vCPU
- RAM: 512MB (1GB recommended)
- Disk: 1GB
- Network: Access to Proxmox API

## Configuration

### How do I add a Proxmox node?
1. Open Pulse web UI
2. Go to Settings → Nodes
3. Click "Add Node"
4. Enter your Proxmox credentials
5. Click "Test Connection" then "Save"

### What permissions does the Pulse user need?
- For PVE: `PVEAuditor` role minimum
- For PBS: `DatastoreReader` permission
- For full features: `PVEAdmin` recommended

### Should I use username/password or API tokens?
API tokens are more secure and recommended for production use. They can be created in Proxmox under Datacenter → Permissions → API Tokens.

### How do I configure alerts?
1. Go to Alerts tab
2. Set your thresholds for CPU, Memory, and Disk
3. Add notification channels (email, webhooks)
4. Configure alert schedule if needed

## Troubleshooting

### Why is Pulse not showing any data?
1. Check if Pulse can reach your Proxmox server
2. Verify credentials are correct
3. Check firewall rules for ports 8006 (PVE) or 8007 (PBS)
4. Look at Pulse logs: `journalctl -u pulse -f`

### Connection refused errors
- Ensure port 7655 is not blocked
- Check if Pulse is running: `systemctl status pulse`
- Verify bind addresses in configuration

### Invalid credentials error
- Ensure user exists in Proxmox
- Check realm is correct (e.g., @pam, @pve)
- For API tokens, ensure token is not expired
- Verify user has required permissions

### High memory usage
- This is usually from storing metrics history
- Reduce `metricsRetentionDays` in settings
- Restart Pulse to clear old metrics

## Security

### Is Pulse secure?
Pulse is designed for internal networks. It uses encrypted storage for credentials and supports various security levels. See the [Security Guide](SECURITY.md) for details.

### Can I expose Pulse to the internet?
This is not recommended. If needed:
1. Use a reverse proxy with authentication
2. Enable HTTPS
3. Use strong passwords
4. Consider VPN access instead

### How are credentials stored?
Credentials are encrypted using AES-256-GCM and stored in `/etc/pulse/pulse.enc`. The encryption key is derived from the machine ID.

## Features

### Can Pulse monitor multiple Proxmox clusters?
Yes! You can add multiple PVE and PBS instances in Settings → Nodes.

### Does Pulse support PBS in push mode?
Yes, for isolated PBS servers, you can use the PBS agent that pushes data to Pulse.

### What webhook providers are supported?
- Discord
- Slack
- Gotify
- Telegram
- ntfy.sh
- Microsoft Teams
- Generic webhooks (any JSON endpoint)

### Can I use Pulse with a reverse proxy?
Yes, Pulse works well behind reverse proxies like Nginx, Caddy, or Traefik. Make sure to configure WebSocket support.

### Does Pulse have an API?
Yes, Pulse provides a REST API. See the API section in the README for endpoints.

## Updates

### How do I update Pulse?
- **Docker**: Pull the latest image and recreate the container
- **LXC/Manual**: Use the update script or download the latest release
- **From source**: Git pull and rebuild

### Will updates break my configuration?
No, Pulse maintains backward compatibility. Your configuration is preserved during updates.

### How do I know when updates are available?
- Watch the GitHub repository for releases
- Check the version in Settings → About
- Enable GitHub notifications for the project