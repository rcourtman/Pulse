# ⚙️ Sensor Proxy Configuration

> **⚠️ Deprecated:** The sensor-proxy is deprecated in favor of the unified Pulse agent.
> For new installations, use `install.sh --enable-proxmox` instead.
> See [TEMPERATURE_MONITORING.md](/docs/security/TEMPERATURE_MONITORING.md).

Safe configuration management using the CLI (v4.31.1+).

## 📂 Files
*   **`config.yaml`**: General settings (logging, metrics).
*   **`allowed_nodes.yaml`**: Authorized node list (managed via CLI).

## 🛠️ CLI Reference

### Validation
Check for errors before restart.
```bash
pulse-sensor-proxy config validate
```

### Managing Nodes
**Add Nodes (Merge):**
```bash
pulse-sensor-proxy config set-allowed-nodes --merge 192.168.0.10
```

**Replace List:**
```bash
pulse-sensor-proxy config set-allowed-nodes --replace \
  --merge 192.168.0.1 --merge 192.168.0.2
```

## ⚠️ Troubleshooting

**Validation Fails:**
*   Check for duplicate `allowed_nodes` blocks in `config.yaml`.
*   Run `pulse-sensor-proxy config validate 2>&1` for details.

**Lock Errors:**
*   Remove stale locks if process is dead: `rm /etc/pulse-sensor-proxy/*.lock`.

**Empty List:**
*   Valid for IPC-only clusters.
*   Populate manually if needed using `--replace`.
