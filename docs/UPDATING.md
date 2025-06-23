# Updating Pulse

This guide covers all methods for updating Pulse to the latest version.

## 🖥️ Web Interface Update (Recommended)

**Available from v3.27.2+**

### For Stable Releases
1. Open your Pulse web interface
2. Go to **Settings** → **System** tab → **Software Updates**
3. Ensure **"Stable"** channel is selected
4. Click **"Check for Updates"**
5. Click **"Apply Update"** when an update is available
6. The interface will automatically refresh after update

### For Release Candidates (Testing)
1. Open your Pulse web interface
2. Go to **Settings** → **System** tab → **Software Updates**
3. Select **"RC"** channel to receive release candidates
4. Click **"Check for Updates"**
5. Click **"Apply Update"** to install the RC version
6. The interface will automatically refresh after update

> **Note**: You can switch between Stable and RC channels at any time

## 🛠️ Script-Based Update

**For LXC, VMs, and regular installations:**

### Update to Latest Stable
```bash
cd /opt/pulse/scripts
./install-pulse.sh --update
```

### Update to Specific Version
```bash
cd /opt/pulse/scripts
./install-pulse.sh --update --version v3.34.0
```

### Update to Latest RC
```bash
cd /opt/pulse/scripts
./install-pulse.sh --update --version rc
```

## 🐳 Docker Update

### Using Docker Compose (Recommended)
```bash
# Update to latest stable
docker compose down
docker compose pull
docker compose up -d
```

### Manual Docker Update

#### Latest Stable Version
```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
docker run -d --name pulse -p 7655:7655 -v pulse-config:/app/config rcourtman/pulse:latest
```

#### Latest RC Version
```bash
docker pull rcourtman/pulse:rc
docker stop pulse
docker rm pulse
docker run -d --name pulse -p 7655:7655 -v pulse-config:/app/config rcourtman/pulse:rc
```

#### Specific Version
```bash
docker pull rcourtman/pulse:v3.34.0
docker stop pulse
docker rm pulse
docker run -d --name pulse -p 7655:7655 -v pulse-config:/app/config rcourtman/pulse:v3.34.0
```

## 📥 Fresh Installation

### Automated Installer
```bash
# Install latest stable
curl -sL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh | bash

# Install specific version
curl -sL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh | bash -s -- --version v3.34.0
```

### Manual Installation
1. Download the release tarball from [GitHub Releases](https://github.com/rcourtman/Pulse/releases)
2. Extract: `tar -xzf pulse-vX.X.X.tar.gz`
3. Enter directory: `cd pulse-vX.X.X`
4. Install dependencies: `npm install --production`
5. Start Pulse: `npm start`

## Version Channels

- **Stable**: Production-ready releases
- **RC**: Release candidates for testing new features
- **Specific versions**: Pin to a particular version if needed

## Troubleshooting

If you encounter issues during update:
1. Check the [release notes](https://github.com/rcourtman/Pulse/releases) for breaking changes
2. Ensure your Node.js version meets requirements (v18+)
3. Check logs: `journalctl -u pulse.service -n 50`
4. Report issues on [GitHub](https://github.com/rcourtman/Pulse/issues)