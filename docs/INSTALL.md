# 📦 Installation Guide

Pulse offers flexible installation options ranging from a simple one-liner for Proxmox to enterprise-ready Kubernetes charts.

## 🚀 Quick Start (Recommended)

### Proxmox VE (LXC)
The easiest way to run Pulse on Proxmox. This script creates a lightweight LXC container, configures networking, and starts the service.

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

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
      - /var/run/docker.sock:/var/run/docker.sock # Optional: Monitor local Docker
    environment:
      - PULSE_AUTH_USER=admin
      - PULSE_AUTH_PASS=secret123

volumes:
  pulse_data:
```

---

## 🛠️ Installation Methods

### 1. Proxmox LXC (Advanced)
The installer supports advanced flags for automation or custom setups.

```bash
# Install specific version
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash -s -- --version v4.24.0

# Install from source (dev branch)
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash -s -- --source develop
```

### 2. Kubernetes (Helm)
Deploy to your cluster using our Helm chart.

```bash
helm registry login ghcr.io
helm install pulse oci://ghcr.io/rcourtman/pulse-chart \
  --version $(curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/VERSION) \
  --namespace pulse \
  --create-namespace
```
See [KUBERNETES.md](KUBERNETES.md) for ingress and persistence configuration.

### 3. Manual / Systemd
For bare-metal Linux servers (Debian/Ubuntu).

```bash
# The installer detects non-Proxmox systems and installs as a systemd service
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

---

## 🔐 First-Time Setup

Pulse is secure by default. On first launch, you must retrieve a **Bootstrap Token** to create your admin account.

### Step 1: Get the Token

| Platform | Command |
|----------|---------|
| **Proxmox LXC** | `pct exec <ID> -- cat /etc/pulse/.bootstrap_token` |
| **Docker** | `docker exec pulse cat /data/.bootstrap_token` |
| **Kubernetes** | `kubectl exec -it <pod> -- cat /data/.bootstrap_token` |
| **Systemd** | `cat /etc/pulse/.bootstrap_token` |

### Step 2: Create Admin Account
1. Open `http://<your-ip>:7655`
2. Paste the **Bootstrap Token**.
3. Create your **Admin Username** and **Password**.

> **Note**: If you configure `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` via environment variables, this step is skipped.

---

## 🔄 Updates

### Automatic Updates (Systemd/LXC only)
Pulse can self-update to the latest stable version.

**Enable via UI**: Settings → System → Automatic Updates  
**Enable via CLI**: `systemctl enable --now pulse-update.timer`

### Manual Update
| Platform | Command |
|----------|---------|
| **LXC** | `pct exec <ID> -- update` |
| **Systemd** | Re-run the install script |
| **Docker** | `docker pull rcourtman/pulse:latest && docker restart pulse` |

### Rollback
If an update causes issues, you can roll back to the previous version instantly.

**Via UI**: Settings → System → Updates → "Restore previous version"  
**Via CLI**: `pulse config rollback`

---

## 🗑️ Uninstall

**LXC**: `pct destroy <ID>`  
**Docker**: `docker rm -f pulse && docker volume rm pulse_data`  
**Systemd**:
```bash
systemctl disable --now pulse
rm -rf /opt/pulse /etc/pulse /etc/systemd/system/pulse.service
```
