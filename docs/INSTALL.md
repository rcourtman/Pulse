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

### Kubernetes (Helm)

Use the bundled Helm chart for Kubernetes clusters:

```bash
helm registry login ghcr.io
helm install pulse oci://ghcr.io/rcourtman/pulse-chart \
  --version $(curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/VERSION) \
  --namespace pulse \
  --create-namespace
# Replace the VERSION lookup with a specific release tag (without "v") if you need to pin.

# Developing locally? Install from the checked-out chart directory instead:
# helm upgrade --install pulse ./deploy/helm/pulse \
#   --namespace pulse \
#   --create-namespace
```

Read the full [Kubernetes deployment guide](KUBERNETES.md) for ingress, persistence, and Docker agent configuration.

## Updating

### Automatic Updates (Recommended)

Pulse can automatically install stable updates to ensure you're always running the latest secure version:

#### Enable During Installation
```bash
# Interactive prompt during fresh install
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash

# Or force enable with flag
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --enable-auto-updates

# Install specific version (e.g., v4.24.0)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.24.0
```

#### Enable/Disable After Installation
```bash
# Via systemctl
systemctl enable --now pulse-update.timer   # Enable auto-updates
systemctl disable --now pulse-update.timer  # Disable auto-updates
systemctl status pulse-update.timer         # Check status

# Via Settings UI
# Navigate to Settings → System → Enable "Automatic Updates"
```

#### How It Works
- Checks daily between 2-6 AM (randomized to avoid server load)
- Only installs stable releases (never release candidates)
- Creates backup before updating
- Automatically rolls back if update fails
- Logs all activity to systemd journal
- **New in v4.25.0**: Adaptive monitoring now ships with circuit breakers, staleness tracking, and richer poll metrics while the Helm chart streamlines Kubernetes installs bundled with the binary.
- **New in v4.24.0**: Rollback history is retained in Settings → System → Updates; use the new 'Restore previous version' button if the latest build regresses

#### View Update Logs
```bash
journalctl -u pulse-update      # View all update logs
journalctl -u pulse-update -f   # Follow logs in real-time
systemctl list-timers pulse-update  # See next scheduled check
```

### Manual Updates

#### For LXC Containers
```bash
pct exec <container-id> -- update
```

#### For Standard Installations
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

#### For Docker
```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
docker run -d --name pulse -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest
```

### Rollback to Previous Version

**New in v4.25.0:** Pulse retains previous versions and allows easy rollback if an update causes issues, now backed by detailed scheduler metrics so you can see why a rollback triggered.

#### Via UI (Recommended)
1. Navigate to **Settings → System → Updates**
2. Click **"Restore previous version"** button
3. Confirm rollback
4. Pulse will restart with the previous working version

#### Via CLI
```bash
# For systemd installations
sudo /opt/pulse/pulse config rollback

# For LXC containers
pct exec <container-id> -- bash -c "cd /opt/pulse && ./pulse config rollback"
```

Rollback history and metadata are tracked in the Updates view. Check system journal for detailed rollback logs:
```bash
journalctl -u pulse | grep rollback
```

## Version Management

### Install Specific Version
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.24.0
```

### Install Release Candidate
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --rc
```

### Install from Source (Testing)
Build and install directly from the main branch to test the latest fixes before they're released:
```bash
# Install from main branch (latest development code)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --source

# Install from a specific branch
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --source develop
```
**Note:** This builds Pulse from source code on your machine. Requires Go, Node.js, and npm.

## Advanced Configuration

### Runtime Logging Configuration

**New in v4.25.0:** Adjust logging settings without restarting Pulse; the structured logging subsystem now centralizes format, destinations, and rotation controls.

#### Via UI
Navigate to **Settings → System → Logging** to configure:
- **Log Level**: debug, info, warn, error
- **Log Format**: json, text
- **File Rotation**: size limits and retention

#### Via Environment Variables
```bash
# Systemd
sudo systemctl edit pulse
[Service]
Environment="LOG_LEVEL=debug"
Environment="LOG_FORMAT=json"

# Docker
docker run -e LOG_LEVEL=debug -e LOG_FORMAT=json rcourtman/pulse:latest
```

### Adaptive Polling

**New in v4.25.0:** Adaptive polling now publishes staleness scores, circuit breaker states, and poll timings in `/api/monitoring/scheduler/health`, giving operators context when the scheduler slows down.

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
