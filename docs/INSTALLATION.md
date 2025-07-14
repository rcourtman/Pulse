# Pulse Installation Guide

This guide covers all installation methods for Pulse in detail.

## Quick Decision Guide

```
Do you have Docker installed?
├─ Yes → Use Docker Compose method
└─ No → Running on Proxmox?
        ├─ Yes → Want a dedicated container?
        │        ├─ Yes → Use Community Scripts
        │        └─ No → Use Manual Install in existing LXC/VM
        └─ No → Use Docker or Manual Install
```

## Installation Methods

### Method 1: Proxmox Community Scripts (Easiest)

**Best for:** New users wanting a dedicated monitoring container

**What it does:**
- Creates a new LXC container automatically
- Installs all dependencies
- Sets up systemd service
- Configures automatic updates (optional)

**Requirements:**
- Proxmox VE host with root access
- Internet connectivity
- Available container ID

**Installation:**
```bash
bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"
```

**The script will prompt for:**
1. Container ID (next available suggested)
2. Hostname (default: pulse)
3. Resource allocation (1 CPU core, 512MB RAM recommended)
4. Storage location
5. Network configuration
6. Automatic updates (recommended)

**Post-installation:**
- Access Pulse at `http://<container-ip>:7655`
- Configure via web interface

### Method 2: Docker Compose

**Best for:** Existing Docker hosts, multi-container setups

**Requirements:**
- Docker Engine 20.10+
- Docker Compose 2.0+
- Port 7655 available

**Installation:**

1. **Create directory:**
   ```bash
   mkdir -p ~/pulse && cd ~/pulse
   ```

2. **Create docker-compose.yml:**
   ```yaml
   services:
     pulse:
       image: rcourtman/pulse:latest
       container_name: pulse
       restart: unless-stopped
       ports:
         - "7655:7655"
       volumes:
         - pulse_config:/usr/src/app/config
         - pulse_data:/usr/src/app/data
       environment:
         - TZ=America/New_York  # Optional: Set your timezone

   volumes:
     pulse_config:
     pulse_data:
   ```

3. **Start container:**
   ```bash
   docker compose up -d
   ```

4. **Verify:**
   ```bash
   docker compose ps
   docker compose logs
   ```

**Advanced Docker Options:**

Using specific version:
```yaml
image: rcourtman/pulse:v3.42.0
```

Using host networking:
```yaml
network_mode: host
ports: []  # Remove ports when using host networking
```

Custom port:
```yaml
ports:
  - "8080:7655"  # Access on port 8080
```

### Method 3: Manual Installation

**Best for:** Existing LXC containers, VMs, or bare metal

**Requirements:**
- Debian 11+ or Ubuntu 20.04+
- sudo access
- Internet connectivity

**Installation:**

1. **Quick install:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh | sudo bash
   ```

2. **Or download and review first:**
   ```bash
   curl -sLO https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh
   cat install-pulse.sh  # Review the script
   chmod +x install-pulse.sh
   sudo ./install-pulse.sh
   ```

**What the script does:**
1. Installs dependencies (Node.js 20, git, curl)
2. Creates pulse user and directories
3. Downloads latest Pulse release
4. Sets up systemd service
5. Optionally configures automatic updates

**Manual steps (if script fails):**

```bash
# Install Node.js 20
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs git

# Create user and directories
sudo useradd -r -s /bin/false pulse
sudo mkdir -p /opt/pulse
sudo chown pulse:pulse /opt/pulse

# Download and extract
cd /opt/pulse
sudo -u pulse git clone https://github.com/rcourtman/Pulse.git .
sudo -u pulse npm install --production

# Create systemd service
sudo tee /etc/systemd/system/pulse-monitor.service > /dev/null <<EOF
[Unit]
Description=Pulse Monitor
After=network.target

