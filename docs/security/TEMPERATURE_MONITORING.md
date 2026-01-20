# 🌡️ Temperature Monitoring

This page describes the recommended v5 approach for temperature monitoring and the security tradeoffs between approaches.

For the full sensor-proxy setup guide (socket mounts, HTTP mode, troubleshooting), see:
`docs/TEMPERATURE_MONITORING.md`.

> **Deprecation notice (v5):** `pulse-sensor-proxy` is deprecated and not recommended for new deployments. Use `pulse-agent --enable-proxmox` for temperature monitoring. The sensor-proxy section below is retained for existing installations during the migration window. In v5, legacy sensor-proxy endpoints are disabled by default unless `PULSE_ENABLE_SENSOR_PROXY=true` is set on the Pulse server.

## Recommended: Pulse Agent

The simplest and most feature-rich method is installing the Pulse agent on your Proxmox nodes:

```bash
curl -fsSL http://your-pulse-server:7655/install.sh | bash -s -- \
  --url http://your-pulse-server:7655 \
  --token YOUR_TOKEN \
  --enable-proxmox
```

**Benefits:**
- ✅ One-command setup
- ✅ Temperature monitoring built-in
- ✅ No SSH keys or proxy configuration required

The agent runs `sensors -j` locally and reports temperatures directly to Pulse.

---

## Deprecated: Sensor Proxy (Host Service)

`pulse-sensor-proxy` is deprecated in v5 and is not recommended for new deployments. This section is retained for existing installations during the migration window.

### 🛡️ Security Model
- **Isolation**: SSH keys live on the host, not in the container.
- **Least Privilege**: Proxy runs as `pulse-sensor-proxy` (no shell).
- **Verification**: Container identity verified via `SO_PEERCRED`.

### 🏗️ Components
1.  **Pulse Backend**: Connects to Unix socket `/mnt/pulse-proxy/pulse-sensor-proxy.sock`.
2.  **Sensor Proxy**: Validates request, executes SSH to node.
3.  **Target Node**: Accepts SSH key restricted to `sensors -j`.

### 🔒 Key Restrictions
SSH keys deployed to nodes are locked down:
```text
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty
```

### 🚦 Rate Limiting
- **Per Peer**: ~12 req/min.
- **Concurrency**: Max 2 parallel requests per peer.
- **Global**: Max 8 concurrent requests.

### 📝 Auditing
All requests logged to system journal:
```bash
journalctl -u pulse-sensor-proxy
```
Logs include: `uid`, `pid`, `method`, `node`, `correlation_id`.

### Related Docs

- Unified Agent Security: [`docs/AGENT_SECURITY.md`](../AGENT_SECURITY.md)
- Repository Security Policy: [`/SECURITY.md`](../../SECURITY.md)
