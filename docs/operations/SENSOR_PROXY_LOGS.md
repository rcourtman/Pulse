# 📝 Sensor Proxy Log Forwarding

> **Deprecated in v5:** `pulse-sensor-proxy` is deprecated and not recommended for new deployments.
> Use `pulse-agent --enable-proxmox` for temperature monitoring.
> This document is retained for existing installations during the migration window.

Forward `audit.log` and `proxy.log` to a central SIEM via RELP + TLS.

## 🚀 Quick Start
Run the helper script with your collector details:

```bash
sudo REMOTE_HOST=logs.example.com \
     REMOTE_PORT=6514 \
     CERT_DIR=/etc/pulse/log-forwarding \
     CA_CERT=/path/to/ca.crt \
     CLIENT_CERT=/path/to/client.crt \
     CLIENT_KEY=/path/to/client.key \
     bash -c "$(curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/setup-log-forwarding.sh)"
```

## 📋 What It Does
1.  **Inputs**: Watches `/var/log/pulse/sensor-proxy/{audit,proxy}.log`.
2.  **Queue**: Disk-backed queue (50k messages) for reliability.
3.  **Output**: RELP over TLS to `REMOTE_HOST`.
4.  **Mirror**: Local debug file at `/var/log/pulse/sensor-proxy/forwarding.log`.

## ✅ Verification
1.  **Check Status**: `sudo systemctl status rsyslog`
2.  **View Mirror**: `tail -f /var/log/pulse/sensor-proxy/forwarding.log`
3.  **Test**: Restart proxy and check remote collector for `pulse.audit` tag.

## 🧹 Maintenance
*   **Disable**: Remove `/etc/rsyslog.d/pulse-sensor-proxy.conf` and restart rsyslog.
*   **Rotate Certs**: Replace files in `CERT_DIR` and restart rsyslog.
