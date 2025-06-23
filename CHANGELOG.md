## 🎉 Version 3.31.0

### ✨ New Features

- **Backup Display Modes**: Added display toggle (Grouped/List) and group by node functionality to backups table for better organization
- **PBS Namespace Support**: Enhanced PBS namespace extraction to support alternative formats and added namespace support to diagnostics
- **Alert System Overhaul**: 
  - Added per-guest alert configuration with integrated table UI
  - Implemented email cooldown system to prevent alert spam with configurable settings
  - Added email/webhook cooldown settings and test buttons to alerts UI
  - Enhanced alert email system with unified HTML templates
  - Added toast notifications for alerts and improved high-volume handling
  - Integrated alert creation into threshold section with live preview
- **Threshold Management**: 
  - Added threshold hide mode and improved view mode isolation
  - Enhanced threshold UX with dimming and smart reset button
- **Webhook Configuration**: Improved webhook configuration and testing experience
- **Chart Performance**: Implemented comprehensive chart rendering performance optimizations

### 🚀 Improvements

- **Backup Health Summary**: Major improvements to backup health summary and display with smart insights and better UX
- **Namespace Filtering**: 
  - Improved PBS and backups namespace filtering for multiple PBS instances
  - Enhanced namespace filtering to use VMID-based matching
  - Robust namespace filtering for PBS backups with VMID collisions
- **Alert System Integration**: Enhanced alerts system integration and threshold row consistency
- **UI Consistency**: 
  - Reorganized backups filter layout to match main dashboard
  - Standardized alerts table row heights to match main dashboard
  - Improved table layout and alerts UI alignment
  - Improved sticky column visual separation and consistency across tables

### 🐛 Bug Fixes

- **Chart Rendering**: 
  - Resolved chart blinking by fixing ID mismatches
  - Eliminated chart blinking by removing deferred updates and transitions
  - Removed debounce delay to restore immediate chart updates
- **Backup Display Issues**: 
  - Fixed PBS tab improvements for namespace handling and time range display
  - Ensured PVE backups display correctly when namespace filters are active (#172)
  - Fixed PBS backup time display in All namespaces view
  - Fixed comprehensive backup tab improvements for namespace filtering and calendar display
  - Eliminated backup tab visual blinking on load and filter changes
- **Alert System Fixes**: 
  - Fixed alert duration dropdown to properly delay alert triggering
  - Resolved alert system issues and enhanced I/O alert display
  - Fixed use of calculated I/O rates for alert thresholds instead of raw counters
  - Fixed alert row styling to instantly respond to toggle state
- **Namespace Filter Fixes**: 
  - Fixed namespace filter to properly apply to calendar date selection
  - Included namespace filter in active filters check for backup detail card
  - Improved backup namespace filtering and owner token matching
  - Fixed PBS owner token matching for multi-cluster environments
- **Clock Sync Issues**: 
  - Properly handled clock sync issues in backup age calculations
  - Fixed handling of negative backup ages from clock sync issues
- **VM Metrics**: 
  - Used accurate uptime from individual VM status endpoint (#180)
  - Resolved metrics showing 0 B/s for I/O rates due to bulk endpoint caching
- **UI Fixes**: 
  - Fixed calendar date selection to use namespace-aware table data
  - Repaired backup health filters and simplified layout
  - Prevented disk slider tooltip from appearing inappropriately
  - Prevented row undimming flash during alert threshold reset
  - Improved alert dropdown sizing to be more reasonable
- **Configuration**: Fixed Docker configuration directory creation before writing configuration files

---

## 📥 Installation & Updates

See the [Updating Pulse](https://github.com/rcourtman/Pulse#-updating-pulse) section in README for detailed update instructions.

### 🐳 Docker Images
- `docker pull rcourtman/pulse:v3.31.0`
- `docker pull rcourtman/pulse:latest`