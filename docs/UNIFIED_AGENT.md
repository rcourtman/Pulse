# Pulse Unified Agent

The unified agent (`pulse-agent`) combines host, Docker, and Kubernetes monitoring into a single binary. It replaces the separate `pulse-host-agent` and `pulse-docker-agent` for simpler deployment and management.
Install it on each host you want Pulse to monitor. This is the primary monitoring path for infrastructure onboarding.

> Note: For temperature monitoring, use `pulse-agent --enable-proxmox` (recommended) or SSH-based collection. The legacy sensor proxy has been removed. See `docs/TEMPERATURE_MONITORING.md`.

## Quick Start

Generate an installation command in the UI:
**Settings → Unified Agents → Installation commands**

Choose a target profile in that screen when you want explicit install flags for Docker, Kubernetes, Proxmox VE, or Proxmox Backup Server.

### Linux (systemd)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

### macOS
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

### Windows (PowerShell, run as Administrator)
```powershell
irm http://<pulse-ip>:7655/install.ps1 | iex
```

With environment variables:
```powershell
$env:PULSE_URL="http://<pulse-ip>:7655"
$env:PULSE_TOKEN="<api-token>"
irm http://<pulse-ip>:7655/install.ps1 | iex
```

### Synology NAS
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

## Features

- **Host Metrics**: CPU, memory, disk, network I/O, temperatures
- **Docker Monitoring**: Container metrics, health checks, Swarm support (when enabled)
- **Kubernetes Monitoring**: Cluster, node, pod, and deployment health (when enabled)
- **Auto-Update**: Automatically updates when a new version is released
- **Multi-Platform**: Linux, macOS, Windows support

## Configuration

| Flag | Env Var | Description | Default |
|------|---------|-------------|---------|
| `--url` | `PULSE_URL` | Pulse server URL | `http://localhost:7655` |
| `--token` | `PULSE_TOKEN` | API token | *(required)* |
| `--token-file` | - | Read API token from file | *(unset)* |
| `--interval` | `PULSE_INTERVAL` | Reporting interval | `30s` |
| `--enable-host` | `PULSE_ENABLE_HOST` | Enable host metrics | `true` |
| `--enable-docker` | `PULSE_ENABLE_DOCKER` | Enable Docker metrics | `false` (auto-detect if not configured) |
| `--docker-runtime` | `PULSE_DOCKER_RUNTIME` | Force container runtime: `auto`, `docker`, or `podman` | `auto` |
| `--enable-kubernetes` | `PULSE_ENABLE_KUBERNETES` | Enable Kubernetes metrics | `false` (installer auto-detect if not configured) |
| `--enable-proxmox` | `PULSE_ENABLE_PROXMOX` | Enable Proxmox integration | `false` |
| `--proxmox-type` | `PULSE_PROXMOX_TYPE` | Proxmox type: `pve` or `pbs` | *(auto-detect)* |
| `--enable-commands` | `PULSE_ENABLE_COMMANDS` | Enable AI command execution (disabled by default) | `false` |
| `--disable-commands` | `PULSE_DISABLE_COMMANDS` | **Deprecated** (commands are disabled by default) | - |
| `--disk-exclude` | `PULSE_DISK_EXCLUDE` | Mount point patterns to exclude from disk monitoring (repeatable or CSV) | *(none)* |
| `--kubeconfig` | `PULSE_KUBECONFIG` | Kubeconfig path (optional) | *(auto)* |
| `--kube-context` | `PULSE_KUBE_CONTEXT` | Kubeconfig context (optional) | *(auto)* |
| `--kube-include-namespace` | `PULSE_KUBE_INCLUDE_NAMESPACES` | Limit namespaces (repeatable or CSV, wildcards supported) | *(all)* |
| `--kube-exclude-namespace` | `PULSE_KUBE_EXCLUDE_NAMESPACES` | Exclude namespaces (repeatable or CSV, wildcards supported) | *(none)* |
| `--kube-include-all-pods` | `PULSE_KUBE_INCLUDE_ALL_PODS` | Include all non-succeeded pods | `false` |
| `--kube-include-all-deployments` | `PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS` | Include all deployments, not just problems | `false` |
| `--kube-max-pods` | `PULSE_KUBE_MAX_PODS` | Max pods per report | `200` |
| `--disable-auto-update` | `PULSE_DISABLE_AUTO_UPDATE` | Disable auto-updates | `false` |
| `--disable-docker-update-checks` | `PULSE_DISABLE_DOCKER_UPDATE_CHECKS` | Disable Docker image update detection | `false` |
| `--insecure` | `PULSE_INSECURE_SKIP_VERIFY` | Skip TLS verification | `false` |
| `--hostname` | `PULSE_HOSTNAME` | Override hostname | *(OS hostname)* |
| `--agent-id` | `PULSE_AGENT_ID` | Unique agent identifier | *(machine-id)* |
| `--report-ip` | `PULSE_REPORT_IP` | Override reported IP (multi-NIC) | *(auto)* |
| `--disable-ceph` | `PULSE_DISABLE_CEPH` | Disable local Ceph status polling | `false` |
| `--tag` | `PULSE_TAGS` | Apply tags (repeatable or CSV) | *(none)* |
| `--log-level` | `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |
| `--health-addr` | `PULSE_HEALTH_ADDR` | Health/metrics server address | `:9191` |

**Token resolution order**: `--token` → `--token-file` → `PULSE_TOKEN` → `/var/lib/pulse-agent/token`.

### Advanced Flags

- `--version`: Print the agent version and exit.
- `--self-test`: Perform a self-test and exit (used during auto-update).

## Auto-Detection

Auto-detection behavior:

- **Host metrics**: Enabled by default.
- **Docker/Podman**: Enabled automatically by the agent if Docker/Podman is detected and `PULSE_ENABLE_DOCKER` was not explicitly set.
- **Kubernetes**: Enabled automatically by the installer when a kubeconfig is detected and `PULSE_ENABLE_KUBERNETES` was not explicitly set.
- **Proxmox**: Enabled automatically by the installer when Proxmox is detected. Type auto-detects `pve` vs `pbs` if not specified.

To disable auto-detection, explicitly set the relevant flags or env vars, for example:

- `--enable-docker=false` or `PULSE_ENABLE_DOCKER=false`
- `--enable-kubernetes=false` or `PULSE_ENABLE_KUBERNETES=false`
- `--enable-proxmox=false` or `PULSE_ENABLE_PROXMOX=false`

## Installation Options

### Simple Install (host + Docker auto-detect)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token>
```

