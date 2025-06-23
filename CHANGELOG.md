## 🎉 Version 3.31.0

This stable release brings major improvements to backup monitoring, alert system enhancements, and numerous bug fixes for a more reliable monitoring experience.

---

## What's Changed

### ✨ New Features
- **Enhanced PBS Dashboard** - Revolutionary PBS2 interface with Timeline, Matrix & Flow views for better backup visualization
- **Backup Display Options** - Added grouped/list view toggle and group by node functionality for backup tables
- **Alert System Overhaul** - Per-guest alert configuration with integrated table UI and live preview
- **Email Cooldown System** - Prevent alert spam with configurable cooldown periods
- **Toast Notifications** - Real-time alert notifications for better visibility
- **PBS Namespace Support** - Full support for PBS namespace filtering across all backup views
- **Webhook Configuration** - Improved webhook setup with test buttons for easier configuration

### 🚀 Improvements  
- **Backup Health Insights** - Smart backup summary cards with meaningful insights and better UX
- **Alert Integration** - Consolidated alert interface with threshold-based configuration
- **Performance Optimizations** - Comprehensive chart rendering improvements eliminate blinking
- **Email Templates** - Unified HTML templates for professional alert emails
- **Namespace Filtering** - Robust filtering for PBS backups with VMID collision handling
- **Calendar Enhancements** - Improved date selection with namespace-aware filtering

### 🐛 Bug Fixes
- **VM/CT Name Display** - Shows meaningful names instead of hex IDs in PBS tab (#172)
- **Accurate Uptime Data** - Fixed uptime display using individual VM status endpoints (#180)
- **I/O Rate Metrics** - Resolved 0 B/s display issue for I/O rates
- **Backup Age Calculations** - Properly handles clock sync issues preventing negative ages
- **Chart Stability** - Eliminated chart blinking on updates and filter changes
- **Alert Timing** - Fixed alert duration dropdown to properly delay triggering
- **Table Layouts** - Standardized row heights and improved visual consistency
- **Filter Performance** - Removed delays in backup type filtering

---

## 📥 Installation & Updates

See the [Updating Pulse](https://github.com/rcourtman/Pulse#-updating-pulse) section in README for detailed update instructions.

### 🐳 Docker Images
- `docker pull rcourtman/pulse:v3.31.0`
- `docker pull rcourtman/pulse:latest`