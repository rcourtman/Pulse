# 🐳 Docker Monitoring Agent

Monitor Docker and Podman hosts alongside your Proxmox infrastructure.

## 🚀 Quick Start

Generate an installation command in the UI:
**Settings → Agents → Docker Agents → "Install New Agent"**

### Standard Install
```bash
curl -fsSL http://<pulse-ip>:7655/install-docker-agent.sh | \
  sudo bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```
*Creates a `pulse-docker-agent` systemd service.*

### Podman (Rootless)
```bash
curl -fsSL http://<pulse-ip>:7655/install-container-agent.sh | \
  bash -s -- --runtime podman --rootless --url http://<pulse-ip>:7655 --token <api-token>
```

---

## 📊 Features

- **Container Metrics**: CPU, Memory, Network, Disk I/O.
- **Health Checks**: Tracks container health status and restart loops.
- **Swarm Support**: Auto-detects Swarm mode and reports service/task data.
- **Multi-Target**: Can report to multiple Pulse servers for HA.

---

## ⚙️ Configuration

The agent is configured via flags or environment variables (in `/etc/pulse/pulse-docker-agent.env`).

| Flag | Env Var | Description | Default |
|------|---------|-------------|---------|
| `--url` | `PULSE_URL` | Pulse Server URL | `http://localhost:7655` |
| `--token` | `PULSE_TOKEN` | API Token (scope: `docker:report`) | *(required)* |
| `--interval` | `PULSE_INTERVAL` | Polling Interval | `30s` |
| `--runtime` | `PULSE_RUNTIME` | `docker` or `podman` | `docker` |
| `--collect-disk` | `PULSE_COLLECT_DISK` | Monitor container disk usage | `true` |

<details>
<summary><strong>Advanced: Run as Container</strong></summary>

You can run the agent as a container instead of a system service.

```bash
docker run -d \
  --name pulse-agent \
  --pid=host --uts=host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e PULSE_URL="http://pulse:7655" \
  -e PULSE_TOKEN="<token>" \
  ghcr.io/rcourtman/pulse-docker-agent:latest
```
</details>

---

## ⚠️ Troubleshooting

- **Agent Rejected?**
  If a host was previously removed, you must "Allow re-enroll" in **Settings → Docker → Removed Hosts**.

- **Permission Denied (Socket)?**
  Ensure the `pulse-docker` user is in the `docker` group (`sudo usermod -aG docker pulse-docker`).

- **Duplicate Hosts?**
  If agents flip-flop in the UI, they share a machine-id. Set a unique `--agent-id` flag.
