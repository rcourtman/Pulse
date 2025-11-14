# pulse-sensor-proxy

The sensor proxy keeps SSH identities and temperature polling logic on the
Proxmox host while presenting a small RPC surface (Unix socket or HTTPS) to the
Pulse server. It protects SSH keys from container breakouts, enforces per-UID
capabilities, and produces append-only audit logs.

## Installation Options

| Scenario | Command |
| --- | --- |
| **Recommended (automated)** | `curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-sensor-proxy.sh \| bash` |
| **Manual build** | `go build ./cmd/pulse-sensor-proxy` and `sudo install -m 0755 pulse-sensor-proxy /usr/local/bin/` |
| **Prebuilt artifact** | Copy the binary from `/opt/pulse/bin/pulse-sensor-proxy-*` inside the Pulse Docker image or download via `/download/pulse-sensor-proxy?platform=linux&arch=amd64`. |

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
`scripts/setup-log-forwarding.sh` so audit data lands in your SIEM.

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
| `Cannot open audit log file` | Check permissions on `/var/log/pulse/sensor-proxy`. Remove `chattr +a` only during rotation. |
| `connection denied` in audit log | UID/GID not listed in `allowed_peers`. Verify Pulse container UID mapping. |
| `HTTP request from unauthorized source IP` | Update `allowed_source_subnets` or run through a reverse proxy that advertises the client IP via `ProxyProtocol` (not supported yet). |
| `rate limit exceeded` | Increase `rate_limit.per_peer_burst` or fix noisy hosts before relaxing limits. |
| `temperature pollers stuck` | Hit `/api/monitoring/scheduler/health`, ensure breakers are `closed`, restart Pulse + proxy if necessary. |

For additional hardening steps, read `docs/PULSE_SENSOR_PROXY_HARDENING.md` and
`docs/TEMPERATURE_MONITORING_SECURITY.md`.
