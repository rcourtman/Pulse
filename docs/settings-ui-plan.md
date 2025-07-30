# Settings UI Enhancement Plan

To achieve the "everything through UI" goal like Radarr/Sonarr, the Settings page needs these sections:

## 1. General Settings
- **Backend Port**: Port for API (default: 3000)
- **Frontend Port**: Port for UI (default: 7655) 
- **Polling Interval**: How often to check nodes (seconds)
- **Log Level**: Debug, Info, Warn, Error
- **Metrics Retention**: Days to keep historical data

## 2. Node Management
- **Add Node** button opens form:
  - Node Type: Proxmox VE or PBS
  - Name: Friendly name
  - Host: URL (https://server:8006)
  - Authentication:
    - Username/Password OR
    - API Token (name + value)
  - SSL Verification toggle
  - What to Monitor (checkboxes)
- **Edit/Delete** existing nodes
- **Test Connection** button for each node

## 3. Notifications
- **Email Settings**:
  - SMTP Server, Port, TLS
  - From address, To addresses
  - Username/Password for auth
- **Webhooks**:
  - Add/Edit/Delete webhook URLs
  - Custom headers
  - Test webhook button

## 4. Alerts
- **Default Thresholds**:
  - CPU, Memory, Disk warning levels
  - Different defaults for nodes vs guests
- **Alert Schedule**:
  - Quiet hours configuration
  - Cooldown between alerts
  - Alert grouping settings

## 5. Security
- **API Token**: For external integrations
- **CORS Settings**: Allowed origins
- **Session Settings**: Timeout, etc.

## Implementation Strategy

1. Each section gets its own API endpoint
2. Settings are saved to pulse.yml automatically
3. Changes take effect immediately (or after quick restart)
4. No manual file editing required
5. Import/Export settings for backup

This makes Pulse as user-friendly as the apps you mentioned while maintaining flexibility for power users.