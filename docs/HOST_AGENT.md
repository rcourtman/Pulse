# 🖥️ Host Agent

Monitor standalone Linux, macOS, and Windows servers that don't run Proxmox or Docker.

## 🚀 Quick Start

Generate an installation command in the UI:
**Settings → Agents → Host Agents → "Install New Agent"**

### Linux (Universal)
```bash
curl -fsSL http://<pulse-ip>:7655/install-host-agent.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```
*Supports systemd, OpenRC, and Unraid.*

### macOS
```bash
curl -fsSL http://<pulse-ip>:7655/install-host-agent.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```
*Installs as a LaunchAgent.*

### Windows (PowerShell)
```powershell
$env:PULSE_URL = "http://<pulse-ip>:7655"
$env:PULSE_TOKEN = "<api-token>"
irm http://<pulse-ip>:7655/install-host-agent.ps1 | iex
```
*Installs as a Windows Service.*

---

## 📊 Features

- **System Metrics**: CPU, Memory, Disk, Network I/O.
- **Temperature**: Auto-detects sensors via `lm-sensors` (Linux).
- **RAID Monitoring**: Auto-detects `mdadm` arrays (Linux).
- **Smart Alerts**: Integrated with Pulse's alerting engine.

---

## ⚙️ Configuration

The agent is a single binary configured via flags.

| Flag | Description | Default |
|------|-------------|---------|
| `--url` | Pulse Server URL | `http://localhost:7655` |
| `--token` | API Token (scope: `host-agent:report`) | *(required)* |
| `--interval` | Polling Interval | `30s` |
| `--hostname` | Override Hostname | *(OS hostname)* |
| `--agent-id` | Unique Agent ID | *(machine-id)* |

<details>
<summary><strong>Advanced: Manual Installation</strong></summary>

Download the binary from [Releases](https://github.com/rcourtman/Pulse/releases) and run it manually:

```bash
# Linux / macOS
sudo ./pulse-host-agent --url http://pulse:7655 --token <token> --interval 30s

# Windows
.\pulse-host-agent.exe --url http://pulse:7655 --token <token>
```

To run as a service, create a systemd unit or use `sc.exe` on Windows.
</details>

---

## ⚠️ Troubleshooting

- **Duplicate Hosts?**
  If cloned VMs show up as the same host, they likely share a `/etc/machine-id`.
  Run `sudo rm /etc/machine-id && sudo systemd-machine-id-setup` to fix.

- **No Temperature Data?**
  Ensure `lm-sensors` is installed and configured (`sudo sensors-detect`).

- **Check Status**
  Go to **Settings → Agents → Host Agents** to verify connection status.
