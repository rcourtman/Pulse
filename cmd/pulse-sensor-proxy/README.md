# pulse-sensor-proxy

> **Deprecated in v5:** `pulse-sensor-proxy` is deprecated and not recommended for new deployments.
> Temperature monitoring should be done via the unified agent (`pulse-agent --enable-proxmox`).
> This README is retained for existing installations during the migration window.
> See `docs/TEMPERATURE_MONITORING.md`.

The sensor proxy keeps SSH identities and temperature polling logic on the
Proxmox host while presenting a small RPC surface (Unix socket or HTTPS) to the
Pulse server. It protects SSH keys from container breakouts, enforces per-UID
capabilities, and produces append-only audit logs.

## Installation Options

| Scenario | Command |
| --- | --- |
| **Recommended (automated)** | `curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh \| bash` |
| **Manual build** | `go build ./cmd/pulse-sensor-proxy` and `sudo install -m 0755 pulse-sensor-proxy /usr/local/bin/` |
| **Prebuilt artifact** | Download `pulse-sensor-proxy-<platform>-<arch>` from GitHub Releases, or copy it from `/opt/pulse/bin/` inside the Pulse Docker image. |

The installer script provisions:

- User & group: `pulse-sensor-proxy`
- Binary: `/usr/local/bin/pulse-sensor-proxy`
- Config: `/etc/pulse-sensor-proxy/config.yaml`
- SSH material: `/var/lib/pulse-sensor-proxy/ssh`
- Socket: `/run/pulse-sensor-proxy/pulse-sensor-proxy.sock`
- Logs: `/var/log/pulse/sensor-proxy/{proxy.log,audit.log}` (append-only)
- Systemd units: `pulse-sensor-proxy.service`, cleanup + self-heal timers

Start the service and verify status:

```bash
systemctl enable --now pulse-sensor-proxy
systemctl status pulse-sensor-proxy --no-pager
journalctl -u pulse-sensor-proxy -n 50
```

## Configuration

The proxy reads `/etc/pulse-sensor-proxy/config.yaml` (see
`config.example.yaml`). Key fields:

| Key | Purpose | Notes |
| --- | --- | --- |
| `allowed_source_subnets` | Restrict peers by CIDR | Empty list = auto-detect host networks |
| `allowed_peers[].uid/gid` | Capability-scoped authorisation | Prefer over legacy `allowed_peer_uids`|
| `allowed_peers[].capabilities` | `read`, `write`, `admin` | `read` covers `get_temperature`; `admin` required for `ensure_cluster_keys` |
| `metrics_address` | Prometheus listener | Default `127.0.0.1:9127`; set `disabled` to turn off |
| `require_proxmox_hostkeys` | Enforce known-host matches | Protects against SSH MITM |
| `max_ssh_output_bytes` | Cap command output | Prevents memory exhaustion (default 1 MiB) |
| `rate_limit.per_peer_interval_ms` / `per_peer_burst` | Token bucket guardrails | Keep interval ≥100 ms in production |
| `http_*` keys | HTTPS bridge mode | Needs TLS files plus bearer token |
| `allowed_nodes_file` | Path to allowed nodes list | Default: `/etc/pulse-sensor-proxy/allowed_nodes.yaml` |

### Configuration Management CLI

The proxy includes built-in commands for safe configuration management. These prevent corruption by using atomic writes and file locking.

**Validate configuration:**
```bash
# Validate config.yaml and allowed_nodes.yaml
pulse-sensor-proxy config validate

# Validate specific config file
pulse-sensor-proxy config validate --config /path/to/config.yaml

# Validate specific allowed_nodes file
pulse-sensor-proxy config validate --allowed-nodes /path/to/allowed_nodes.yaml
```

