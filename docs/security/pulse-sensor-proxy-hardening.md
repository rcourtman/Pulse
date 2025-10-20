# Pulse Sensor Proxy AppArmor & Seccomp Hardening

## AppArmor Profile
- Profile path: `security/apparmor/pulse-sensor-proxy.apparmor`
- Grants read-only access to configs, logs, SSH keys, and binaries; allows outbound TCP/SSH; blocks raw sockets, module loading, ptrace, and absolute command execution outside the allowlist.

### Installation
```bash
sudo install -m 0644 security/apparmor/pulse-sensor-proxy.apparmor /etc/apparmor.d/pulse-sensor-proxy
sudo apparmor_parser -r /etc/apparmor.d/pulse-sensor-proxy
sudo ln -sf /etc/apparmor.d/pulse-sensor-proxy /etc/apparmor.d/force-complain/pulse-sensor-proxy  # optional staged mode
sudo systemctl restart apparmor
```

### Enforce Mode
```bash
sudo aa-enforce pulse-sensor-proxy
```
Monitor `/var/log/syslog` for `DENIED` events and update the profile as needed.

## Seccomp Filter
- OCI-style profile: `security/seccomp/pulse-sensor-proxy.json`
- Allows standard Go runtime syscalls, network operations, file IO, and `execve` for whitelisted helpers; other syscalls return `EPERM`.

### Apply via systemd (classic service)
Add to the override:
```ini
[Service]
AppArmorProfile=pulse-sensor-proxy
RestrictNamespaces=yes
NoNewPrivileges=yes
SystemCallFilter=@system-service
SystemCallArchitectures=native
SystemCallAllow=accept;connect;recvfrom;sendto;recvmsg;sendmsg;sendmmsg;getsockname;getpeername;getsockopt;setsockopt;shutdown
```

Reload and restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart pulse-sensor-proxy
```

### Apply seccomp JSON (containerised deployments)
- Profile: `security/seccomp/pulse-sensor-proxy.json`
- Use with Podman/Docker style runtimes:
```bash
podman run --seccomp-profile /opt/pulse/security/seccomp/pulse-sensor-proxy.json ...
```

## Operational Notes
- Use `journalctl -t auditbeat -g pulse-sensor-proxy` or `aa-status` to confirm profile status.
- Pair with network ACLs (see `docs/security/pulse-sensor-proxy-network.md`) and log shipping (`scripts/setup-log-forwarding.sh`).
