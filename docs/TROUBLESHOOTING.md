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
1. Check which service is running: `systemctl status pulse` (or `pulse-backend`).
2. Verify environment override: `systemctl show pulse --property=Environment`.
3. Docker: Ensure you updated the `-p` flag (e.g., `-p 8080:7655`).

### "Connection Refused"
- Check if Pulse is running.
- Verify the port is open on your firewall.
- **PBS**: Remember PBS uses port **8007** and requires **HTTPS**.

---

## 🔍 Common Issues

### Authentication

**"Invalid username or password" after setup**
- **Docker Compose**: Did you escape the `$` signs in your hash? Use `$$2a$$...`.
- **Truncated Hash**: Ensure your bcrypt hash is exactly 60 characters.

**Cannot login / 401 Unauthorized**
- Clear browser cookies.
- Check if your IP is banned (wait 15 mins or restart Pulse).

### Monitoring Data

**VMs show "-" for disk usage**
- Install **QEMU Guest Agent** in the VM.
- Enable "QEMU Guest Agent" in Proxmox VM Options.
- Restart the VM.
- See [VM Disk Monitoring](VM_DISK_MONITORING.md).

**Temperature data missing**
- Install `lm-sensors` on the host.
- Run `sensors-detect`.
- Install the unified agent on the Proxmox host with `--enable-proxmox`.
- See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

**Docker hosts appearing/disappearing**
- **Duplicate IDs**: Cloned VMs often share `/etc/machine-id`.
- **Fix**: Run `rm /etc/machine-id && systemd-machine-id-setup` on the clone.

### Notifications

**Emails not sending**
- Check SMTP settings in **Settings → Alerts**.
- Check logs: `docker logs pulse | grep email`.
- Ensure your SMTP provider allows the connection (e.g., Gmail App Passwords).

**Webhooks failing**
- Verify the URL is reachable from the Pulse server.
- Check `Allowed Origins` if you are getting CORS errors.

---

## 🛠️ Advanced Diagnostics

### Correlate Logs with Requests
Every API response has an `X-Request-ID` header. Use it to find the exact log entry:
```bash
grep "request_id=abc123" /var/log/pulse/pulse.log
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
If you are completely locked out, you can trigger a recovery token from the localhost CLI:
```bash
curl -X POST http://localhost:7655/api/security/recovery \
  -d '{"action":"generate_token","duration":30}'
```
Use the returned token to bypass auth temporarily.

---

## 🆘 Getting Help

If you're still stuck:
1. **Check Logs**: `journalctl -u pulse -n 100` or `docker logs --tail 100 pulse`.
2. **Check Version**: `curl http://localhost:7655/api/version`.
3. **Open Issue**: Report on [GitHub Issues](https://github.com/rcourtman/Pulse/issues) with your logs and version info.
