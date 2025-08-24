# Installation Guide

## Quick Install

The official installer automatically detects your environment and chooses the best installation method:

```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

The installer will prompt you for the port (default: 7655). To skip the prompt, set the environment variable:
```bash
FRONTEND_PORT=8080 curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

## Installation Methods

### Proxmox VE Hosts

When run on a Proxmox VE host, the installer automatically:
1. Creates a lightweight LXC container
2. Installs Pulse inside the container
3. Configures networking and security

**Quick Mode** (recommended):
- 1GB RAM, 4GB disk, 2 CPU cores
- Unprivileged container with firewall
- Auto-starts with your host
- Takes about 1 minute

**Advanced Mode**:
- Customize all container settings
- Choose specific network bridges and storage
- Configure static IP if needed
- Set custom port (default: 7655)

### Standard Linux Systems

On Debian/Ubuntu systems, the installer:
1. Installs required dependencies
2. Downloads the latest Pulse binary
3. Creates a systemd service
4. Starts Pulse automatically

### Docker

For containerized deployments:

```bash
docker run -d -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest
```

See [Docker Guide](DOCKER.md) for advanced options.

## Updating

### For LXC Containers
```bash
pct exec <container-id> -- update
```

### For Standard Installations
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

### For Docker
```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
docker run -d --name pulse -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest
```

## Version Management

### Install Specific Version
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.6.0
```

### Install Release Candidate
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --rc
```

## Troubleshooting

### Permission Denied
If you encounter permission errors, you may need to run with `sudo` on some systems, though most installations (including LXC containers) run as root and don't need it.

### Container Creation Failed
Ensure you have:
- Available container IDs (check with `pct list`)
- Sufficient storage space
- Network bridge configured

### Port Already in Use
Pulse uses port 7655 by default. You can change it during installation or check current usage with:
```bash
sudo netstat -tlnp | grep 7655
```
To use a different port during installation:
```bash
FRONTEND_PORT=8080 curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

## Uninstalling

### From LXC Container
```bash
pct stop <container-id>
pct destroy <container-id>
```

### From Standard System
```bash
sudo systemctl stop pulse
sudo systemctl disable pulse
sudo rm -rf /opt/pulse /etc/pulse
sudo rm /etc/systemd/system/pulse.service
```

### Docker
```bash
docker stop pulse
docker rm pulse
docker volume rm pulse_data  # Warning: deletes all data
```