**Manage allowed nodes:**
```bash
# Add nodes to the allowed list (merge mode)
pulse-sensor-proxy config set-allowed-nodes --merge 192.168.0.1 --merge node1.local

# Replace entire list with new nodes
pulse-sensor-proxy config set-allowed-nodes --replace --merge 192.168.0.1 --merge 192.168.0.2

# Clear the allowed nodes list (replace with empty)
pulse-sensor-proxy config set-allowed-nodes --replace

# Use custom path
pulse-sensor-proxy config set-allowed-nodes --allowed-nodes /custom/path.yaml --merge 192.168.0.10
```

**How it works:**
- All writes are atomic (temp file + rename)
- File locking prevents concurrent modifications
- Deduplication and normalization happen automatically
- Empty lists are allowed (useful for security lockdown or IPC-only clusters)
- Config validation runs before service startup (systemd ExecStartPre)

**Best practices:**
- Use the CLI instead of manual editing whenever possible
- The installer automatically uses these commands
- Manual edits to `config.yaml` are safe if the service is stopped
- Never edit `allowed_nodes.yaml` while the service is running

### Allowed Nodes File

The proxy maintains a separate YAML file for the authorized node list at
`/etc/pulse-sensor-proxy/allowed_nodes.yaml`. This separation prevents
config corruption when the installer or control-plane sync updates the list.

Format:
```yaml
# Managed by pulse-sensor-proxy config CLI
# Do not edit manually while service is running
allowed_nodes:
  - 192.168.0.1
  - 192.168.0.2
  - node1.local
  - node2.example.com
```

The file is optional - if missing or empty, the proxy falls back to IPC-based
discovery (pvecm status) when available.

### Environment Overrides

- `PULSE_SENSOR_PROXY_SOCKET`, `PULSE_SENSOR_PROXY_SSH_DIR`,
  `PULSE_SENSOR_PROXY_CONFIG` – relocate runtime paths
- `PULSE_SENSOR_PROXY_USER` – run under a different service account (defaults to
  `pulse-sensor-proxy`)
- `PULSE_SENSOR_PROXY_ALLOWED_SUBNETS` – comma-separated list appended at boot
- `PULSE_SENSOR_PROXY_ALLOWED_PEER_UIDS/GIDS` – extend authorisation without
  editing YAML
- `PULSE_SENSOR_PROXY_ALLOW_IDMAPPED_ROOT` – explicitly allow/deny ID-mapped root
- `PULSE_SENSOR_PROXY_READ_TIMEOUT` / `_WRITE_TIMEOUT` – Go duration strings
- `PULSE_SENSOR_PROXY_AUDIT_LOG` – custom log path (still append-only)

## HTTP Mode

Set `http_enabled: true` when the backend cannot mount the Unix socket (for
example, Kubernetes). Requirements:

1. Populate `http_listen_addr` (e.g. `0.0.0.0:9443`).
2. Provide `http_tls_cert`/`http_tls_key`. The installer can place certs under
   `/etc/pulse-sensor-proxy/tls`.
3. Set a long `http_auth_token`. Pulse sends it as a bearer token.
4. Restrict `allowed_source_subnets` to the Pulse control-plane addresses.

The HTTP server exports `/temps` and `/health`, enforces Bearer tokens, and logs
all HTTP access attempts to the audit log.

## Audit Logging & Rotation

- Location: `/var/log/pulse/sensor-proxy/audit.log`
- Format: JSON with hash chaining (`prev_hash`, `event_hash`, `seq`)
- Access: Owned by `pulse-sensor-proxy`, `0640`, `chattr +a`

Follow `docs/operations/audit-log-rotation.md` for rotation (remove `+a`,
truncate, restart service, reapply `+a`). Also consider forwarding with
`scripts/setup-log-forwarding.sh`; see
`docs/operations/sensor-proxy-log-forwarding.md` for RELP/TLS forwarding
instructions and verification steps.

## Metrics & Monitoring

