# ⚙️ Sensor Proxy Configuration

> **Deprecated in v5:** `pulse-sensor-proxy` is deprecated and not recommended for new deployments.
> Use `pulse-agent --enable-proxmox` for temperature monitoring.
> This document is retained for existing installations during the migration window.

Safe configuration management using the built-in CLI.

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
