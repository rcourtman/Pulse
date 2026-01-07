# Proxmox Mail Gateway (PMG) Monitoring

Pulse 5.0 adds support for monitoring Proxmox Mail Gateway instances alongside your PVE and PBS infrastructure.

## Features

- **Mail Queue Monitoring**: Track active, deferred, and held messages
- **Spam Statistics**: View spam detection rates and virus blocks
- **Cluster Status**: Monitor PMG cluster node health
- **Quarantine Overview**: See quarantine size and pending reviews

## Adding a PMG Instance

### Via Settings UI

1. Navigate to **Settings → Proxmox**
2. Click **Add Node**
3. Select **Proxmox Mail Gateway** as the type
4. Enter connection details:
   - Host: Your PMG IP or hostname
   - Port: 8006 (default)
   - Username: e.g., `root@pam` or a dedicated `api@pmg` user
   - Password: the PMG account password

### Via Discovery

Pulse can automatically discover PMG instances on your network:

1. Enable discovery in **Settings → System → Network**
2. Go to **Settings → Proxmox**
3. PMG instances on port 8006 are detected and shown in the Proxmox discovery panels
4. Click a discovered PMG server to add it

## Service Account Setup on PMG

PMG does not support API tokens. Use a dedicated PMG user with read-only access if possible:

- Create a user in the PMG UI (or CLI) such as `api@pmg`.
- Assign the minimum permissions needed to read mail statistics and cluster status.
- Use that username and password when adding the node in Pulse.

## Dashboard

The Mail Gateway tab shows:

| Metric | Description |
|--------|-------------|
| **Mail Processed** | Total emails processed today |
| **Spam Rate** | Percentage of spam detected |
| **Virus Blocked** | Malicious emails caught |
| **Queue Depth** | Messages pending delivery |
| **Quarantine Size** | Emails in quarantine |

### Status Indicators

- 🟢 **Healthy**: Normal operation
- 🟡 **Warning**: Queue building up or high spam rate
- 🔴 **Critical**: Delivery issues or cluster problems

## Alerts

Configure alerts for PMG metrics in **Alerts → Thresholds**:

- Queue depth exceeding threshold
- Spam rate spike
- Delivery failures
- Cluster node offline

## Multi-Instance Support

Monitor multiple PMG instances from a single Pulse dashboard:

- Compare spam rates across gateways
- Aggregate mail statistics
- View cluster-wide health

## Troubleshooting

### Connection refused
1. Verify PMG is accessible on port 8006
2. Check firewall rules
3. Ensure the PMG user/password is correct and has read permissions

### No statistics showing
1. Wait for initial data collection (may take 1-2 polling cycles)
2. Verify PMG has mail activity
3. Check Pulse logs for API errors

### Cluster nodes missing
1. PMG cluster must be properly configured
2. The PMG user needs cluster-wide permissions
3. All nodes must be reachable from Pulse
