# ❓ Frequently Asked Questions

## 🛠️ Installation & Setup

### What's the easiest way to install?
If you run Proxmox VE, use the signed LXC installer flow in [INSTALL.md](INSTALL.md) and replace `vX.Y.Z` with the exact release tag you want.

If you prefer Docker:

Use a pinned image tag such as `rcourtman/pulse:vX.Y.Z` instead of `:latest`.

See [INSTALL.md](INSTALL.md) for all options (Docker Compose, Kubernetes, systemd).

### How do I add a node?
Go to **Settings → Infrastructure → Install on a host** for systems that should
run the unified agent directly.

- **Recommended (agent setup)**: copy the generated install command and run it on the target host.
- **Manual/API-backed platforms**: use **Settings → Infrastructure → Platform connections** for systems such as Proxmox, PBS, PMG, or TrueNAS that connect over an API instead of running the agent locally.

If you want Pulse to find servers automatically, enable discovery in **Settings → System → Network** and then review discovered servers in **Settings → Infrastructure**.

### How do I change the port?
- **Systemd**: `sudo systemctl edit pulse`, add `Environment="FRONTEND_PORT=8080"`, restart.
- **Docker**: Use `-p 8080:7655` in your run command.

### Why can't I change settings in the UI?
If a setting is disabled with an amber warning, it's being overridden by an environment variable (e.g., `DISCOVERY_ENABLED`). Remove the env var to regain UI control.

---

## 🔍 Monitoring & Metrics

### What do Relay, Pro, and Cloud unlock?
Relay adds remote access, mobile access, push notifications, and 14-day history. Pro and Cloud unlock **alert-triggered root-cause analysis and safe remediation workflows** along with the broader operations feature set. Existing legacy Pro+ holders keep their current continuity, but self-hosted pricing no longer sells more monitoring volume. Pulse Patrol is available to everyone on Community with BYOK and provides scheduled, cross-system analysis that correlates real-time state, recent metrics history, and diagnostics to surface actionable findings.

Example output includes trend-based capacity warnings, backup regressions, Kubernetes cluster analysis, and correlated container failures that simple threshold alerts miss.
See [Pulse AI](AI.md), [Plans and entitlements](PULSE_PRO.md), and <https://pulserelay.pro>.

### Why do VMs show "-" for disk usage?
Proxmox API returns `0` for VM disk usage by default. You must install the **QEMU Guest Agent** inside the VM and enable it in Proxmox (VM → Options → QEMU Guest Agent).
See [VM Disk Monitoring](VM_DISK_MONITORING.md) for details.

### Does Pulse monitor Ceph?
Yes! If Pulse detects Ceph storage, it automatically queries cluster health, OSD status, and pool usage. No extra config needed.

### Does Pulse monitor TrueNAS?
Yes! Pulse v6 includes first-class TrueNAS SCALE/CORE integration. Add your TrueNAS server in **Settings → TrueNAS** with the URL and API key. Pulse monitors pools, datasets, disks, ZFS snapshots, replication tasks, and alerts. TrueNAS resources appear in the unified Infrastructure, Storage, and Recovery views.

### Where did my pages go? (Unified Navigation)
Pulse v6 organises the UI by **task** instead of **platform**:
- **Infrastructure** → all hosts (Proxmox, Docker, K8s, TrueNAS)
- **Workloads** → VMs, LXCs, containers, pods
- **Storage** → all storage pools
- **Recovery** → backups, snapshots, replication

Legacy URLs (`/proxmox`, `/docker`, `/kubernetes`, `/hosts`, `/services`) redirect automatically. See [Migration Guide](MIGRATION_UNIFIED_NAV.md) for the full mapping.

### Can I disable alerts for specific metrics?
Yes. Go to **Alerts → Thresholds** and set any value to `-1` to disable it. You can do this globally or per-resource (VM/Node).

### How do I monitor temperature?
Recommended: install the unified agent on your Proxmox hosts with Proxmox integration enabled:

1. Install `lm-sensors` on the host (`apt install lm-sensors && sensors-detect`)
2. Install `pulse-agent` with `--enable-proxmox`

If you do not run the agent, Pulse can collect temperatures over SSH. See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

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
Yes. Pulse supports **OIDC** and **SAML** SSO providers, with multi-provider support (multiple IdPs active simultaneously). Configure in **Settings → Security → SSO Providers**. Pulse also supports Proxy Auth (Authentik, Authelia, Cloudflare). See [Proxy Auth Guide](PROXY_AUTH.md).

---

## ⚠️ Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006).
- Verify credentials in **Settings → Infrastructure**.
- Check logs: `journalctl -u pulse -f` or `docker logs -f pulse`.

### Connection refused?
- Check if Pulse is running: `systemctl status pulse` or `docker ps`.
- Verify the port (default 7655) is open on your firewall.

### CORS errors?
Pulse defaults to same-origin only. If you access the API from a different domain, set **Settings → System → Network → Allowed Origins** or use `ALLOWED_ORIGINS` (single origin, or `*` if you explicitly want all origins).

### High memory usage?
If you are storing long history windows, reduce metrics retention (see [METRICS_HISTORY.md](METRICS_HISTORY.md)). Also confirm your polling intervals match your environment size.
