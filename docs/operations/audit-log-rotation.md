# Sensor Proxy Audit Log Rotation

The temperature sensor proxy writes append-only, hash-chained audit events to
`/var/log/pulse/sensor-proxy/audit.log`. The file is created with `0640`
permissions, owned by `pulse-sensor-proxy`, and protected with `chattr +a` via
`scripts/secure-sensor-files.sh`. Because the process keeps the file handle open
and enforces append-only mode, you **must** follow the steps below to rotate the
log without losing events.

## When to Rotate

- File exceeds **200 MB** or contains more than 30 days of history
- Prior to exporting evidence for an incident review
- Immediately before changing log-forwarding endpoints (rsyslog/RELp)

The proxy falls back to stderr (systemd journal) only when the file cannot be
opened. Do not rely on the fallback for long-term retention.

## Pre-flight Checklist

1. Confirm the service is healthy:
   ```bash
   systemctl status pulse-sensor-proxy --no-pager
   ```
2. Make sure `/var/log/pulse/sensor-proxy` is mounted with enough free space:
   ```bash
   df -h /var/log/pulse/sensor-proxy
   ```
3. Note the current scheduler health inside Pulse for later verification:
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health | jq '.queue.depth, .deadLetter.count'
   ```

## Manual Rotation Procedure

> Run these steps as **root** on the Proxmox host that runs the proxy.

1. Remove the append-only flag (logrotate needs to truncate the file):
   ```bash
   chattr -a /var/log/pulse/sensor-proxy/audit.log
   ```
2. Copy the current file to an evidence path, then truncate in place:
   ```bash
   ts=$(date +%Y%m%d-%H%M%S)
   cp -a /var/log/pulse/sensor-proxy/audit.log /var/log/pulse/sensor-proxy/audit.log.$ts
   : > /var/log/pulse/sensor-proxy/audit.log
   ```
3. Restore permissions and the append-only flag:
   ```bash
   chown pulse-sensor-proxy:pulse-sensor-proxy /var/log/pulse/sensor-proxy/audit.log
   chmod 0640 /var/log/pulse/sensor-proxy/audit.log
   chattr +a /var/log/pulse/sensor-proxy/audit.log
   ```
4. Restart the proxy so the file descriptor is reopened:
   ```bash
   systemctl restart pulse-sensor-proxy
   ```
5. Verify the service recreated the correlation hash chain:
   ```bash
   journalctl -u pulse-sensor-proxy -n 20 | grep -i "audit" || true
   ```
6. Re-check Pulse adaptive polling health (temperature pollers rely on the
   proxy):
   ```bash
   curl -s http://localhost:7655/api/monitoring/scheduler/health \
     | jq '.instances[] | select(.key | contains("temperature")) | {key, breaker: .breaker.state, deadLetter: .deadLetter.present}'
   ```
   All temperature instances should show `breaker: "closed"` with
   `deadLetter: false`.

## Logrotate Configuration

Automate rotation with `/etc/logrotate.d/pulse-sensor-proxy`. Copy the snippet
below and adjust retention to match your compliance needs:

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

Keep `copytruncate` disabled—the restart ensures the proxy writes to a fresh
file with a new hash chain. Always forward rotated files to your SIEM before
removing them.

## Forwarding Validations

If you forward audit logs over RELP using `scripts/setup-log-forwarding.sh`:

1. Tail the forwarding log:
   ```bash
   tail -f /var/log/pulse/sensor-proxy/forwarding.log
   ```
2. Ensure queues drain (`action.resumeRetryCount=-1` keeps retrying).
3. Confirm the remote receiver ingests the new file (look for the `pulse.audit`
tag).

## Troubleshooting

| Symptom | Action |
| --- | --- |
| `Operation not permitted` when truncating | `chattr -a` was not executed or SELinux/AppArmor denies it. Check `auditd`. |
| Proxy fails to restart | Run `journalctl -u pulse-sensor-proxy -xe` for context. The proxy refuses to start if the audit file cannot be opened. |
| Temperature polls stop after rotation | Check `/api/monitoring/scheduler/health` for dead-letter entries. Restart the main Pulse service if breakers stay open. |

Once logs are rotated and validated, upload the archived copy to your evidence
store and record the event in your change log.
