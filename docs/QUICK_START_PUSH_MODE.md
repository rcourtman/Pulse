# Quick Start: PBS Push Mode

Enable monitoring of isolated PBS servers in 3 steps:

## 1. Enable Push Mode on Pulse

### Option A: Via Settings UI (Recommended)

1. Open Pulse Settings (⚙️ gear icon)
2. Go to the **PBS** tab
3. In the **PBS Push Mode Settings** section:
   - Click **Generate** to create a secure API key
   - Or enter your own secure key
4. Click **Save Changes**

### Option B: Manual Configuration

If using Docker or need to set via environment variables:

```bash
PULSE_PUSH_API_KEY=<generate-a-secure-key-here>
```

Then restart Pulse if manually configured.

## 2. Install Agent on PBS Server

On each isolated PBS server, run:

```bash
# Download and run installer
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse-agent.sh | bash

# Create PBS API token
pvesh create /access/users/pulse@pbs/token/monitoring --privsep=0

# Edit configuration
nano /etc/pulse-agent/pulse-agent.env
```

Configure these required settings:
- `PULSE_SERVER_URL` - Your Pulse server URL
- `PULSE_API_KEY` - The key you set in step 1
- `PBS_API_TOKEN` - The token from pvesh command above

## 3. Start the Agent

```bash
systemctl start pulse-agent
systemctl enable pulse-agent
```

## Verify It's Working

1. Check agent logs: `journalctl -u pulse-agent -f`
2. In Pulse UI, look for the "Push Mode (Online)" badge on your PBS server
3. Metrics should appear within 30 seconds

## Troubleshooting

- **Agent can't connect**: Check firewall allows outbound HTTPS
- **Authentication failed**: Verify API keys match exactly
- **No metrics**: Check PBS API token has correct permissions

For detailed documentation, see [PBS_PUSH_MODE.md](PBS_PUSH_MODE.md)