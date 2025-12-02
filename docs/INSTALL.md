# 📦 Installation Guide

Pulse offers flexible installation options from Docker to enterprise-ready Kubernetes charts.

## 🚀 Quick Start (Recommended)

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

### 1. Kubernetes (Helm)
Deploy to your cluster using our Helm chart.

```bash
helm repo add pulse https://rcourtman.github.io/Pulse/
helm repo update
helm install pulse pulse/pulse \
  --namespace pulse \
  --create-namespace
```
See [KUBERNETES.md](KUBERNETES.md) for ingress and persistence configuration.

### 2. Bare Metal / Systemd
For bare-metal Linux servers, download the release binary directly.

```bash
# Download and extract
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/pulse-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m).tar.gz | tar xz
sudo mv pulse /usr/local/bin/
sudo chmod +x /usr/local/bin/pulse

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

---

## 🔐 First-Time Setup

Pulse is secure by default. On first launch, you must retrieve a **Bootstrap Token** to create your admin account.

### Step 1: Get the Token

| Platform | Command |
|----------|---------|
| **Docker** | `docker exec pulse cat /data/.bootstrap_token` |
| **Kubernetes** | `kubectl exec -it <pod> -- cat /data/.bootstrap_token` |
| **Systemd** | `sudo cat /etc/pulse/.bootstrap_token` |

### Step 2: Create Admin Account
1. Open `http://<your-ip>:7655`
2. Paste the **Bootstrap Token**.
3. Create your **Admin Username** and **Password**.

> **Note**: If you configure `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` via environment variables, this step is skipped.

---

## 🔄 Updates

### Automatic Updates (Systemd only)
Pulse can self-update to the latest stable version.

**Enable via UI**: Settings → System → Automatic Updates

### Manual Update
| Platform | Command |
|----------|---------|
| **Docker** | `docker pull rcourtman/pulse:latest && docker restart pulse` |
| **Kubernetes** | `helm repo update && helm upgrade pulse pulse/pulse -n pulse` |
| **Systemd** | Re-download binary and restart service |

### Rollback
If an update causes issues on systemd installations, backups are created automatically during the update process.

**Manual rollback**: Check for backup directories at `/etc/pulse/backup-<timestamp>/` created during updates. Restore the previous binary manually if needed.

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
