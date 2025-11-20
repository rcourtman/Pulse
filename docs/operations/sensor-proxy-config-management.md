# Sensor Proxy Configuration Management

This guide covers safe configuration management for pulse-sensor-proxy, including the new CLI tools introduced in v4.31.1+ to prevent config corruption.

## Overview

Starting with v4.31.1, pulse-sensor-proxy uses a two-file configuration system:

1. **Main config:** `/etc/pulse-sensor-proxy/config.yaml` - Contains all settings except allowed nodes
2. **Allowed nodes:** `/etc/pulse-sensor-proxy/allowed_nodes.yaml` - Separate file for the authorized node list

This separation prevents corruption from concurrent updates by the installer, control-plane sync, and self-heal timer.

## Architecture

### Why Two Files?

Earlier versions stored `allowed_nodes:` inline in `config.yaml`, causing corruption when:
- The installer updated node lists
- The self-heal timer ran (every 5 minutes)
- Control-plane sync modified the list
- Version detection had edge cases

Multiple code paths (shell, Python, Go) would race to update the same YAML file, creating duplicate `allowed_nodes:` keys that broke YAML parsing.

### New System (v4.31.1+)

**Phase 1 (Migration):**
- Force file-based mode exclusively
- Installer migrates inline blocks to `allowed_nodes.yaml`
- Self-heal timer includes corruption detection and repair

**Phase 2 (Atomic Operations):**
- Go CLI replaces all shell/Python config manipulation
- File locking prevents concurrent writes
- Atomic writes (temp file + rename) ensure consistency
- systemd validation prevents startup with corrupt config

## Configuration CLI Reference

### Validate Configuration

Check config files for errors before restarting the service:

```bash
# Validate both config.yaml and allowed_nodes.yaml
pulse-sensor-proxy config validate

# Validate specific config file
pulse-sensor-proxy config validate --config /path/to/config.yaml

# Validate specific allowed_nodes file
pulse-sensor-proxy config validate --allowed-nodes /path/to/allowed_nodes.yaml
```

**Exit codes:**
- 0 = valid
- Non-zero = validation failed (check stderr for details)

**Common validation errors:**
- "duplicate allowed_nodes blocks" - Run migration (see below)
- "failed to parse YAML" - Syntax error in config file
- "read_timeout must be positive" - Invalid timeout value

### Manage Allowed Nodes

The CLI provides two modes:

**Merge mode (default):** Adds nodes to existing list
```bash
# Add single node
pulse-sensor-proxy config set-allowed-nodes --merge 192.168.0.10

# Add multiple nodes
pulse-sensor-proxy config set-allowed-nodes \
  --merge 192.168.0.1 \
  --merge 192.168.0.2 \
  --merge node1.local
```

**Replace mode:** Overwrites entire list
```bash
# Replace with new list
pulse-sensor-proxy config set-allowed-nodes --replace \
  --merge 192.168.0.1 \
  --merge 192.168.0.2

# Clear the list (empty is valid for IPC-only clusters)
pulse-sensor-proxy config set-allowed-nodes --replace
```

**Custom paths:**
```bash
# Use non-default path
pulse-sensor-proxy config set-allowed-nodes \
  --allowed-nodes /custom/path.yaml \
  --merge 192.168.0.10
```

### How It Works

1. **File locking:** Uses `flock(LOCK_EX)` on separate `.lock` file
2. **Atomic writes:** Writes to temp file, syncs, then renames
3. **Deduplication:** Automatically removes duplicate entries
4. **Normalization:** Trims whitespace, sorts entries
5. **Empty lists allowed:** Useful for security lockdown or IPC-based discovery

## Common Tasks

### Adding Nodes After Cluster Expansion

When you add a new node to your Proxmox cluster:

```bash
# Add the new node to allowed list
pulse-sensor-proxy config set-allowed-nodes --merge new-node.local

# Validate config
pulse-sensor-proxy config validate

# Restart proxy to apply
sudo systemctl restart pulse-sensor-proxy

# Verify in Pulse UI
# Check Settings → Diagnostics → Temperature Proxy
```

### Removing Decommissioned Nodes

When removing a node from your cluster:

```bash
# Get current list
cat /etc/pulse-sensor-proxy/allowed_nodes.yaml

# Replace with updated list (without old node)
pulse-sensor-proxy config set-allowed-nodes --replace \
  --merge 192.168.0.1 \
  --merge 192.168.0.2
  # (omit the decommissioned node)

# Validate and restart
pulse-sensor-proxy config validate
sudo systemctl restart pulse-sensor-proxy
```

**Note:** The proxy cleanup system automatically removes SSH keys from deleted nodes. See temperature monitoring docs for details.

### Migrating from Inline Config

If you're running an older version with inline `allowed_nodes:` in config.yaml:

