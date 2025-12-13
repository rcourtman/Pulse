# 🌐 Sensor Proxy Network Segmentation

> **⚠️ Deprecated:** The sensor-proxy is deprecated in favor of the unified Pulse agent.
> For new installations, use `install.sh --enable-proxmox` instead.
> See [TEMPERATURE_MONITORING.md](/docs/security/TEMPERATURE_MONITORING.md).

Isolate the proxy to prevent lateral movement.

## 🚧 Zones
*   **Pulse App**: Connects to Proxy via Unix socket (local).
*   **Sensor Proxy**: Outbound SSH to Proxmox nodes only.
*   **Proxmox Nodes**: Accept SSH from Proxy.
*   **Logging**: Accepts RELP/TLS from Proxy.

## 🛡️ Firewall Rules

| Source | Dest | Port | Purpose | Action |
| :--- | :--- | :--- | :--- | :--- |
| **Pulse App** | Proxy | `unix` | RPC Requests | **Allow** (Local) |
| **Proxy** | Nodes | `22` | SSH (sensors) | **Allow** |
| **Proxy** | Logs | `6514` | Audit Logs | **Allow** |
| **Any** | Proxy | `22` | SSH Access | **Deny** (Use Bastion) |
| **Proxy** | Internet | `any` | Outbound | **Deny** |

## 🔧 Implementation (iptables)
```bash
# Allow SSH to Proxmox
iptables -A OUTPUT -p tcp -d <PROXMOX_SUBNET> --dport 22 -j ACCEPT

# Allow Log Forwarding
iptables -A OUTPUT -p tcp -d <LOG_HOST> --dport 6514 -j ACCEPT

# Drop all other outbound
iptables -P OUTPUT DROP
```

## 🚨 Monitoring
*   Alert on outbound connections to non-whitelisted IPs.
*   Monitor `pulse_proxy_limiter_rejects_total` for abuse.
