# 🛡️ Sensor Proxy AppArmor (Optional)

> **Deprecated in v5:** `pulse-sensor-proxy` is deprecated and not recommended for new deployments.
> Use `pulse-agent --enable-proxmox` for temperature monitoring.
> This document is retained for existing installations during the migration window.

Secure `pulse-sensor-proxy` with AppArmor and Seccomp.

## 🛡️ AppArmor

Profile: `security/apparmor/pulse-sensor-proxy.apparmor`
*   **Allows**: Configs, logs, SSH keys, outbound TCP/SSH.
*   **Blocks**: Raw sockets, module loading, ptrace, exec outside allowlist.

### Install & Enforce
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/security/apparmor/pulse-sensor-proxy.apparmor | \
  sudo tee /etc/apparmor.d/pulse-sensor-proxy >/dev/null
sudo apparmor_parser -r /etc/apparmor.d/pulse-sensor-proxy
sudo aa-enforce pulse-sensor-proxy
```

## 🔒 Seccomp

Profile: `security/seccomp/pulse-sensor-proxy.json`
*   **Allows**: Go runtime syscalls, network, file IO.
*   **Blocks**: Everything else (returns `EPERM`).

### Systemd (Classic)
Add to service override:
```ini
[Service]
AppArmorProfile=pulse-sensor-proxy
SystemCallFilter=@system-service
SystemCallAllow=accept;connect;recvfrom;sendto;recvmsg;sendmsg;sendmmsg;getsockname;getpeername;getsockopt;setsockopt;shutdown
```

### Containers (Docker/Podman)
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/security/seccomp/pulse-sensor-proxy.json | \
  sudo tee /etc/pulse-sensor-proxy.seccomp.json >/dev/null

podman run --seccomp-profile /etc/pulse-sensor-proxy.seccomp.json ...
```

## 🔍 Verification
Check status with `aa-status` or `journalctl -t auditbeat`.
