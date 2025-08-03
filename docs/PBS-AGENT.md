# PBS Agent Guide

The PBS Agent allows isolated or firewalled Proxmox Backup Servers to push their data to Pulse, eliminating the need for inbound connections.

## When to Use PBS Agent

Use the PBS Agent when:
- Your PBS server is behind a firewall with no inbound access
- PBS is on an isolated network segment
- You want to monitor PBS without opening firewall ports
- Corporate security policies prevent inbound connections

## How It Works

```
┌─────────────┐           ┌─────────────┐
│     PBS     │  Push →   │    Pulse    │
│   (Agent)   │ --------> │   Server    │
│  Port: 8007 │           │ Port: 3000  │
└─────────────┘           └─────────────┘
```

The agent runs on your PBS server and:
1. Collects metrics locally via PBS API
2. Pushes data to your Pulse server
3. Handles connection failures gracefully
4. Automatically retries on network issues

## Installation

### Quick Install

On your PBS server:

```bash
# Download and extract
cd /opt
wget https://github.com/rcourtman/Pulse/releases/latest/download/pulse-pbs-agent.tar.gz
tar xzf pulse-pbs-agent.tar.gz
cd pulse-pbs-agent

# Run installer
sudo ./install.sh
```

### Manual Installation

1. **Download the agent binary**:
```bash
wget https://github.com/rcourtman/Pulse/releases/latest/download/pulse-pbs-agent
chmod +x pulse-pbs-agent
sudo mv pulse-pbs-agent /usr/local/bin/
```

2. **Create configuration**:
```bash
sudo mkdir -p /etc/pulse-agent
sudo nano /etc/pulse-agent/config.yml
```

3. **Add configuration**:
```yaml
# Pulse server details
pulse:
  url: http://your-pulse-server:3000
  token: your-agent-token  # Generated in Pulse UI

# PBS connection
pbs:
  host: https://localhost:8007
  user: monitor@pbs
  password: your-password
  # Or use API token:
  # tokenName: monitor
  # tokenValue: secret-token
  verifySSL: false

# Agent settings
agent:
  interval: 30  # Seconds between updates
  name: pbs-prod  # Unique name for this PBS
```

4. **Create systemd service**:
```bash
sudo nano /etc/systemd/system/pulse-pbs-agent.service
```

```ini
[Unit]
Description=Pulse PBS Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse-pbs-agent
Restart=always
RestartSec=10
User=pulse-agent
Group=pulse-agent
WorkingDirectory=/etc/pulse-agent

[Install]
WantedBy=multi-user.target
```

5. **Start the service**:
```bash
sudo useradd -r -s /bin/false pulse-agent
sudo chown -R pulse-agent:pulse-agent /etc/pulse-agent
sudo chmod 600 /etc/pulse-agent/config.yml
sudo systemctl daemon-reload
sudo systemctl enable --now pulse-pbs-agent
```

## Configuration

### Pulse Server Setup

1. In Pulse UI, go to **Settings** → **PBS Agents**
2. Click **Add Agent**
3. Enter a name and generate a token
4. Copy the token for agent configuration

### Agent Configuration Options

```yaml
pulse:
  url: http://pulse.example.com:3000  # Your Pulse server
  token: abc123...                     # Agent auth token
  timeout: 30                          # Connection timeout (seconds)
  retryInterval: 60                    # Retry interval on failure

pbs:
  host: https://localhost:8007         # PBS API endpoint
  user: monitor@pbs                    # PBS user
  password: secret                     # PBS password
  fingerprint: "AA:BB:CC..."          # Optional TLS fingerprint
  verifySSL: true                     # Verify SSL certificates

agent:
  interval: 30                        # Update interval (seconds)
  name: pbs-prod                      # Unique agent name
  logLevel: info                      # Log verbosity (debug|info|warn|error)
```

### Environment Variables

All settings can be overridden with environment variables:

```bash
PULSE_URL=http://pulse:3000
PULSE_TOKEN=your-token
PBS_HOST=https://localhost:8007
PBS_USER=monitor@pbs
PBS_PASSWORD=secret
AGENT_INTERVAL=60
AGENT_NAME=pbs-backup
```

## Monitoring Multiple PBS Servers

To monitor multiple PBS servers with agents:

1. Install the agent on each PBS server
2. Use unique agent names for each
3. Generate separate tokens in Pulse UI
4. Each agent pushes to the same Pulse server

## Troubleshooting

### Check Agent Status
```bash
sudo systemctl status pulse-pbs-agent
sudo journalctl -u pulse-pbs-agent -f
```

### Common Issues

**Agent can't connect to Pulse**:
- Verify Pulse server URL is correct
- Check network connectivity: `curl http://pulse-server:3000/api/health`
- Ensure firewall allows outbound connections
- Verify agent token is valid

**Agent can't connect to PBS**:
- Check PBS API is accessible: `curl -k https://localhost:8007`
- Verify PBS credentials
- Ensure PBS user has DatastoreReader permission
- Check SSL settings if using self-signed certificates

**High CPU usage**:
- Increase the update interval
- Check PBS API performance
- Review agent logs for errors

**Data not appearing in Pulse**:
- Verify agent name matches configuration
- Check Pulse logs for incoming data
- Ensure agent token has correct permissions
- Look for errors in agent logs

### Debug Mode

Run the agent in debug mode:
```bash
pulse-pbs-agent -debug
```

Or set in config:
```yaml
agent:
  logLevel: debug
```

## Security Considerations

1. **Use API tokens** instead of passwords when possible
2. **Secure the config file**: `chmod 600 /etc/pulse-agent/config.yml`
3. **Use HTTPS** for Pulse server connections in production
4. **Rotate tokens** periodically
5. **Monitor agent logs** for suspicious activity

## Uninstall

```bash
# Stop and disable service
sudo systemctl stop pulse-pbs-agent
sudo systemctl disable pulse-pbs-agent

# Remove files
sudo rm /usr/local/bin/pulse-pbs-agent
sudo rm -rf /etc/pulse-agent
sudo rm /etc/systemd/system/pulse-pbs-agent.service
sudo systemctl daemon-reload

# Remove user
sudo userdel pulse-agent
```