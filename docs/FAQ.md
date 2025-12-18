# ❓ Frequently Asked Questions

## 🛠️ Installation & Setup

### What's the easiest way to install?
If you run Proxmox VE, use the official LXC installer (recommended):

```bash
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
```

If you prefer Docker:

```bash
docker run -d --name pulse -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest
```

See [INSTALL.md](INSTALL.md) for all options (Docker Compose, Kubernetes, systemd).

### How do I add a node?
Go to **Settings → Proxmox**.

- **Recommended (Agent setup)**: choose **Setup mode: Agent** and run the generated install command on the Proxmox host.
- **Manual**: choose **Setup mode: Manual** and enter the credentials (password or API token) for the Proxmox API.

If you want Pulse to find servers automatically, enable discovery in **Settings → System → Network** and then return to **Settings → Proxmox** to review discovered servers.

### How do I change the port?
- **Systemd**: `sudo systemctl edit pulse`, add `Environment="FRONTEND_PORT=8080"`, restart.
- **Docker**: Use `-p 8080:7655` in your run command.

### Why can't I change settings in the UI?
If a setting is disabled with an amber warning, it's being overridden by an environment variable (e.g., `DISCOVERY_ENABLED`). Remove the env var to regain UI control.

---

## 🔍 Monitoring & Metrics

### Why do VMs show "-" for disk usage?
Proxmox API returns `0` for VM disk usage by default. You must install the **QEMU Guest Agent** inside the VM and enable it in Proxmox (VM → Options → QEMU Guest Agent).
See [VM Disk Monitoring](VM_DISK_MONITORING.md) for details.

### Does Pulse monitor Ceph?
Yes! If Pulse detects Ceph storage, it automatically queries cluster health, OSD status, and pool usage. No extra config needed.

### Can I disable alerts for specific metrics?
Yes. Go to **Alerts → Thresholds** and set any value to `-1` to disable it. You can do this globally or per-resource (VM/Node).

### How do I monitor temperature?
Install the unified agent on your Proxmox hosts with Proxmox integration enabled:

1. Install `lm-sensors` on the host (`apt install lm-sensors && sensors-detect`)
2. Install `pulse-agent` with `--enable-proxmox`

`pulse-sensor-proxy` is deprecated in v5 and is not recommended for new deployments.
See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

---

## 🔐 Security & Access

### I forgot my password. How do I reset it?
**Docker**:
```bash
docker exec pulse rm /data/.env
docker restart pulse
# Access UI again. Pulse will require a bootstrap token for setup.
# Get it with:
docker exec pulse /app/pulse bootstrap-token
```
**Systemd**:
Delete `/etc/pulse/.env` and restart the service. Pulse will require a bootstrap token for setup:

```bash
sudo pulse bootstrap-token
```

### How do I enable HTTPS?
Set `HTTPS_ENABLED=true` and provide `TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables. See [Configuration](CONFIGURATION.md#https--tls).

### Can I use Single Sign-On (SSO)?
Yes. Pulse supports OIDC in **Settings → Security → Single Sign-On** and Proxy Auth (Authentik, Authelia). See [Proxy Auth Guide](PROXY_AUTH.md) and [OIDC](OIDC.md).

---

## ⚠️ Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006).
- Verify credentials in **Settings → Proxmox**.
- Check logs: `journalctl -u pulse -f` or `docker logs -f pulse`.

### Connection refused?
- Check if Pulse is running: `systemctl status pulse` or `docker ps`.
- Verify the port (default 7655) is open on your firewall.

### CORS errors?
Set `ALLOWED_ORIGINS=https://your-domain.com` environment variable if accessing Pulse from a different domain.

### High memory usage?
If you are storing long history windows, reduce metrics retention (see [METRICS_HISTORY.md](METRICS_HISTORY.md)). Also confirm your polling intervals match your environment size.
