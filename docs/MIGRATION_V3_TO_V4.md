# Pulse v3 to v4 Migration Guide

## ⚠️ CRITICAL: Manual Migration Required

**Pulse v4 is a complete rewrite from Node.js to Go**. Due to fundamental architecture changes, automatic upgrades from v3 to v4 are not supported and will break your installation.

## 🚫 DO NOT Attempt These Actions

1. **DO NOT run auto-update** from v3 to v4
2. **DO NOT use the old Proxmox helper script** - it's still configured for v3
3. **DO NOT try to upgrade in-place** - the applications are completely different

## ✅ Recommended Migration Process

### Option 1: Fresh Installation (Recommended)

1. **Create a new LXC container or VM** for Pulse v4
   ```bash
   # Create new container in Proxmox
   pct create [VMID] local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst \
     --hostname pulse-v4 \
     --memory 1024 \
     --cores 1 \
     --rootfs local-lvm:8 \
     --net0 name=eth0,bridge=vmbr0,ip=dhcp
   ```

2. **Install Pulse v4** in the new container
   ```bash
   # Download and run the v4 installer
   wget https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh
   chmod +x install.sh
   ./install.sh
   ```

3. **Configure your nodes** through the new web UI (port 7655)
   - Add your Proxmox VE nodes
   - Add your PBS instances (if applicable)
   - Configure notifications (Discord, email, etc.)
   - Set up alert thresholds

4. **Reference your old configuration** if needed
   - Check your v3 `.env` file for API tokens and credentials
   - Note your notification webhook URLs
   - Document your alert threshold preferences

5. **Test thoroughly** before decommissioning v3
   - Verify all nodes are being monitored
   - Test alert notifications
   - Ensure backups are visible (if using PBS)

6. **Shut down the old v3 instance** once v4 is working

### Option 2: Docker Migration

If you were using Docker for v3, the process is simpler:

1. **Stop and backup your v3 container**
   ```bash
   docker stop pulse-v3
   docker rename pulse-v3 pulse-v3-backup
   ```

2. **Run Pulse v4 with Docker**
   ```bash
   docker run -d \
     --name pulse \
     -p 7655:7655 \
     -v pulse-data:/data \
     -e PROXMOX_HOST=your-proxmox-host \
     -e PROXMOX_USER=your-api-user \
     -e PROXMOX_TOKEN_NAME=your-token-name \
     -e PROXMOX_TOKEN_VALUE=your-token-value \
     rcourtman/pulse:latest
   ```

3. **Configure through the web UI** at http://localhost:7655

## 📋 Configuration Reference

### Environment Variables (v3 → v4)

| v3 Variable | v4 Equivalent | Notes |
|------------|---------------|-------|
| `PROXMOX_HOST` | Same | Now configured via UI |
| `PROXMOX_USER` | Same | Now configured via UI |
| `PROXMOX_PASSWORD` | Same | Password OR token |
| `PROXMOX_TOKEN_ID` | Split into `PROXMOX_TOKEN_NAME` | Token auth recommended |
| `PROXMOX_TOKEN_SECRET` | `PROXMOX_TOKEN_VALUE` | Token auth recommended |
| `DISCORD_WEBHOOK` | Configure in UI | Settings → Notifications |
| `EMAIL_*` | Configure in UI | Settings → Notifications |

### Key Differences

1. **Port Change**: v3 used port 3000, v4 uses port **7655**
2. **No Node.js**: v4 is a Go binary, no npm/node required
3. **New UI**: Complete redesign with dark mode support
4. **Multi-node**: v4 supports multiple PVE/PBS instances natively
5. **Better Performance**: Significantly faster and lower resource usage

## 🔧 Troubleshooting

### "Missing package.json" Error
This means you're trying to run v4 with v3 startup scripts. Solution:
- Use the new systemd service file
- Don't use npm commands
- Run the binary directly: `/opt/pulse/pulse`

### Port 3000 Not Working
v4 uses port 7655 by default. Update your firewall rules and bookmarks.

### Can't Find .env File
v4 doesn't use .env files. Configuration is done through the web UI and stored encrypted.

### Credentials Not Working
v4 has its own authentication system. Default credentials on fresh install:
- No authentication by default
- Set up security mode in Settings → Security

## 📚 Additional Resources

- [Pulse v4 Release Notes](https://github.com/rcourtman/Pulse/releases/tag/v4.0.0)
- [Pulse v4 Documentation](https://github.com/rcourtman/Pulse#readme)
- [Report Issues](https://github.com/rcourtman/Pulse/issues)

## ⚡ Quick Start After Migration

1. Access the web UI at `http://your-server:7655`
2. Click Settings → Nodes → Add Node
3. Enter your Proxmox credentials
4. Configure notifications if desired
5. Customize alert thresholds as needed

Remember: v4 is a complete rewrite with many improvements. Take time to explore the new features!