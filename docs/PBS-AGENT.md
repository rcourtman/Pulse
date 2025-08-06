# PBS Agent (Push Mode)

For PBS servers behind firewalls or in isolated networks that can't be reached by Pulse directly.

## Quick Install

On your PBS server:
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install-pbs-agent.sh | sudo bash
```

## Configuration

Edit `/etc/pulse-pbs-agent/config.json`:
```json
{
  "pulse_url": "http://your-pulse-server:7655",
  "api_key": "your-api-key-from-pulse-settings",
  "poll_interval": 30,
  "pbs_url": "https://localhost:8007",
  "pbs_username": "apiuser@pbs",
  "pbs_token_name": "pulse",
  "pbs_token_value": "uuid-from-pbs"
}
```

## Start Agent
```bash
sudo systemctl enable --now pulse-pbs-agent
sudo systemctl status pulse-pbs-agent
```

## Get API Key

In Pulse web UI: Settings → General → API Key → Generate

## Create PBS Token

In PBS web UI:
1. Configuration → Access Control → API Tokens
2. Add Token
3. User: Choose user with Datastore.Audit permission
4. Copy token ID (username@pbs!tokenname) and secret

## Verify

Check agent logs:
```bash
journalctl -u pulse-pbs-agent -f
```

In Pulse UI, PBS data should appear within 1 minute.

## Uninstall
```bash
sudo systemctl stop pulse-pbs-agent
sudo systemctl disable pulse-pbs-agent
sudo rm -rf /etc/pulse-pbs-agent
sudo rm /usr/local/bin/pulse-pbs-agent
sudo rm /etc/systemd/system/pulse-pbs-agent.service
```