```bash
# Upgrade to latest version (auto-migrates)
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh | \
  sudo bash -s -- --standalone --pulse-server http://your-pulse:7655

# Verify migration
pulse-sensor-proxy config validate

# Check that allowed_nodes only appears in allowed_nodes.yaml
grep -n "allowed_nodes:" /etc/pulse-sensor-proxy/*.yaml
# Should show: allowed_nodes.yaml:3:allowed_nodes:
# Should NOT show duplicate entries in config.yaml
```

### Changing Other Config Settings

For settings in `config.yaml` (not allowed_nodes):

```bash
# Stop the service first
sudo systemctl stop pulse-sensor-proxy

# Edit config.yaml manually
sudo nano /etc/pulse-sensor-proxy/config.yaml

# Validate before starting
pulse-sensor-proxy config validate

# Start service
sudo systemctl start pulse-sensor-proxy

# Check for errors
sudo systemctl status pulse-sensor-proxy
journalctl -u pulse-sensor-proxy -n 50
```

**Safe to edit in config.yaml:**
- `allowed_source_subnets`
- `allowed_peers` (UID/GID permissions)
- `rate_limit` settings
- `metrics_address`
- `http_*` settings (HTTPS mode)
- `pulse_control_plane` block

**Never edit manually:**
- `allowed_nodes:` (use CLI instead, or it will be in allowed_nodes.yaml anyway)
- Lock files (`.lock`)

## Troubleshooting

### Config Validation Fails

**Symptom:** `pulse-sensor-proxy config validate` returns error

**Diagnosis:**
```bash
# Run validation with full output
pulse-sensor-proxy config validate 2>&1

# Check for duplicate blocks
grep -n "allowed_nodes:" /etc/pulse-sensor-proxy/config.yaml

# Check YAML syntax
python3 -c "import yaml; yaml.safe_load(open('/etc/pulse-sensor-proxy/config.yaml'))"
```

**Common fixes:**
- Duplicate blocks: Run migration (upgrade to v4.31.1+)
- YAML syntax errors: Fix indentation, remove tabs, check colons
- Missing required fields: Add `read_timeout`, `write_timeout`

### Service Won't Start After Config Change

**Diagnosis:**
```bash
# Check systemd logs
journalctl -u pulse-sensor-proxy -n 100

# Look for validation errors
journalctl -u pulse-sensor-proxy | grep -i "validation\|corrupt\|duplicate"

# Try starting in foreground for better errors
sudo -u pulse-sensor-proxy /opt/pulse/sensor-proxy/bin/pulse-sensor-proxy  # legacy installs: /usr/local/bin/pulse-sensor-proxy
```

**Fix:**
```bash
# Validate config first
pulse-sensor-proxy config validate

# If validation passes but service fails, check permissions
ls -la /etc/pulse-sensor-proxy/
ls -la /var/lib/pulse-sensor-proxy/

# Ensure proxy user owns files
sudo chown -R pulse-sensor-proxy:pulse-sensor-proxy /etc/pulse-sensor-proxy/
sudo chown -R pulse-sensor-proxy:pulse-sensor-proxy /var/lib/pulse-sensor-proxy/
```

### Lock File Errors

**Symptom:** `failed to acquire file lock` or `failed to open lock file`

**Cause:** Lock file has wrong permissions or process holds stale lock

**Fix:**
```bash
# Check lock file permissions (should be 0600)
ls -la /etc/pulse-sensor-proxy/*.lock

# Fix permissions
sudo chmod 0600 /etc/pulse-sensor-proxy/*.lock
sudo chown pulse-sensor-proxy:pulse-sensor-proxy /etc/pulse-sensor-proxy/*.lock

# If stale lock, identify holder
sudo lsof /etc/pulse-sensor-proxy/allowed_nodes.yaml.lock

# Kill stale process if needed (use with caution)
sudo kill <PID>
```

**Prevention:** Locks are automatically released when process exits. Don't manually delete lock files.

### Allowed Nodes List is Empty

**Symptom:** allowed_nodes.yaml exists but has no entries

**Is this a problem?** Not necessarily:
- Empty list is valid for clusters using IPC discovery (pvecm status)
- Control-plane mode populates the list automatically
- Standalone nodes require manual node entries

**To populate manually:**
```bash
# Add your cluster nodes
pulse-sensor-proxy config set-allowed-nodes --replace \
  --merge 192.168.0.1 \
  --merge 192.168.0.2 \
  --merge 192.168.0.3

# Verify
cat /etc/pulse-sensor-proxy/allowed_nodes.yaml
```

## Best Practices

### General Guidelines

1. **Always validate before restarting:**
   ```bash
   pulse-sensor-proxy config validate && sudo systemctl restart pulse-sensor-proxy
   ```

2. **Use the CLI for allowed_nodes changes:**
   - Don't edit `allowed_nodes.yaml` manually
   - Use `config set-allowed-nodes` instead

3. **Stop service before editing config.yaml:**
   - Prevents race conditions with running process
   - systemd validation will catch errors on startup

