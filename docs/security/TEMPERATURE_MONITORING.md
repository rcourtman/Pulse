# 🌡️ Temperature Monitoring Security

Secure architecture for collecting hardware temperatures.

## 🛡️ Security Model
*   **Isolation**: SSH keys live on the host, not in the container.
*   **Least Privilege**: Proxy runs as `pulse-sensor-proxy` (no shell).
*   **Verification**: Container identity verified via `SO_PEERCRED`.

## 🏗️ Components
1.  **Pulse Backend**: Connects to Unix socket `/mnt/pulse-proxy/pulse-sensor-proxy.sock`.
2.  **Sensor Proxy**: Validates request, executes SSH to node.
3.  **Target Node**: Accepts SSH key restricted to `sensors -j`.

## 🔒 Key Restrictions
SSH keys deployed to nodes are locked down:
```
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty
```

## 🚦 Rate Limiting
*   **Per Peer**: ~12 req/min.
*   **Concurrency**: Max 2 parallel requests per peer.
*   **Global**: Max 8 concurrent requests.

## 📝 Auditing
All requests logged to system journal:
```bash
journalctl -u pulse-sensor-proxy
```
Logs include: `uid`, `pid`, `method`, `node`, `correlation_id`.
