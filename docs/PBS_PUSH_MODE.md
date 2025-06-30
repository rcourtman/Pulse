# PBS Push Mode Documentation

This document describes how to set up and use the Push Mode feature for monitoring isolated PBS instances with Pulse.

## Overview

Push Mode allows PBS instances that are behind firewalls or in isolated networks to send their metrics to Pulse, rather than Pulse pulling the metrics. This is useful for:

- Secondary/disaster recovery PBS instances
- PBS servers in DMZ or isolated networks
- Environments where only outbound connections are allowed
- Air-gapped systems with periodic connectivity

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│   PBS Server    │         │  Pulse Server   │
│  (Isolated)     │         │                 │
│                 │  HTTPS  │                 │
│  ┌───────────┐  │ ──────> │  ┌───────────┐ │
│  │   Agent   │  │  Push   │  │ Webhook   │ │
│  │  Service  │  │ Metrics │  │ Endpoint  │ │
│  └───────────┘  │         │  └───────────┘ │
└─────────────────┘         └─────────────────┘
```

## Setup Instructions

### 1. Configure Pulse Server

First, set up the Pulse server to accept pushed metrics:

1. Set the API key environment variable:
   ```bash
   # Add to your Pulse environment configuration
   PULSE_PUSH_API_KEY=your-secure-api-key-here
   ```

2. Restart Pulse to apply the configuration:
   ```bash
   systemctl restart pulse
   ```

3. The push endpoint will be available at:
   ```
   https://your-pulse-server.com/api/push/metrics
   ```

### 2. Install the Agent on PBS Server

On each PBS server that needs to push metrics:

1. Copy the agent files to the PBS server:
   ```bash
   scp -r /opt/pulse/agent root@pbs-server:/tmp/pulse-agent
   ```

2. SSH to the PBS server and run the installation:
   ```bash
   ssh root@pbs-server
   cd /tmp/pulse-agent
   sudo ./install.sh
   ```

3. Create a PBS API token for the agent:
   ```bash
   # On the PBS server
   pvesh create /access/users/pulse@pbs/token/monitoring --privsep=0
   ```
   
   Save the generated token - you'll need it for configuration.

4. Edit the configuration file:
   ```bash
   sudo nano /etc/pulse-agent/pulse-agent.env
   ```

   Configure the following:
   ```bash
   # Required settings
   PULSE_SERVER_URL=https://your-pulse-server.com
   PULSE_API_KEY=your-api-key-here
   PBS_API_TOKEN=pulse@pbs!monitoring:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   
   # Optional settings
   PUSH_INTERVAL=30  # Push every 30 seconds
   AGENT_ID=pbs-backup-01  # Unique identifier for this PBS
   ```

5. Start the agent:
   ```bash
   sudo systemctl start pulse-agent
   sudo systemctl enable pulse-agent
   ```

6. Check the agent status:
   ```bash
   sudo systemctl status pulse-agent
   sudo journalctl -u pulse-agent -f
   ```

## Configuration Options

### Pulse Server

| Environment Variable | Description | Default | Required |
|---------------------|-------------|---------|----------|
| `PULSE_PUSH_API_KEY` | API key for authenticating push requests | - | Yes |

### PBS Agent

| Environment Variable | Description | Default | Required |
|---------------------|-------------|---------|----------|
| `PULSE_SERVER_URL` | URL of the Pulse server | - | Yes |
| `PULSE_API_KEY` | API key for authentication | - | Yes |
| `PBS_API_URL` | PBS API URL | `https://localhost:8007` | No |
| `PBS_API_TOKEN` | PBS API token | - | Yes |
| `PBS_FINGERPRINT` | PBS server certificate fingerprint | - | No |
| `PUSH_INTERVAL` | Interval between pushes (seconds) | `30` | No |
| `AGENT_ID` | Unique identifier for this PBS | hostname | No |

## API Endpoints

### Push Metrics
`POST /api/push/metrics`

Accepts PBS metrics data from agents.

**Headers:**
- `X-API-Key` or `Authorization: Bearer <key>` - Required

**Request Body:**
```json
{
  "pbsId": "pbs-backup-01",
  "nodeStatus": { /* node metrics */ },
  "datastores": [ /* datastore info */ ],
  "tasks": [ /* recent tasks */ ],
  "version": "2.4.1",
  "timestamp": 1234567890,
  "agentVersion": "1.0.0",
  "pushInterval": 30000
}
```

**Response:**
```json
{
  "success": true,
  "received": 1234567890,
  "processed": 1234567891,
  "nextExpectedBefore": 1234567925
}
```