4. **Back up config before major changes:**
   ```bash
   sudo cp /etc/pulse-sensor-proxy/config.yaml /etc/pulse-sensor-proxy/config.yaml.backup
   sudo cp /etc/pulse-sensor-proxy/allowed_nodes.yaml /etc/pulse-sensor-proxy/allowed_nodes.yaml.backup
   ```

5. **Monitor after changes:**
   ```bash
   journalctl -u pulse-sensor-proxy -f
   # Check Pulse UI: Settings → Diagnostics → Temperature Proxy
   ```

### Automation Scripts

When scripting config changes:

```bash
#!/bin/bash
set -euo pipefail

# Function to safely update allowed nodes
update_allowed_nodes() {
    local nodes=("$@")

    # Build command
    local cmd="pulse-sensor-proxy config set-allowed-nodes --replace"
    for node in "${nodes[@]}"; do
        cmd="$cmd --merge $node"
    done

    # Execute with validation
    if eval "$cmd"; then
        echo "Allowed nodes updated successfully"
    else
        echo "Failed to update allowed nodes" >&2
        return 1
    fi

    # Validate
    if ! pulse-sensor-proxy config validate; then
        echo "Config validation failed after update" >&2
        return 1
    fi

    # Restart service
    if sudo systemctl restart pulse-sensor-proxy; then
        echo "Service restarted successfully"
    else
        echo "Service restart failed" >&2
        return 1
    fi

    # Wait for service to be active
    sleep 2
    if systemctl is-active --quiet pulse-sensor-proxy; then
        echo "Service is running"
    else
        echo "Service failed to start" >&2
        journalctl -u pulse-sensor-proxy -n 20
        return 1
    fi
}

# Example usage
update_allowed_nodes "192.168.0.1" "192.168.0.2" "node3.local"
```

### Monitoring Config Health

Add to your monitoring system:

```bash
# Check for config corruption (should return 0)
pulse-sensor-proxy config validate
echo $?

# Check for duplicate blocks (should be empty)
grep "allowed_nodes:" /etc/pulse-sensor-proxy/config.yaml | wc -l

# Check lock file permissions (should be 0600)
stat -c "%a" /etc/pulse-sensor-proxy/*.lock

# Check service is running
systemctl is-active pulse-sensor-proxy
```

## Migration Path

### Upgrading from Pre-v4.31.1

**Automatic migration** (recommended):
```bash
# Simply reinstall - migration runs automatically
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh | \
  sudo bash -s -- --standalone --pulse-server http://your-pulse:7655

# Verify
pulse-sensor-proxy config validate
sudo systemctl status pulse-sensor-proxy
```

**Manual migration** (if needed):
```bash
# 1. Stop service
sudo systemctl stop pulse-sensor-proxy

# 2. Extract allowed_nodes from config.yaml
grep -A 100 "^allowed_nodes:" /etc/pulse-sensor-proxy/config.yaml > /tmp/nodes.txt

# 3. Parse and add to allowed_nodes.yaml
# (Example for simple list - adjust for your format)
pulse-sensor-proxy config set-allowed-nodes --replace \
  --merge node1.local \
  --merge node2.local

# 4. Remove allowed_nodes from config.yaml
# Edit manually or use sed:
sudo sed -i '/^allowed_nodes:/,/^[a-z_]/d' /etc/pulse-sensor-proxy/config.yaml

# 5. Add reference to allowed_nodes.yaml
echo "allowed_nodes_file: /etc/pulse-sensor-proxy/allowed_nodes.yaml" | \
  sudo tee -a /etc/pulse-sensor-proxy/config.yaml

# 6. Validate
pulse-sensor-proxy config validate

# 7. Start service
sudo systemctl start pulse-sensor-proxy
```

## Related Documentation

- [Temperature Monitoring](../TEMPERATURE_MONITORING.md) - Setup and troubleshooting
- [Sensor Proxy README](/opt/pulse/cmd/pulse-sensor-proxy/README.md) - Complete CLI reference
- [Audit Log Rotation](audit-log-rotation.md) - Managing append-only logs
- [Temperature Monitoring Security](../TEMPERATURE_MONITORING_SECURITY.md) - Security architecture

## Support

If config management issues persist after following this guide:

1. Collect diagnostics:
   ```bash
   pulse-sensor-proxy config validate 2>&1 > /tmp/validate.log
   sudo systemctl status pulse-sensor-proxy > /tmp/status.log
   journalctl -u pulse-sensor-proxy -n 200 > /tmp/journal.log
   grep -n "allowed_nodes:" /etc/pulse-sensor-proxy/*.yaml > /tmp/grep.log
   ```

2. File an issue at https://github.com/rcourtman/Pulse/issues

3. Include:
   - Pulse version
   - Sensor proxy version (`pulse-sensor-proxy --version`)
   - Output from diagnostic commands above
   - Steps that led to the issue
