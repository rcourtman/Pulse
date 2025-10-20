# Pulse Sensor Proxy Audit Log Rotation

The sensor proxy writes a tamper-evident audit trail to
`/var/log/pulse/sensor-proxy/audit.log`. Every entry includes the SHA-256 hash
of the previous entry, so any modification becomes obvious. Because the process
keeps the file open and maintains the running hash in memory, rotation requires
special handling.

## Rotation Strategy

Use `logrotate` to rotate the file once it reaches 100 MB. After each rotation,
restart the proxy so it opens a new file and starts a fresh hash chain.

Create `/etc/logrotate.d/pulse-sensor-proxy` with the following contents:

```conf
/var/log/pulse/sensor-proxy/audit.log {
    daily
    size 100M
    rotate 90
    compress
    delaycompress
    missingok
    notifempty
    create 0640 pulse pulse
    sharedscripts
    postrotate
        systemctl restart pulse-sensor-proxy.service >/dev/null 2>&1 || true
    endscript
}
```

### Why a Restart Is Mandatory

`copytruncate` and similar tricks break the chain integrity. Restarting the
service ensures:

1. The proxy releases the old file descriptor.
2. A new hash chain starts at sequence 1 with an all-zero `prev_hash`.

If the proxy is not restarted, it will continue writing to the renamed file and
the rotation will have no effect.

### Chain Continuity Across Rotations

Each rotated log (`audit.log.1.gz`, `audit.log.2.gz`, …) is self-contained. To
prove continuity between files:

1. After each rotation, record the final `event_hash` from the rotated file (for
   example, store it in the filename or a checksum manifest).
2. When reviewing logs, verify the `prev_hash` of the first entry in the new
   file is the zero hash, and reconcile the recorded final hash from the prior
   file to show no entries were removed.

Maintaining this “final hash ledger” allows auditors to stitch the rotated files
together chronologically while preserving the tamper-evident guarantees.

### Permissions

Adjust the `create` directive to match the user and group that run the sensor
proxy. The example assumes both user and group are `pulse`.
