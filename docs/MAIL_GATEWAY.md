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
   - API Token ID: e.g., `root@pam!pulse`
   - API Token Secret: Your token secret

### Via Discovery

Pulse can automatically discover PMG instances on your network:

1. Go to **Settings → Discovery**
2. Enable network discovery
3. PMG instances on port 8006 will be detected
4. Click to add discovered instances

## API Token Setup on PMG

Create an API token on your PMG server:

```bash
# SSH to your PMG server
pveum user token add root@pam pulse --privsep 0

# Note the token secret - it's only shown once!
```

Required permissions:
- `Sys.Audit` - Read system status
- `Datastore.Audit` - Read mail statistics

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

Configure alerts for PMG metrics in **Settings → Alerts**:

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
3. Ensure API token has correct permissions

### No statistics showing
1. Wait for initial data collection (may take 1-2 polling cycles)
2. Verify PMG has mail activity
3. Check Pulse logs for API errors

### Cluster nodes missing
1. PMG cluster must be properly configured
2. API token needs cluster-wide permissions
3. All nodes must be reachable from Pulse
