# Migrating Pulse

## Quick Migration Guide

### ❌ DON'T: Copy files directly
Never copy `/etc/pulse` or `/var/lib/pulse` directories between systems:
- The encryption key is tied to the files
- Credentials may be exposed
- Configuration may not work on different systems

### ✅ DO: Use Export/Import

#### Exporting (Old Server)
1. Open Pulse web interface
2. Go to **Settings** → **Configuration Management**
3. Click **Export Configuration**
4. Enter a strong passphrase (you'll need this for import!)
5. Save the downloaded file securely

#### Importing (New Server)
1. Install fresh Pulse instance
2. Open Pulse web interface  
3. Go to **Settings** → **Configuration Management**
4. Click **Import Configuration**
5. Select your exported file
6. Enter the same passphrase
7. Click Import

## What Gets Migrated

✅ **Included:**
- All PVE/PBS nodes and credentials
- Alert settings and thresholds
- Email configuration
- Webhook configurations
- System settings
- Guest metadata (custom URLs, notes)

❌ **Not Included:**
- Historical metrics data
- Alert history
- Authentication settings (passwords, API tokens)
- Each instance should configure its own authentication

## Common Scenarios

### Moving to New Hardware
1. Export from old server
2. Shut down old Pulse instance
3. Install Pulse on new hardware
4. Import configuration
5. Verify all nodes are connected

### Docker to Systemd (or vice versa)
The export/import process works across all installation methods:
- Docker → Systemd ✅
- Systemd → Docker ✅
- Docker → LXC ✅

### Backup Strategy
**Weekly Backups:**
1. Export configuration weekly
2. Store exports with date: `pulse-backup-2024-01-15.enc`
3. Keep last 4 backups
4. Store passphrase securely (password manager)

### Disaster Recovery
1. Install Pulse: `curl -sL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash`
2. Import latest backup
3. System restored in under 5 minutes!

## Security Notes

- **Passphrase Protection**: Exports are encrypted with PBKDF2 (100,000 iterations)
- **Safe to Store**: Encrypted exports can be stored in cloud backups
- **Minimum 12 characters**: Use a strong passphrase
- **Password Manager**: Store your passphrase securely

## Troubleshooting

**"Invalid passphrase" error**
- Ensure you're using the exact same passphrase
- Check for extra spaces or capitalization

**Missing nodes after import**
- Verify the export was taken after adding the nodes
- Check Settings to ensure nodes are listed

**Connection errors after import**
- Node IPs may have changed
- Update node addresses in Settings

## Pro Tips

1. **Test imports**: Try importing on a test instance first
2. **Document changes**: Note any manual configs not in Pulse
3. **Version matching**: Best to import into same or newer Pulse version
4. **Network access**: Ensure new server can reach all nodes

---

*Remember: Export/Import is the ONLY supported migration method. Direct file copying is not supported and may result in data loss.*