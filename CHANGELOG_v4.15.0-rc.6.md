## Note for RC versions
This is a pre-release version for testing. Consider backing up your Pulse configuration before updating.

## What's Changed

### Improvements
- **Enhanced diagnostics export** - Now includes critical troubleshooting data:
  - Backend diagnostics when available
  - Node online/offline status
  - Physical disks information  
  - ZFS pool health details
  - Alert configuration
  - Connection health metrics
  - Helps identify issues like missing storage, timeout errors, and threshold save problems

### Documentation  
- Added GitHub issue templates with diagnostics export instructions
- Templates guide users to provide better information when reporting issues

### Why this matters
This release significantly improves troubleshooting capabilities. The enhanced diagnostics export will help identify and resolve issues like:
- Storage only showing from one node (when others timeout)
- 504 errors when saving thresholds
- Missing physical disk information
- Alert configuration problems

## Testing v4.15.0-rc.6

**Install script:**
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version v4.15.0-rc.6
```

**Docker:**
```bash
docker pull rcourtman/pulse:v4.15.0-rc.6
docker run -d --name pulse -p 7655:7655 -v pulse-data:/etc/pulse rcourtman/pulse:v4.15.0-rc.6
```

## How to provide feedback
If you're still experiencing issues after updating:
1. Go to Settings → Diagnostics tab
2. Click "Run Diagnostics" first
3. Then click "Export for GitHub" 
4. Attach the exported file to your issue report

## Downloads
Pre-built binaries available below for linux-amd64, linux-arm64, and linux-armv7.