| Signal | Command |
| --- | --- |
| Prometheus metrics | `curl -s http://127.0.0.1:9127/metrics \| head` |
| Scheduler health (Pulse) | `curl -s http://localhost:7655/api/monitoring/scheduler/health \| jq '.instances[] \| select(.key \| contains(\"temperature\")) \| {key, breaker: .breaker.state, deadLetter: .deadLetter.present}'` |
| Journal logs | `journalctl -u pulse-sensor-proxy -f` |
| Rate-limit hits | `journalctl -u pulse-sensor-proxy \| grep "rate limit"` |

Set alerts on:

- `pulse_proxy_rate_limit_hits_total` spikes (potential abuse)
- `pulse_proxy_hostkey_changes_total` increments (SSH MITM)
- Temperature instances showing `breaker.state != "closed"` for >10 minutes

## Troubleshooting

| Symptom | Guidance |
| --- | --- |
| Service fails to start with "Config validation failed" | Run `pulse-sensor-proxy config validate` to see specific errors. Check for duplicate keys or malformed YAML. |
| Config corruption detected during startup | Older versions had dual code paths. Update to the latest release and reinstall the proxy. The migration runs automatically. |
| Temperature monitoring stops working after config change | Validate config first with `pulse-sensor-proxy config validate`, then restart service: `systemctl restart pulse-sensor-proxy`. |
| `Cannot open audit log file` | Check permissions on `/var/log/pulse/sensor-proxy`. Remove `chattr +a` only during rotation. |
| `connection denied` in audit log | UID/GID not listed in `allowed_peers`. Verify Pulse container UID mapping. |
| `HTTP request from unauthorized source IP` | Update `allowed_source_subnets` or run through a reverse proxy that advertises the client IP via `ProxyProtocol` (not supported yet). |
| `rate limit exceeded` | Increase `rate_limit.per_peer_burst` or fix noisy hosts before relaxing limits. |
| `temperature pollers stuck` | Hit `/api/monitoring/scheduler/health`, ensure breakers are `closed`, restart Pulse + proxy if necessary. |
| Lock file permissions error | Lock files use 0600 to prevent unprivileged DoS. Check file ownership matches proxy user. |

### Config Corruption Recovery

If you suspect config corruption (service won't start, temperatures stopped):

1. **Validate the config:**
   ```bash
   pulse-sensor-proxy config validate
   ```

2. **If corruption is detected, reinstall the proxy:**
   ```bash
   curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | \
     sudo bash -s -- --standalone --pulse-server http://your-pulse:7655
   ```
   The installer automatically migrates to file-based config and fixes corruption.

3. **Check for duplicate allowed_nodes blocks:**
   ```bash
   grep -n "allowed_nodes:" /etc/pulse-sensor-proxy/config.yaml
   ```
   Should only appear once. Multiple instances indicate corruption that Phase 1 migration will fix.

4. **Manual recovery (if installer unavailable):**
   ```bash
   # Stop the service
   sudo systemctl stop pulse-sensor-proxy

   # Validate and identify issues
   pulse-sensor-proxy config validate --config /etc/pulse-sensor-proxy/config.yaml

   # If allowed_nodes appears in config.yaml, extract it manually:
   grep -A 100 "^allowed_nodes:" /etc/pulse-sensor-proxy/config.yaml | \
     head -n 20 > /tmp/nodes.txt

   # Remove duplicate allowed_nodes from config.yaml (edit manually)
   # Then create allowed_nodes.yaml:
   pulse-sensor-proxy config set-allowed-nodes --replace --merge node1 --merge node2

   # Add allowed_nodes_file reference to config.yaml if missing:
   echo "allowed_nodes_file: /etc/pulse-sensor-proxy/allowed_nodes.yaml" | \
     sudo tee -a /etc/pulse-sensor-proxy/config.yaml

   # Validate again
   pulse-sensor-proxy config validate

   # Start service
   sudo systemctl start pulse-sensor-proxy
   ```

For additional hardening steps, read `docs/PULSE_SENSOR_PROXY_HARDENING.md` and
`docs/TEMPERATURE_MONITORING_SECURITY.md`.
