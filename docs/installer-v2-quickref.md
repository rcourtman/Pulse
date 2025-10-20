# Installer v2 Quick Reference

## Opt-In / Opt-Out

```bash
# Use the new installer
export PULSE_INSTALLER_V2=1
curl -fsSL https://download.pulse.example/install-docker-agent.sh | bash -s -- [flags]

# Force the legacy installer
export PULSE_INSTALLER_V2=0
curl -fsSL https://download.pulse.example/install-docker-agent.sh | bash -s -- [flags]
```

## Common Flags

- `--url <https://pulse.example>` — Primary Pulse server URL
- `--token <api-token>` — API token for enrollment
- `--target <url|token[|insecure]>` — Additional targets (repeatable)
- `--interval <duration>` — Poll interval (default `30s`)
- `--dry-run` — Show actions without applying changes
- `--uninstall` — Remove agent binary, systemd unit, and startup hooks

## Examples

```bash
# Preview installation without changes
export PULSE_INSTALLER_V2=1
curl -fsSL https://download.pulse.example/install-docker-agent.sh | bash -s -- \
  --dry-run \
  --url https://pulse.example \
  --token <api-token>

# Install with two targets and custom interval
export PULSE_INSTALLER_V2=1
curl -fsSL https://download.pulse.example/install-docker-agent.sh | bash -s -- \
  --url https://pulse-primary \
  --token <api-token> \
  --target https://pulse-dr|<dr-token> \
  --target https://pulse-edge|<edge-token>|true \
  --interval 15s

# Uninstall
curl -fsSL https://download.pulse.example/install-docker-agent.sh | bash -s -- --uninstall
```

## Verification

- Binary path: `/usr/local/bin/pulse-docker-agent`
- Systemd unit: `/etc/systemd/system/pulse-docker-agent.service`
- Logs: `journalctl -u pulse-docker-agent -f`

## Rollback

```bash
# Force legacy installer
export PULSE_INSTALLER_V2=0
curl -fsSL https://download.pulse.example/install-docker-agent.sh | bash -s -- ...
```

Contact support or the Pulse engineering team if issues arise during rollout.
