# 📦 Installation Guide

Pulse offers flexible installation options from Docker to enterprise-ready Kubernetes charts.

## 🚀 Quick Start (Recommended)

### Proxmox VE (LXC installer)
If you run Proxmox VE, the easiest and most “Pulse-native” deployment is the official installer which creates and configures a lightweight LXC container.

Run this on your Proxmox host:

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

> **Note**: The GitHub `install.sh` is the **server** installer. The agent installer is served from your Pulse server at `/install.sh` (see **Settings → Agents → Installation commands**).

### Docker
Ideal for containerized environments or testing.

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### Docker Compose
Create a `docker-compose.yml` file:

```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - PULSE_AUTH_USER=admin
      - PULSE_AUTH_PASS=secret123

volumes:
  pulse_data:
```

> **Note**: Plain text passwords set via `PULSE_AUTH_PASS` are auto-hashed on startup. For production, prefer Quick Security Setup or a pre-hashed bcrypt value.
> **Note**: Docker monitoring requires the unified agent on the Docker host with socket access; the Pulse server container does not need `/var/run/docker.sock`. See [UNIFIED_AGENT.md](UNIFIED_AGENT.md).

---

## 🛠️ Installation Methods

### 1. Kubernetes (Helm)
Deploy to your cluster using our Helm chart.

```bash
helm repo add pulse https://rcourtman.github.io/Pulse
helm repo update
helm upgrade --install pulse pulse/pulse \
  --namespace pulse \
  --create-namespace
```
See [KUBERNETES.md](KUBERNETES.md) for ingress and persistence configuration.

### 2. Bare Metal / Systemd
For Linux servers (VM or bare metal), use the official installer:

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | sudo bash
```

> **Note**: This installs the Pulse server. Use the `/install.sh` endpoint from your Pulse UI for installing `pulse-agent` on monitored hosts.

<details>
<summary><strong>Manual systemd install (advanced)</strong></summary>

```bash
# Download the correct tarball from GitHub Releases and extract it
# https://github.com/rcourtman/Pulse/releases

sudo install -m 0755 pulse /usr/local/bin/pulse

# Create systemd service
sudo tee /etc/systemd/system/pulse.service > /dev/null << 'EOF'
[Unit]
Description=Pulse Monitoring
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulse
Restart=always
RestartSec=10
Environment=PULSE_DATA_DIR=/etc/pulse

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo mkdir -p /etc/pulse
sudo systemctl daemon-reload
sudo systemctl enable --now pulse
```
</details>

---

## 🔐 First-Time Setup

Pulse is secure by default. On first launch, you must retrieve a **Bootstrap Token** to create your admin account.

### Step 1: Get the Token

| Platform | Command |
|----------|---------|
| **Docker** | `docker exec pulse cat /data/.bootstrap_token` or `docker exec pulse /app/pulse bootstrap-token` |
| **Kubernetes** | `kubectl exec -it <pod> -- cat /data/.bootstrap_token` or `kubectl exec -it <pod> -- /app/pulse bootstrap-token` |
| **Systemd** | `sudo cat /etc/pulse/.bootstrap_token` or `sudo pulse bootstrap-token` |

### Step 2: Create Admin Account
1. Open `http://<your-ip>:7655`
2. Paste the **Bootstrap Token**.
3. Complete the **Quick Security Setup** wizard.
   - Set your **Admin Username** and **Password** (or let Pulse generate one).
   - Pulse generates an **API token** for agents and automations.
   - Copy the credentials before leaving the page.

> **Note**: If you configure authentication via environment variables (`PULSE_AUTH_USER`/`PULSE_AUTH_PASS` and/or legacy `API_TOKENS`), the bootstrap token is automatically removed and this step is skipped.

---

## 🔄 Updates

### Automatic Updates (Systemd/LXC only)
Pulse can self-update to the latest stable version.

**Enable via UI**: Settings → System → Updates

### Manual Update

| Platform | Command |
|----------|---------|
| **Docker** | `docker pull rcourtman/pulse:latest && docker restart pulse` |
| **Kubernetes** | `helm repo update && helm upgrade pulse pulse/pulse -n pulse` |
| **Systemd** | Re-download binary and restart service |

### Rollback
If an update causes issues on systemd installations, backups are created automatically during the update process.

**Manual rollback**: In-app updates store backups under `/etc/pulse/backup-<timestamp>/`. The systemd auto-update timer uses a temporary `/tmp/pulse-backup-<timestamp>` during the update and auto-restores on failure.

---

## 🗑️ Uninstall

**Docker**:
```bash
docker rm -f pulse && docker volume rm pulse_data
```

**Kubernetes**:
```bash
helm uninstall pulse -n pulse
```

**Systemd**:
```bash
sudo systemctl disable --now pulse
sudo rm -rf /etc/pulse /etc/systemd/system/pulse.service /usr/local/bin/pulse
sudo systemctl daemon-reload
```