### List Push Agents
`GET /api/push/agents`

Lists all connected push agents.

**Headers:**
- `X-API-Key` or `Authorization: Bearer <key>` - Required

**Response:**
```json
{
  "agents": [
    {
      "id": "pbs-backup-01",
      "name": "pbs-backup-01",
      "lastPushReceived": 1234567890,
      "agentVersion": "1.0.0",
      "online": true,
      "version": "2.4.1"
    }
  ]
}
```

## UI Features

Push mode PBS instances are displayed with special indicators in the Pulse UI:

- **Connection Badge**: Shows "Push Mode" with online/offline status
- **Online Status**: Green badge with pulsing indicator when receiving data
- **Offline Status**: Red badge when no data received for >2 minutes
- **Last Push Time**: Hover over the badge to see when data was last received
- **Agent Version**: Displayed under the PBS version in the server status

## Monitoring & Troubleshooting

### Agent Logs
View agent logs on the PBS server:
```bash
sudo journalctl -u pulse-agent -f
```

### Common Issues

1. **Agent can't connect to Pulse**
   - Check firewall allows outbound HTTPS
   - Verify PULSE_SERVER_URL is correct
   - Ensure API key matches

2. **Authentication failures**
   - Verify PBS_API_TOKEN is valid
   - Check PBS API is accessible locally
   - Ensure pulse user has correct permissions

3. **No metrics showing in Pulse**
   - Check agent is running: `systemctl status pulse-agent`
   - Verify push endpoint is working: `curl -H "X-API-Key: your-key" https://pulse-server/api/push/agents`
   - Look for errors in Pulse logs

### Testing the Setup

1. Test the push endpoint manually:
   ```bash
   curl -X POST https://your-pulse-server/api/push/metrics \
     -H "X-API-Key: your-api-key" \
     -H "Content-Type: application/json" \
     -d '{"pbsId":"test","nodeStatus":{},"timestamp":'$(date +%s)'000}'
   ```

2. Check agent connectivity from PBS server:
   ```bash
   curl -H "X-API-Key: your-api-key" \
     https://your-pulse-server/api/push/agents
   ```

## Security Considerations

### ⚠️ Important Security Notice

The push mode feature is designed for monitoring isolated PBS instances and includes basic security measures. However, please be aware of the following:

1. **API Key Security**
   - Create your own strong API key for `PULSE_PUSH_API_KEY` (min 32 characters)
   - This is NOT a Proxmox-generated key - you create this yourself
   - Store keys securely in environment files with restricted permissions (chmod 600)
   - Rotate keys periodically
   - The same API key is shared by all agents (consider this in your threat model)

2. **Network Security**
   - **Always use HTTPS** for all communications to prevent API key interception
   - Consider using a reverse proxy (nginx/traefik) with additional authentication
   - Limit API endpoint access by IP if possible using firewall rules
   - The push endpoint is rate-limited to 60 requests/minute per API key

3. **PBS Permissions**
   - The PBS API token only needs read permissions
   - Use token with `privsep=0` to limit scope
   - Consider creating a dedicated PBS user for monitoring

4. **Built-in Protections**
   - Rate limiting: 60 requests per minute per API key
   - Request size limit: 10MB maximum
   - Input validation: Prevents malformed data
   - Authentication logging: Failed attempts are logged

5. **Deployment Recommendations**
   - Run Pulse behind a reverse proxy with HTTPS
   - Use firewall rules to restrict access to the push endpoint
   - Monitor logs for authentication failures
   - Consider network segmentation for the monitoring infrastructure

### Generating Secure API Keys

The `PULSE_PUSH_API_KEY` is a key YOU create (not generated by Proxmox). Here's how to generate a strong one:

```bash
# Generate a secure 32-character API key
openssl rand -hex 16

# Or using /dev/urandom
cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1

# Example output: 7f3b9c4e2a1d5f8b6c9e3a7d4f2b8e5c
```

Then set this in your Pulse environment:
```bash
PULSE_PUSH_API_KEY=7f3b9c4e2a1d5f8b6c9e3a7d4f2b8e5c
```

## Limitations

- Push mode requires the agent to be installed and running on each PBS server
- Metrics are only as current as the last push (default 30 seconds)
- If the agent stops, Pulse will show the PBS as offline after 2 minutes
- Historical data depends on the agent running continuously

## Future Enhancements

Potential improvements for the push mode feature:

- Automatic agent updates
- Configurable offline timeout
- Metric buffering for network interruptions
- Compression for large metric payloads
- Multiple Pulse server targets for redundancy