# Pulse Host Agent

The Pulse host agent extends monitoring to standalone servers that do not expose
Proxmox or Docker APIs. With it you can surface uptime, OS metadata, CPU load,
memory/disk utilisation, and connection health for any Linux, macOS, or Windows
machine alongside the rest of your infrastructure. Starting in v4.26.0 the
installer handshakes with Pulse in real time so you can confirm registration
from the UI and receive host-agent alerts alongside your existing
Docker/Proxmox notifications.

## Prerequisites

- Pulse v4.26.0 or newer (host agent reporting shipped with `/api/agents/host/report`)
- An API token with the `host-agent:report` scope (create under **Settings → Security**)
- Outbound HTTP/HTTPS connectivity from the host back to Pulse

> ℹ️ The agent only initiates outbound connections; no inbound firewall rules are required.

If your Pulse instance does not require API tokens (e.g. during an on-premises
lab install) you can still generate commands without embedding a credential.
Confirm the warning in **Settings → Agents → Host agents** and the script will
prompt for a token instead of hard-coding one.

## Quick Start

> Replace `<api-token>` with a Pulse API token limited to the `host-agent:report` scope. Tokens generated from **Settings → Agents → Host agents** already apply this scope.

### Linux

The hosted installer handles systemd, rc.local environments, and Unraid automatically.

```bash
curl -fsSL http://pulse.example.local:7655/install-host-agent.sh | \
  bash -s -- --url http://pulse.example.local:7655 --token <api-token>
```

- On systemd machines the script installs the binary, wires up `/etc/systemd/system/pulse-host-agent.service`, enables it, and tails the registration status.
- On Unraid hosts it starts the agent under `nohup`, creates `/var/log/pulse`, and (optionally) inserts the auto-start line into `/boot/config/go`.
- On minimalist distros without systemd (e.g. Alpine) it creates/updates `/etc/rc.local`, adds the background runner, and verifies it launches.

Use `--force` to skip interactive prompts or `--interval 1m` to change the polling cadence.

### macOS

```bash
curl -fsSL http://pulse.example.local:7655/install-host-agent.sh | \
  bash -s -- --url http://pulse.example.local:7655 --token <api-token>
```

On macOS the installer stores the token in the Keychain when possible, generates a launchd plist inside `~/Library/LaunchAgents`, and restarts the job so the agent survives logouts and reboots.

### Windows

Run the PowerShell bootstrapper as an administrator:

```powershell
irm http://pulse.example.local:7655/install-host-agent.ps1 | iex
```

Set `PULSE_URL` and `PULSE_TOKEN` in the environment first for a non-interactive flow:

```powershell
$env:PULSE_URL    = "http://pulse.example.local:7655"
$env:PULSE_TOKEN  = "<api-token>"
irm http://pulse.example.local:7655/install-host-agent.ps1 | iex
```

The script installs the service under `PulseHostAgent`, registers Windows Event Log messages, configures automatic recovery on failure, and waits for Pulse to acknowledge the new host.

### Manual installation (advanced)

Prefer to take full control or working in air-gapped environments? You can still download the static binaries and wire them up manually. The commands below mirror what the installer scripts perform for their respective platforms.

#### Linux (systemd)

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

#### macOS (launchd)

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

#### Windows (manual)

Compile from source (`GOOS=windows GOARCH=amd64`) or download the latest release, then install the Windows service yourself:

```powershell
New-Service -Name PulseHostAgent `
  -BinaryPathName '"C:\Program Files\Pulse\pulse-host-agent.exe" --url http://pulse.example.local:7655 --token <api-token> --interval 30s' `
  -DisplayName "Pulse Host Agent" `
  -Description "Monitors system metrics and reports to Pulse monitoring server" `
  -StartupType Automatic
Start-Service -Name PulseHostAgent
```

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

### Checking installation status

- Click **Check status** under **Settings → Agents → Host agents** and enter the host ID or hostname you just installed.
- Pulse hits `/api/agents/host/lookup`, highlights the matching row for 10 seconds, and refreshes the connection badge, last-seen timestamp, and agent version in-line.
- If the host has not checked in yet, the UI returns a friendly "Host has not registered" message so you can retry without re-running the script.

### Alerts and notifications

- Host agents now participate in the main alert engine. Offline detection, metric thresholds, and override scopes (global or per-host) live in **Settings → Alerts → Thresholds** beside your Docker and Proxmox rules.
- Alert notifications, webhooks, and quiet-hours behaviour reuse the existing pipelines—no extra setup is required once you enable host-agent monitoring.

## Updating

Since the agent is a single static binary, updates are as simple as replacing the file and restarting your launchd/systemd unit. The Settings pane always links to the latest release artefacts.