### Proxmox VE Node (explicit profile)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-proxmox --proxmox-type pve
```

### Proxmox Backup Server Node (explicit profile)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-proxmox --proxmox-type pbs
```

### Force Enable Docker (if auto-detection fails)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-docker
```

### Disable Docker (even if detected)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-docker=false
```

### Host + Kubernetes Monitoring
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-kubernetes
```

### Docker Monitoring Only
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-host=false --enable-docker
```

### Exclude Specific Disks from Monitoring
```bash
# Exclude specific mount points
pulse-agent --disk-exclude /mnt/backup --disk-exclude /media/external

# Exclude using patterns (prefix match)
pulse-agent --disk-exclude '/mnt/pbs*'  # Matches /mnt/pbs-data, /mnt/pbs-backup, etc.

# Exclude using patterns (contains match)
pulse-agent --disk-exclude '*pbs*'  # Matches any path containing 'pbs'

# Via environment variable (comma-separated)
PULSE_DISK_EXCLUDE=/mnt/backup,*pbs*,/media/external
```

**Pattern types:**
- Exact: `/mnt/backup` - matches only that exact path
- Prefix: `/mnt/ext*` - matches paths starting with `/mnt/ext`
- Contains: `*pbs*` - matches paths containing `pbs`

## S.M.A.R.T. Disk Temperatures

The agent can report S.M.A.R.T. disk temperatures when running in Agent mode. This requires:

1. **smartmontools** installed on the host:
   ```bash
   # Debian/Ubuntu
   apt install smartmontools

   # RHEL/CentOS
   yum install smartmontools

   # Alpine
   apk add smartmontools
   ```

2. The agent must have permission to run `smartctl` (typically requires root)

**Notes:**
- Disks in standby mode are reported as such (no temperature) to avoid waking them
- S.M.A.R.T. data is collected alongside other host metrics
- If `smartctl` is not available, S.M.A.R.T. monitoring is silently skipped
- **Disk exclusions** (`--disk-exclude` / `PULSE_DISK_EXCLUDE`) also apply to S.M.A.R.T. monitoring.
  Use patterns like `sda`, `/dev/sdb`, `nvme*`, or `*cache*` to exclude specific block devices.

## Auto-Update

The unified agent automatically checks for updates every hour. When a new version is available:

1. Agent downloads the new binary from the Pulse server
2. Verifies the checksum
3. Replaces itself atomically (with backup)
4. Restarts with the same configuration

