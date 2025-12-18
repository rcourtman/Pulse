# 🔄 Sensor Proxy Audit Log Rotation

> **Deprecated in v5:** `pulse-sensor-proxy` is deprecated and not recommended for new deployments.
> This document is retained for existing installations during the migration window.

The proxy writes append-only, hash-chained logs to `/var/log/pulse/sensor-proxy/audit.log`.

## ⚠️ Important
*   **Do not delete**: The file is protected with `chattr +a`.
*   **Rotate when**: >200MB or >30 days.

## 🛠️ Manual Rotation

Run as root:

```bash
# 1. Unlock file
chattr -a /var/log/pulse/sensor-proxy/audit.log

# 2. Rotate (copy & truncate)
cp -a /var/log/pulse/sensor-proxy/audit.log /var/log/pulse/sensor-proxy/audit.log.$(date +%Y%m%d)
: > /var/log/pulse/sensor-proxy/audit.log

# 3. Relock & Restart
chown pulse-sensor-proxy:pulse-sensor-proxy /var/log/pulse/sensor-proxy/audit.log
chmod 0640 /var/log/pulse/sensor-proxy/audit.log
chattr +a /var/log/pulse/sensor-proxy/audit.log
systemctl restart pulse-sensor-proxy
```

## 🤖 Logrotate Config

Create `/etc/logrotate.d/pulse-sensor-proxy`:

```conf
/var/log/pulse/sensor-proxy/audit.log {
    weekly
    rotate 8
    compress
    missingok
    notifempty
    create 0640 pulse-sensor-proxy pulse-sensor-proxy
    sharedscripts
    prerotate
        /usr/bin/chattr -a /var/log/pulse/sensor-proxy/audit.log || true
    endscript
    postrotate
        /bin/systemctl restart pulse-sensor-proxy.service || true
        /usr/bin/chattr +a /var/log/pulse/sensor-proxy/audit.log || true
    endscript
}
```

**Note**: Do NOT use `copytruncate`. The restart is required to reset the hash chain.
