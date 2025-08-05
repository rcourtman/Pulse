# ⚠️ IMPORTANT: Proxmox Helper Script Issue

## The Problem

The Proxmox VE Helper Scripts (community-scripts) are still configured for Pulse v3 (Node.js version) and **will not work** with Pulse v4 (Go version).

### Why Fresh Installs Are Failing

When users run the helper script:
```bash
bash -c "$(wget -qLO - https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/ct/pulse.sh)"
```

It installs Node.js and creates a systemd service that tries to run `npm start`, but v4 doesn't have a `package.json` file because it's a Go binary.

## Temporary Solution

Until the helper scripts are updated, users should:

1. **Manual Installation**
   ```bash
   wget https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh
   chmod +x install.sh
   ./install.sh
   ```

2. **Or use Docker**
   ```bash
   docker run -d --name pulse -p 7655:7655 rcourtman/pulse:latest
   ```

## Current Status

**A PR has been submitted** to update the Proxmox VE helper scripts for Pulse v4. Once merged, the helper script will work correctly with the Go version.

The updated script will:
- Download the pre-built binary (not install Node.js)
- Create proper systemd service for the Go binary
- Use port 7655 (not 3000)
- Handle the new directory structure

## For Users Getting Errors

If you see these errors:
- "Missing package.json"
- "npm: command not found"
- "Exit code 254"
- Service failing to start

This means the old helper script was used. You need to:
1. Remove the broken installation
2. Use the manual installation method above