To disable auto-updates:
```bash
# During installation
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --disable-auto-update

# Or set environment variable
PULSE_DISABLE_AUTO_UPDATE=true
```

## Remote Configuration (Agent Profiles, Pro/Cloud)

Pro and Cloud can push centralized settings to agents via Agent Profiles.

Behavior:
- The agent fetches remote config on startup from `/api/agents/host/{agent_id}/config`.
- Profile settings override local flags/env for supported keys.
- Profile changes take effect on the next agent restart.
- Command execution (`commandsEnabled`) is controlled per agent in **Settings → Unified Agents** and can change live.
- Remote config responses can be signed with `PULSE_AGENT_CONFIG_SIGNING_KEY` (base64 Ed25519 private key).
- To require signed payloads, set `PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true` on Pulse and agents.
- If you use a custom signing key, set `PULSE_AGENT_CONFIG_PUBLIC_KEYS` on agents to trust the matching public key.

See [Centralized Agent Management](CENTRALIZED_MANAGEMENT.md) for supported keys and profile setup.

## Uninstall

```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | bash -s -- --uninstall
```

This removes:
- The agent binary
- The systemd/launchd service
- Any legacy agents (pulse-host-agent, pulse-docker-agent)

## Migration from Legacy Agents

The install script automatically removes legacy agents when installing the unified agent:
- `pulse-host-agent` service is stopped and removed
- `pulse-docker-agent` service is stopped and removed
- Binaries are deleted from `/usr/local/bin/`

No manual cleanup is required.

## Health Checks & Metrics

The agent exposes HTTP endpoints for health checks and Prometheus metrics on port 9191 by default.

### Endpoints

| Endpoint | Description |
|----------|-------------|
| `/healthz` | Liveness probe - returns 200 if agent is running |
| `/readyz` | Readiness probe - returns 200 when agents are initialized |
| `/metrics` | Prometheus metrics |

### Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `pulse_agent_info` | Gauge | Agent info with version, host_enabled, docker_enabled labels |
| `pulse_agent_up` | Gauge | 1 when running, 0 when shutting down |

### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 9191
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 9191
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Disable Health Server

Set `--health-addr=""` or `PULSE_HEALTH_ADDR=""` to disable the health/metrics server.

## Troubleshooting

### Agent Not Updating
- Check logs: `journalctl -u pulse-agent -f`
- Verify network connectivity to Pulse server
- Ensure auto-update is not disabled

### Duplicate Hosts
If cloned VMs appear as the same host:
```bash
sudo rm /etc/machine-id && sudo systemd-machine-id-setup
```

Or set a unique agent ID:
```bash
--agent-id my-unique-host-id
```

### Permission Denied (Docker)
Ensure the agent can access the Docker socket:
```bash
sudo usermod -aG docker $USER
```

### Check Status
```bash
# Linux
systemctl status pulse-agent

# macOS
launchctl list | grep pulse
```

### Docker Swarm Not Detected

If your Docker Swarm cluster isn't being detected:

1. **Check runtime detection**: Pulse disables Swarm for Podman. Look for "Podman runtime detected" in logs:
   ```bash
   journalctl -u pulse-agent | grep -i podman
   ```

2. **Force Docker runtime**: If auto-detection is incorrect:
   ```bash
   --docker-runtime docker
   # Or set environment variable
   PULSE_DOCKER_RUNTIME=docker
   ```

3. **Check Docker info**: Verify Swarm is active on the host:
   ```bash
   docker info | grep -i swarm
   # Should show "Swarm: active"
   ```

4. **Check socket permissions**: The agent needs access to the Docker socket:
   ```bash
   ls -la /var/run/docker.sock
   ```

5. **Enable debug logging**: For more detail:
   ```bash
   LOG_LEVEL=debug journalctl -u pulse-agent -f
   ```

### PVE Backups Not Showing

If local PVE backups aren't appearing in Pulse after setting up via `--enable-proxmox`:

1. **Check permissions**: The API token needs `PVEDatastoreAdmin` on `/storage`:
   ```bash
   pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin
   ```

2. **Re-run setup** (v5.1.x or later): Delete the node in Pulse Settings and re-run the agent with `--enable-proxmox`. Newer versions grant this permission automatically.

3. **Check state file**: If re-running doesn't trigger setup, remove the state file:
   ```bash
   rm /var/lib/pulse-agent/proxmox-pve-registered
   ```
   Then restart the agent.
