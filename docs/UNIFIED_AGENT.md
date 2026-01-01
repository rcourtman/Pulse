# Pulse Unified Agent

The unified agent (`pulse-agent`) combines host, Docker, and Kubernetes monitoring into a single binary. It replaces the separate `pulse-host-agent` and `pulse-docker-agent` for simpler deployment and management.

> Note: In v5, temperature monitoring should be done via `pulse-agent --enable-proxmox`. `pulse-sensor-proxy` is deprecated and retained only for existing installs during the migration window.

## Quick Start

Generate an installation command in the UI:
**Settings > Agents > "Install New Agent"**

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
| `--interval` | `PULSE_INTERVAL` | Reporting interval | `30s` |
| `--enable-host` | `PULSE_ENABLE_HOST` | Enable host metrics | `true` |
| `--enable-docker` | `PULSE_ENABLE_DOCKER` | Force enable Docker metrics | **auto-detect** |
| `--disable-docker` | - | Disable Docker even if detected | - |
| `--docker-runtime` | `PULSE_DOCKER_RUNTIME` | Force container runtime: `auto`, `docker`, or `podman` | `auto` |

| `--enable-kubernetes` | `PULSE_ENABLE_KUBERNETES` | Force enable Kubernetes metrics | **auto-detect** |
| `--disable-kubernetes` | - | Disable Kubernetes even if detected | - |
| `--enable-proxmox` | `PULSE_ENABLE_PROXMOX` | Force enable Proxmox integration | **auto-detect** |
| `--disable-proxmox` | - | Disable Proxmox even if detected | - |
| `--enable-commands` | `PULSE_ENABLE_COMMANDS` | Enable AI command execution (disabled by default) | `false` |
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
| `--health-addr` | `PULSE_HEALTH_ADDR` | Health/metrics server address | `:9191` |


## Auto-Detection

The installer automatically detects available platforms on the target machine:

- **Docker/Podman**: Enabled if `docker info` or `podman info` succeeds
- **Kubernetes**: Enabled if `kubectl cluster-info` succeeds or kubeconfig exists
- **Proxmox**: Enabled if `/etc/pve` or `/etc/proxmox-backup` exists

Use `--disable-*` flags to skip auto-detected platforms, or `--enable-*` to force enable.

## Installation Options

### Simple Install (auto-detects everything)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token>
```

### Force Enable Docker (if auto-detection fails)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-docker
```

### Disable Docker (even if detected)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --disable-docker
```

### Host + Kubernetes Monitoring
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-kubernetes
```

### Docker Monitoring Only
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --disable-host --enable-docker
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
