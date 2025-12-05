# 🛡️ Sensor Proxy Hardening

> **⚠️ Deprecated:** The sensor-proxy is deprecated in favor of the unified Pulse agent.
> For new installations, use `install.sh --enable-proxmox` instead.
> See [TEMPERATURE_MONITORING.md](/docs/security/TEMPERATURE_MONITORING.md).

Secure `pulse-sensor-proxy` with AppArmor and Seccomp.

## 🛡️ AppArmor

Profile: `security/apparmor/pulse-sensor-proxy.apparmor`
*   **Allows**: Configs, logs, SSH keys, outbound TCP/SSH.
*   **Blocks**: Raw sockets, module loading, ptrace, exec outside allowlist.

### Install & Enforce
```bash
sudo install -m 0644 security/apparmor/pulse-sensor-proxy.apparmor /etc/apparmor.d/pulse-sensor-proxy
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
podman run --seccomp-profile /opt/pulse/security/seccomp/pulse-sensor-proxy.json ...
```

## 🔍 Verification
Check status with `aa-status` or `journalctl -t auditbeat`.
