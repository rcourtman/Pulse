# 🌡️ Temperature Monitoring

Pulse supports two methods for collecting hardware temperatures from Proxmox nodes.

## Recommended: Pulse Agent

The simplest and most feature-rich method is installing the Pulse agent on your Proxmox nodes:

```bash
curl -fsSL http://your-pulse-server:7655/api/download/install.sh | bash -s -- \
  --url http://your-pulse-server:7655 \
  --token YOUR_TOKEN \
  --enable-proxmox
```

**Benefits:**
- ✅ One-command setup
- ✅ Automatic API token creation
- ✅ Temperature monitoring built-in
- ✅ Enables AI features for VM/container management
- ✅ No SSH keys or proxy configuration required

The agent runs `sensors -j` locally and reports temperatures directly to Pulse.

---

## Legacy: Sensor Proxy (SSH-based)

For users who prefer not to install an agent on their hypervisor, the sensor-proxy method is still available.

> **Note:** This method is deprecated and will be removed in a future release. Consider migrating to the agent-based approach.

### 🛡️ Security Model
*   **Isolation**: SSH keys live on the host, not in the container.
*   **Least Privilege**: Proxy runs as `pulse-sensor-proxy` (no shell).
*   **Verification**: Container identity verified via `SO_PEERCRED`.

### 🏗️ Components
1.  **Pulse Backend**: Connects to Unix socket `/mnt/pulse-proxy/pulse-sensor-proxy.sock`.
2.  **Sensor Proxy**: Validates request, executes SSH to node.
3.  **Target Node**: Accepts SSH key restricted to `sensors -j`.

### 🔒 Key Restrictions
SSH keys deployed to nodes are locked down:
```
command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty
```

### 🚦 Rate Limiting
*   **Per Peer**: ~12 req/min.
*   **Concurrency**: Max 2 parallel requests per peer.
*   **Global**: Max 8 concurrent requests.

### 📝 Auditing
All requests logged to system journal:
```bash
journalctl -u pulse-sensor-proxy
```
Logs include: `uid`, `pid`, `method`, `node`, `correlation_id`.
