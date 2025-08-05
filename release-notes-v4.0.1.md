# Pulse v4.0.1 - Bug Fixes

## 🐛 Bug Fixes

### PBS Token Authentication
- **Fixed "invalid user format" error** when configuring PBS with API tokens (#250)
  - PBS token authentication now properly handles various token formats
  - Supports entering full token ID (`user@realm!tokenname`) or just token name
  - Test connection and save operations now use consistent validation

### Docker Persistence
- **Fixed configuration not persisting in Docker containers** (#249)
  - Resolved "permission denied" errors when creating data directories
  - Docker containers now properly respect `PULSE_DATA_DIR` environment variable
  - Discord webhooks and other settings now persist across container restarts
  - Alert history is preserved between container runs

## 🔧 Technical Improvements

- Added configurable data directory support via `PULSE_DATA_DIR` environment variable
- Improved error handling for PBS authentication methods
- Better parsing of PBS token formats from Proxmox

## 📦 Downloads

### Universal Package (Auto-detects architecture)
- [pulse-v4.0.1.tar.gz](https://github.com/rcourtman/Pulse/releases/download/v4.0.1/pulse-v4.0.1.tar.gz)

### Architecture-Specific
- [pulse-v4.0.1-linux-amd64.tar.gz](https://github.com/rcourtman/Pulse/releases/download/v4.0.1/pulse-v4.0.1-linux-amd64.tar.gz) - Intel/AMD 64-bit
- [pulse-v4.0.1-linux-arm64.tar.gz](https://github.com/rcourtman/Pulse/releases/download/v4.0.1/pulse-v4.0.1-linux-arm64.tar.gz) - ARM 64-bit (RPi 4/5)
- [pulse-v4.0.1-linux-armv7.tar.gz](https://github.com/rcourtman/Pulse/releases/download/v4.0.1/pulse-v4.0.1-linux-armv7.tar.gz) - ARM 32-bit

## 🐳 Docker

```bash
docker pull rcourtman/pulse:v4.0.1
# or
docker pull rcourtman/pulse:latest
```

## 📝 Upgrade Notes

This is a patch release with bug fixes only. It's safe to upgrade from v4.0.0.

For Docker users: If you were experiencing issues with settings not persisting, ensure you're mounting a volume to `/data`:

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse-data:/data \
  -e PROXMOX_HOST=your-proxmox-host \
  rcourtman/pulse:v4.0.1
```