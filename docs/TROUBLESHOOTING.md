# 🔧 Troubleshooting Guide

## ⚡ Quick Fixes

### I forgot my password
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

### Port change didn't take effect
1. Check which service is running: `systemctl status pulse` (legacy installs may use `pulse-backend`).
2. Verify environment override: `systemctl show pulse --property=Environment`.
3. Docker: Ensure you updated the `-p` flag (e.g., `-p 8080:7655`).

### "Connection Refused"
- Check if Pulse is running.
- Verify the port is open on your firewall.
- **PBS**: Remember PBS uses port **8007** and requires **HTTPS**.

---

## 🔍 Common Issues

### Authentication

#### "Invalid username or password" after setup
- **Docker Compose**: Did you escape the `$` signs in your hash? Use `$$2a$$...`.
- **Truncated Hash**: Ensure your bcrypt hash is exactly 60 characters.

#### Cannot login / 401 Unauthorized
- Clear browser cookies.
- Check if your IP is locked out (wait 15 mins).
- If another admin can log in, use `POST /api/security/reset-lockout` to clear the lockout for your username or IP.

#### Audit Log verification shows unsigned events
- **Symptom**: Audit Log entries show “Unsigned” or verification fails in the UI.
- **Root cause**: Audit signing is disabled (crypto manager unavailable), so events are stored without signatures.
- **Fix**: Ensure `.encryption.key` is present and Pro/Cloud audit logging is enabled, then restart Pulse to regenerate `.audit-signing.key`. Newly created events will be signed; existing unsigned events remain unsigned.

#### Audit Log is empty
- **Symptom**: Audit Log shows zero events or "Console Logging Only."
- **Root cause**: Community plan uses console logging only, or Pro/Cloud audit logging is not enabled.
- **Fix**: Use Pro/Cloud with audit logging enabled, then generate new audit events (logins, token creation, password changes).

#### Audit Log verification fails for older events
- **Symptom**: Older events fail verification while newer events pass.
- **Root cause**: The audit signing key changed (for example, `.audit-signing.key` was regenerated), so signatures no longer match.
- **Fix**: Restore the previous `.audit-signing.key` from backup to verify older events. If rotated intentionally, expect older events to fail verification.

### Monitoring Data

#### VMs show "-" for disk usage
- Install **QEMU Guest Agent** in the VM.
- Enable "QEMU Guest Agent" in Proxmox VM Options.
- Restart the VM.
- See [VM Disk Monitoring](VM_DISK_MONITORING.md).

#### Temperature data missing
- Install `lm-sensors` on the host.
- Run `sensors-detect`.
- Install the unified agent on the Proxmox host with `--enable-proxmox`.
- See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

#### Docker hosts appearing/disappearing
- **Duplicate IDs**: Cloned VMs often share `/etc/machine-id`.
- **Fix**: Run `rm /etc/machine-id && systemd-machine-id-setup` on the clone.

### Notifications

#### Emails not sending
- Check SMTP settings in **Alerts → Notification Destinations**.
- Check logs: `docker logs pulse | grep email`.
- Ensure your SMTP provider allows the connection (e.g., Gmail App Passwords).

#### Webhooks failing
- Verify the URL is reachable from the Pulse server.
- If targeting private IPs, allow them in **Settings → System → Network → Webhook Security**.
- Check Pulse logs for HTTP status codes and response bodies.

### TrueNAS

#### "TrueNAS service unavailable"
- Ensure TrueNAS was added in **Settings → TrueNAS** with a valid URL and API key.
- Check that the TrueNAS system is reachable from the Pulse server (default HTTPS port).
- Verify the API key has read access. Test with:
  ```bash
  curl -sk -H "Authorization: Bearer <api-key>" https://<truenas-ip>/api/v2.0/system/info
  ```

#### TrueNAS pools/datasets not appearing
- TrueNAS data appears in the unified resource model and may take one polling cycle (30s) to appear.
- Check **Infrastructure** (TrueNAS host), **Storage** (pools/datasets), and **Recovery** (snapshots/replication).

### Navigation (v6)

#### Old bookmarks don't work
- Legacy URLs (`/proxmox`, `/docker`, `/kubernetes`, `/hosts`, `/services`) redirect automatically to v6 equivalents.
- If redirects are disabled (via `PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS=true`), update your bookmarks. See [Migration Guide](MIGRATION_UNIFIED_NAV.md).

### Relay / Mobile

#### Relay showing "Disconnected"
- Confirm a valid Pro or Cloud license is active (**Settings → License**).
- Check Pulse server can reach the relay server (outbound WebSocket to `relay.pulserelay.pro`).
- Review logs: `journalctl -u pulse | grep relay` or `docker logs pulse | grep relay`.

---

## 🛠️ Advanced Diagnostics

### Correlate Logs with Requests
Every API response has an `X-Request-ID` header. Use it to find the exact log entry:
```bash
# systemd / Proxmox LXC
journalctl -u pulse --no-pager | grep "request_id=abc123"

# Docker
docker logs pulse 2>&1 | grep "request_id=abc123"
```

### Check Permissions (Proxmox)
If Pulse can't see VMs or storage, check the user permissions on Proxmox:
```bash
pveum user permissions <user>@pam
```
At minimum, ensure the user/token has read access for inventory and metrics:

- `Sys.Audit`
- `VM.Monitor`
- `Datastore.Audit`

For VM disk usage via QEMU guest agent, also ensure `VM.GuestAgent.Audit` (PVE 9+).

### Recovery Mode
If you are completely locked out, you can trigger a recovery token from localhost:
```bash
curl -X POST http://localhost:7655/api/security/recovery \
  -d '{"action":"generate_token","duration":30}'
```
Use the returned token in `X-Recovery-Token` when calling `/api/security/recovery` to enable or disable local-only auth bypass (`disable_auth` / `enable_auth`). Token generation is localhost-only.

Example (enable recovery mode):
```bash
curl -X POST http://localhost:7655/api/security/recovery \
  -H "X-Recovery-Token: <token>" \
  -d '{"action":"disable_auth"}'
```

---

## 🆘 Getting Help

If you're still stuck:
1. **Check Logs**: `journalctl -u pulse -n 100` or `docker logs --tail 100 pulse`.
2. **Check Version**: `curl http://localhost:7655/api/version`.
3. **Open Issue**: Report on [GitHub Issues](https://github.com/rcourtman/Pulse/issues) with your logs and version info.
