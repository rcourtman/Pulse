# v3.31.0-rc2 Release Notes

## 🎉 Major Features

### 🚨 Alert System
- **Real-time Alerts**: Get notified when VMs or containers exceed resource thresholds
- **Multiple Notification Channels**: Send alerts to Discord, Slack, Teams, Telegram, and email
- **Per-Guest Thresholds**: Set custom alert limits for individual VMs and containers
- **Alert History**: View complete timeline of when alerts triggered and resolved
- **Smart Suppression**: Prevent alert spam with configurable quiet periods

### 📊 Backup Improvements
- **PBS Namespace Support**: View and filter backups by namespace
- **Backup Health Dashboard**: See backup status for all guests at a glance
- **Group by Node**: Organize backup view by physical host
- **Flexible Display**: Toggle between grouped and list views
- **Enhanced Search**: Better filtering for finding specific backups

### 🎨 Interface Enhancements
- **Faster Performance**: Smoother scrolling for large VM lists
- **Better Accessibility**: Improved color contrast for better readability
- **Mobile Improvements**: Enhanced touch scrolling and responsive tables
- **Loading Indicators**: Clear feedback while data loads

## 🐛 Bug Fixes

- Fixed flickering charts when switching between views
- Resolved backup filter layout issues
- Fixed incorrect time displays in PBS backup listings
- Corrected backup counts in health summary
- Fixed UI state loss when hot reloading during development

## 📦 Update Instructions

For detailed update instructions for all installation methods, see:
**https://github.com/rcourtman/Pulse/blob/main/docs/UPDATING.md**

Quick update options:
- **Web Interface**: Settings → System → Check for Updates
- **Script**: `./install-pulse.sh --update`
- **Docker**: `docker pull rcourtman/pulse:v3.33.0-rc2`

---

*This is a release candidate. Please test thoroughly before using in production.*