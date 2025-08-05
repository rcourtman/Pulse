# Issue Response Templates

## For Issue #248 (Migration wiki link not accessible)

```markdown
Thank you for reporting this. I've now added a comprehensive migration guide directly to the repository:

📚 **[Migration Guide: v3 to v4](https://github.com/rcourtman/Pulse/blob/main/docs/MIGRATION_V3_TO_V4.md)**

This guide covers:
- Why automatic upgrades don't work (complete rewrite from Node.js to Go)
- Step-by-step migration process
- Configuration mapping from v3 to v4
- Troubleshooting common issues

The wiki was not properly configured, so I've moved all documentation into the main repository for better accessibility.
```

## For Issues #251 & #252 (Fresh install failures)

```markdown
Thank you for reporting this issue. The problem is that the Proxmox VE helper script is still configured for Pulse v3 (Node.js version), which is why you're seeing "missing package.json" errors.

**Immediate Solutions:**

1. **Use Docker (Recommended):**
   ```bash
   docker run -d -p 7655:7655 -v pulse_data:/data --restart unless-stopped rcourtman/pulse:latest
   ```

2. **Manual Installation:**
   ```bash
   wget https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh
   chmod +x install.sh
   ./install.sh
   ```

**Status:** I've already submitted a PR to fix the Proxmox helper script. Once merged, the automated installation will work again.

For more details, see:
- [Migration Guide](https://github.com/rcourtman/Pulse/blob/main/docs/MIGRATION_V3_TO_V4.md) if upgrading from v3
- [Installation Instructions](https://github.com/rcourtman/Pulse#quick-start-2-minutes) (updated)

The v4.0.1 release also includes fixes for Docker persistence issues.
```

## For Issue #249 (Docker persistence - CLOSED)

```markdown
This issue has been fixed in v4.0.1! 

The application now properly respects the `PULSE_DATA_DIR` environment variable, and Docker containers will persist configuration and alerts correctly.

To upgrade:
```bash
docker pull rcourtman/pulse:v4.0.1
# or
docker pull rcourtman/pulse:latest
```

Make sure you're mounting a volume to `/data`:
```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse-data:/data \
  rcourtman/pulse:v4.0.1
```

Thank you for reporting this issue!
```

## For Issue #250 (PBS token auth - CLOSED)

```markdown
This issue has been fixed in v4.0.1!

PBS token authentication now properly handles various token formats. You can enter tokens in multiple ways:
- Full token ID in the Token Name field: `user@realm!tokenname`
- Or enter user and token name separately
- The system will parse it correctly either way

To get the fix, please update to v4.0.1:
- Docker: `docker pull rcourtman/pulse:v4.0.1`
- Manual: Download from [releases](https://github.com/rcourtman/Pulse/releases/tag/v4.0.1)

Thank you for reporting this issue!
```