[Service]
Type=simple
User=pulse
WorkingDirectory=/opt/pulse
ExecStart=/usr/bin/node server.js
Restart=always
Environment=NODE_ENV=production

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable --now pulse-monitor
```

### Method 4: Release Tarball

**Best for:** Air-gapped environments, specific version requirements

1. **Download from [GitHub Releases](https://github.com/rcourtman/Pulse/releases)**

2. **Extract:**
   ```bash
   mkdir pulse-app && cd pulse-app
   tar -xzf pulse-v3.42.0.tar.gz
   cd pulse-v3.42.0
   ```

3. **Run:**
   ```bash
   npm start
   ```

### Method 5: Development Setup

**Best for:** Contributing, testing new features

See [DEVELOPMENT.md](../DEVELOPMENT.md) for details.

## Post-Installation Steps

### 1. Initial Configuration

1. Open `http://<pulse-ip>:7655`
2. Settings modal opens automatically
3. Add Proxmox connection:
   - URL: `https://your-proxmox:8006`
   - Token ID & Secret (see next section)
4. Test connection
5. Save configuration

### 2. Create API Tokens

**Proxmox VE:**
```bash
# On Proxmox host
pveum user add pulse@pam
pveum user token add pulse@pam monitoring --privsep 0
pveum acl modify / --users pulse@pam --roles PVEAuditor
pveum acl modify /storage --users pulse@pam --roles PVEDatastoreAdmin
```

**PBS (if using):**
```bash
# On PBS host
proxmox-backup-manager user create pulse@pbs --password 'TempPass'
proxmox-backup-manager user generate-token pulse@pbs monitoring
proxmox-backup-manager acl update /datastore DatastoreAudit --auth-id 'pulse@pbs!monitoring'
```

**Verify Permissions:**
After creating tokens, you can verify permissions are correctly set:
```bash
# Check PVE permissions
cd /opt/pulse
./scripts/check-pve-permissions.sh

# Check PBS permissions
./scripts/check-pbs-permissions.sh

# Auto-fix any permission issues
./scripts/check-pve-permissions.sh --fix
./scripts/check-pbs-permissions.sh --fix
```

### 3. Configure Firewall

Allow Pulse to reach Proxmox APIs:

```bash
# On Proxmox hosts
# If using Proxmox firewall, allow from Pulse IP
```

Allow access to Pulse web interface:
```bash
# On Pulse host
sudo ufw allow 7655/tcp
```

### 4. Set Up Monitoring

1. Configure alert thresholds
2. Set up notifications (email/webhooks)
3. Add custom per-VM thresholds if needed
4. Enable automatic updates (recommended)

## Verifying Installation

### Check Service Status

**Systemd installation:**
```bash
sudo systemctl status pulse-monitor
sudo journalctl -u pulse-monitor -f
```

**Docker installation:**
```bash
docker compose ps
docker compose logs -f
```

### Run Diagnostics

Open `http://<pulse-ip>:7655/diagnostics.html` to verify:
- Configuration is valid
- API connections work
- Permissions are correct
- Data is being collected

## Updating Pulse

### Web Interface (Non-Docker)
Settings → Software Updates → Check for Updates

### Command Line
```bash
# LXC/Manual
sudo /opt/pulse/scripts/install-pulse.sh --update

# Docker
docker compose pull && docker compose up -d

# Community Scripts
# Re-run the original installation command
```

## Uninstalling

### Docker
```bash
docker compose down
docker volume rm pulse_config pulse_data
```

### Manual/LXC
```bash
sudo systemctl stop pulse-monitor
sudo systemctl disable pulse-monitor
sudo rm -rf /opt/pulse
sudo userdel pulse
sudo rm /etc/systemd/system/pulse-monitor.service
```

## Troubleshooting Installation

### Common Issues

**Port already in use:**
```bash
# Find what's using port 7655
sudo lsof -i :7655
# Use a different port in configuration
```

**Permission denied errors:**
- Ensure running installation with sudo
- Check directory ownership

**Node.js version issues:**
- Pulse requires Node.js 18+
- Update Node.js if needed

**Network connectivity:**
- Verify Pulse can reach Proxmox hosts
- Check firewall rules
- Test with curl: `curl https://your-proxmox:8006`

For more help, see [Troubleshooting Guide](TROUBLESHOOTING.md).