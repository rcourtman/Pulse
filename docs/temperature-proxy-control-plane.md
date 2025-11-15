# Pulse Temperature Proxy – Control Plane Sync

## Goals

1. Make `pulse-sensor-proxy` trust Pulse itself instead of scraping `pvecm`/editing `/etc/pve`.
2. Ensure host installers always create a pulse-proxy registration, regardless of socket vs HTTP mode.
3. Keep backwards compatibility: existing `allowed_nodes` entries remain a fallback cache, but the runtime source of truth is Pulse.

## Overview

```
┌─────────────────────┐       HTTPS / Unix socket       ┌─────────────────────┐
│ Pulse server (LXC)  │ <═════════════════════════════> │ pulse-sensor-proxy  │
│                     │           /api/...              │ (Proxmox host)      │
│ - Stores nodes      │                                  │ - Collects temps    │
│ - Issues proxy token│                                  │ - Validates node    │
└─────────────────────┘                                  │   via synced list   │
                                                        └─────────────────────┘
```

1. Installer registers the proxy using `/api/temperature-proxy/register`.
   - Response now includes `ctrl_token`, `instance_id`, and `allowed_nodes`.
   - Pulse persists `{instance_id, ctrl_token, last_seen, allowed_nodes_cache}`.
2. Proxy writes:
   ```yaml
   pulse_control_plane:
     url: https://pulse.example.com:7655
     token_file: /etc/pulse-sensor-proxy/.pulse-control-token
     refresh_interval: 60s
   ```
3. Proxy boot sequence:
   - Load cached `allowed_nodes` from YAML (fallback only).
   - If `pulse_control_plane` configured, fetch `/api/temperature-proxy/authorized-nodes`.
   - Replace in-memory allowlist atomically, log version/hash.
   - Retry based on exponential backoff; stay on cached list if control plane unreachable.

## API Changes (Pulse)

1. **Extend existing registration endpoint**
   - Request: `{hostname, proxy_url, kind}` (`kind` = `socket` or `http`).
   - Response: `{success, token, ctrl_token, pve_instance, allowed_nodes, refresh_interval}`.
   - Persist `ctrl_token` (or reuse `TemperatureProxyToken` field if `proxy_url` empty).
2. **New endpoint** `/api/temperature-proxy/authorized-nodes`
   - Auth: `X-Proxy-Token: <ctrl_token>` or `Authorization: Bearer`.
   - Response:
     ```json
     {
       "nodes": [
         {"name": "delly", "ip": "192.168.0.5"},
         {"name": "minipc", "ip": "192.168.0.134"}
       ],
       "hash": "sha256:...",
       "refresh_interval": 60,
       "updated_at": "2025-11-15T20:47:00Z"
     }
     ```
   - Uses Pulse config (`nodes.enc` + cluster endpoints) to build list.
   - Derives `ip` from cluster endpoints or stored host value; duplicates removed.
   - Logs when proxies pull list (metrics + last_seen).
3. **Persistence**
   - `config.PVEInstance` already has `TemperatureProxyURL`/`Token`. Add `TemperatureProxyControlToken` or reuse existing field when URL empty.
   - Add `LastProxyPull`, `LastAllowlistHash`.
4. **Access control**
   - Router should treat `/api/temperature-proxy/authorized-nodes` as public but requiring proxy token (bypasses user auth).
   - Rate limit per proxy (maybe 12/min).

## Proxy Changes

1. **Config additions**
   ```yaml
   pulse_control_plane:
     url: https://pulse.lan:7655
     token_file: /etc/pulse-sensor-proxy/.pulse-control-token
     refresh_interval: 60s   # default
     insecure_skip_verify: false
   ```
2. **Startup**
   - Read token from `token_file`.
   - Launch goroutine: `syncAllowlist(ctx)` loops:
     1. GET `/api/temperature-proxy/authorized-nodes`.
     2. Validate response (non-empty, verify hash changes).
     3. Replace `nodeValidator` allowlist in thread-safe way.
     4. Write new snapshot to `allowed_nodes_cache` (optional).
     5. Sleep `refresh_interval` (server-provided).
   - If call fails: log warning, keep last known list, use fallback allowlist when empty.
3. **NodeValidator**
   - Keep ability to parse static `allowed_nodes`.
   - Add `SetAuthorizedNodes([]string)` to update hosts + CIDRs.
   - When `hasAllowlist == false` but control-plane sync enabled, we never fall back to cluster detection.
   - Provide metrics: last sync success timestamp, number of nodes, etc.

## Installer Changes

1. Host install path (`install.sh` invoking `install-sensor-proxy.sh`)
   - Always pass `--pulse-server http://<container-ip>:<port>`.
   - If `--pulse-server` not supplied manually, `install-sensor-proxy.sh` fetches from `PULSE_SERVER` env.
2. `install-sensor-proxy.sh`
   - After downloading binary, run registration:
     ```
     ctrl_token=$(register_with_pulse "$PULSE_SERVER" "$SHORT_HOSTNAME" "$PROXY_URL" "$MODE")
     echo "$ctrl_token" > /etc/pulse-sensor-proxy/.pulse-control-token
     ```
   - Append control-plane block to config if not present.
   - After install, call new authorized-nodes endpoint once to prime the cache.
   - Continue merging `allowed_nodes` for fallback, but treat as `# Legacy fallback`.
3. Provide migration flag `--legacy-allowlist` to skip control plane (for air-gapped hosts).

## Migration Plan

1. Ship allowlist merge fix (already done locally) so reruns stop causing YAML errors.
2. Release intermediate version where installer accepts `--pulse-server` and registers proxies; proxy ignores new config fields until next release.
3. Release proxy with control-plane sync; ensure it tolerates missing control block (for older installs).
4. Update docs + UI to show last proxy sync state (diagnostics tab).

## Open Questions / TODO

- Decide whether ctrl_token reuses `TemperatureProxyToken` (rename field) or is separate.
- How to handle multiple Pulse servers controlling the same host (future?). For now, one ctrl token per PVE instance.
- Should HTTP-mode proxies reuse the same sync endpoint (yes).

