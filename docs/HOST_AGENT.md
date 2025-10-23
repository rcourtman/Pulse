# Pulse Host Agent

The Pulse host agent extends monitoring to standalone servers that do not expose
Proxmox or Docker APIs. With it you can surface uptime, OS metadata, CPU load,
memory/disk utilisation, and connection health for any Linux, macOS, or Windows
machine alongside the rest of your infrastructure.

## Prerequisites

- Pulse `main` (or a release that includes `/api/agents/host/report`)
- An API token with the `host-agent:report` scope (create under **Settings → Security**)
- Outbound HTTP/HTTPS connectivity from the host back to Pulse

> ℹ️ The agent only initiates outbound connections; no inbound firewall rules are required.

## Quick Start

> Replace `<api-token>` with a Pulse API token limited to the `host-agent:report` scope. Tokens generated from **Settings → Agents → Host agents** already apply this scope.

### Linux (systemd)

```bash
sudo curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/pulse-host-agent-linux-amd64 \
  -o /usr/local/bin/pulse-host-agent
sudo chmod +x /usr/local/bin/pulse-host-agent
sudo /usr/local/bin/pulse-host-agent \
  --url http://pulse.example.local:7655 \
  --token <api-token> \
  --interval 30s
```

For persistence, drop a systemd unit (e.g. `/etc/systemd/system/pulse-host-agent.service`) referencing the same command and enable it with `systemctl enable --now pulse-host-agent`.

### macOS (launchd)

```bash
sudo curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/pulse-host-agent-darwin-arm64 \
  -o /usr/local/bin/pulse-host-agent
sudo chmod +x /usr/local/bin/pulse-host-agent
sudo /usr/local/bin/pulse-host-agent \
  --url http://pulse.example.local:7655 \
  --token <api-token> \
  --interval 30s
```

Create `~/Library/LaunchAgents/com.pulse.host-agent.plist` to keep the agent running between logins:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>com.pulse.host-agent</string>
    <key>ProgramArguments</key>
    <array>
      <string>/usr/local/bin/pulse-host-agent</string>
      <string>--url</string>
      <string>http://pulse.example.local:7655</string>
      <string>--token</string>
      <string>&lt;api-token&gt;</string>
      <string>--interval</string>
      <string>30s</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>/Users/your-user/Library/Logs/pulse-host-agent.log</string>
    <key>StandardErrorPath</key><string>/Users/your-user/Library/Logs/pulse-host-agent.log</string>
  </dict>
</plist>
```

Load it with `launchctl load ~/Library/LaunchAgents/com.pulse.host-agent.plist`.

### Windows

A Windows build will ship shortly. In the meantime run the Linux/WSL binary or compile from source (`GOOS=windows GOARCH=amd64`).

## Command Flags

| Flag | Description |
|------|-------------|
| `--url` | Pulse base URL (defaults to `http://localhost:7655`) |
| `--token` | API token with the `host-agent:report` scope |
| `--interval` | Polling interval (`30s` default) |
| `--hostname` | Override reported hostname |
| `--agent-id` | Override agent identifier (used as dedupe key) |
| `--tag` | Optional tag(s) to annotate the host (repeatable) |
| `--insecure` | Skip TLS verification (development/testing only) |
| `--once` | Send a single report and exit |

Run `pulse-host-agent --help` for the full list.

## Viewing Hosts

- **Settings → Agents → Host agents** lists every reporting host and provides ready-made install commands.
- The **Servers** tab surfaces host telemetry alongside Proxmox/Docker data in the main dashboard.

## Updating

Since the agent is a single static binary, updates are as simple as replacing the file and restarting your launchd/systemd unit. The Settings pane always links to the latest release artefacts.
