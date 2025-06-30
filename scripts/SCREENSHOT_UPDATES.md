# Screenshot Script - Streamlined Version

## Overview

The screenshot script has been simplified to capture only the 6 essential screenshots that best showcase Pulse's core functionality.

## Screenshots Captured:

1. **`01-dashboard.png`** - Main dashboard view showing VMs/containers overview
2. **`02-pbs-view.png`** - Proxmox Backup Server integration view
3. **`03-storage-view.png`** - Storage overview showing all datastores
4. **`04-backups-view.png`** - Backups overview and management
5. **`05-charts-view.png`** - Real-time performance charts with tooltips
6. **`06-alerts-view.png`** - Alerts/thresholds configuration view

## Technical Details:

- **Resolution**: 2880x1800 (2x device scale factor)
- **Viewport**: 1440x900 (16:10 aspect ratio)
- **Mode**: Dark mode enforced for consistency
- **Font**: Inter font with antialiasing
- **Format**: PNG with full color

## Running the Script:

```bash
npm run screenshot
```

Or with custom URL:
```bash
PULSE_URL=http://192.168.1.100:7655 npm run screenshot
```

## Output:

Screenshots are saved to `/opt/pulse/docs/images/`

## Notes:

- Mobile screenshots have been removed to focus on desktop experience
- Settings/configuration screenshots removed for privacy
- Script automatically cleans up old screenshots before capturing new ones
- If PBS or backups data is not available, empty views will be captured