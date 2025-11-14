# Sensor Proxy Log Forwarding

Forward `pulse-sensor-proxy` logs to a central syslog/SIEM endpoint so audit
records survive host loss and can drive alerting. Pulse ships a helper script
(`scripts/setup-log-forwarding.sh`) that configures rsyslog to ship both
`audit.log` and `proxy.log` over RELP + TLS.

## Requirements

- Debian/Ubuntu host with **rsyslog** and the `imfile` + `omrelp` modules (present
  by default).
- Root privileges to install certificates and restart rsyslog.
- TLS assets for the RELP connection:
  - `ca.crt` – CA that issued the remote collector certificate.
  - `client.crt` / `client.key` – mTLS credentials for this host.
- Network access to the remote collector (`REMOTE_HOST`, default `logs.pulse.example`,
  port `6514`).

## Installation Steps

1. Copy your CA and client certificates into a safe directory on the host (the
   script defaults to `/etc/pulse/log-forwarding`).
2. Run the helper with environment overrides for your collector:
   ```bash
   sudo REMOTE_HOST=logs.company.tld \
        REMOTE_PORT=6514 \
        CERT_DIR=/etc/pulse/log-forwarding \
        CA_CERT=/etc/pulse/log-forwarding/ca.crt \
        CLIENT_CERT=/etc/pulse/log-forwarding/pulse.crt \
        CLIENT_KEY=/etc/pulse/log-forwarding/pulse.key \
        /opt/pulse/scripts/setup-log-forwarding.sh
   ```
   The script writes `/etc/rsyslog.d/pulse-sensor-proxy.conf`, ensures the
   certificate directory exists (`0750`), and restarts rsyslog.

## What the Script Configures

- Two `imfile` inputs that watch `/var/log/pulse/sensor-proxy/audit.log` and
  `/var/log/pulse/sensor-proxy/proxy.log` with `Tag`s `pulse.audit` and
  `pulse.app`.
- A local mirror file at `/var/log/pulse/sensor-proxy/forwarding.log` so you can
  inspect rsyslog activity.
- An RELP action with TLS, infinite retry (`action.resumeRetryCount=-1`), and a
  50k message disk-backed queue to absorb collector outages.

## Verification Checklist

1. Confirm rsyslog picked up the new config:
   ```bash
   sudo rsyslogd -N1
   sudo systemctl status rsyslog --no-pager
   ```
2. Tail the local mirror to ensure entries stream through:
   ```bash
   sudo tail -f /var/log/pulse/sensor-proxy/forwarding.log
   ```
3. On the collector side, filter for the `pulse.audit` tag and make sure new
   entries arrive. For Splunk/ELK, index on `programname`.
4. Simulate a test event (e.g., restart `pulse-sensor-proxy` or deny a fake peer)
   and verify it appears remotely.

## Maintenance

- **Certificate rotation**: Replace the key/cert files, then restart rsyslog.
  Because the config points at static paths, no additional edits are required.
- **Disable forwarding**: Remove `/etc/rsyslog.d/pulse-sensor-proxy.conf` and run
  `sudo systemctl restart rsyslog`. The local audit log remains untouched.
- **Queue monitoring**: Track rsyslog’s main log or use `rsyslogd -N6` to check
  for queue overflows. At scale, scrape `/var/log/pulse/sensor-proxy/forwarding.log`
  for `action resumed` messages.

For rotation guidance on the underlying audit file, see
[operations/audit-log-rotation.md](audit-log-rotation.md).
