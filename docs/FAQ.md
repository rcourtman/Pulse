# ❓ Frequently Asked Questions

## 🛠️ Installation & Setup

### What's the easiest way to install?
Use Docker:
```bash
docker run -d --name pulse -p 7655:7655 -v pulse_data:/data rcourtman/pulse:latest
```

See [INSTALL.md](INSTALL.md) for all options (Docker Compose, Kubernetes, systemd).

### How do I add a node?
**Auto-discovery (Recommended)**: Go to **Settings → Nodes**, find your node in the "Discovered" list, click "Setup Script", and run the provided command on your Proxmox host.
**Manual**: Go to **Settings → Nodes → Add Node** and enter the credentials manually.

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
Pulse uses a secure sensor proxy.
1. Install `lm-sensors` on your host (`apt install lm-sensors && sensors-detect`).
2. Run the Pulse setup script on the node again to install the sensor proxy.
See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

---

## 🔐 Security & Access

### I forgot my password. How do I reset it?
**Docker**:
```bash
docker exec pulse rm /data/.env
docker restart pulse
# Access UI to run setup wizard again
```
**Systemd**:
Delete `/etc/pulse/.env` and restart the service.

### How do I enable HTTPS?
Set `HTTPS_ENABLED=true` and provide `TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables. See [Configuration](CONFIGURATION.md#https--tls).

### Can I use Single Sign-On (SSO)?
Yes. Pulse supports OIDC (Settings → Security → OIDC) and Proxy Auth (Authentik, Authelia). See [Proxy Auth Guide](PROXY_AUTH.md).

---

## ⚠️ Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006).
- Verify credentials in **Settings → Nodes**.
- Check logs: `journalctl -u pulse -f` or `docker logs -f pulse`.

### Connection refused?
- Check if Pulse is running: `systemctl status pulse` or `docker ps`.
- Verify the port (default 7655) is open on your firewall.

### CORS errors?
Set `ALLOWED_ORIGINS=https://your-domain.com` environment variable if accessing Pulse from a different domain.

### High memory usage?
Reduce `METRICS_RETENTION_DAYS` (default 7) via environment variable if running on very constrained hardware.
