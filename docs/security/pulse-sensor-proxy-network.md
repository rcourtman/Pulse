# Pulse Sensor Proxy Network Segmentation

## Overview
- **Proxy host** collects temperatures via SSH from Proxmox nodes and serves a Unix socket to the Pulse stack.
- Goals: isolate the proxy from production hypervisors, prevent lateral movement, and ensure log forwarding/audit channels remain available.

## Zones & Connectivity
- **Pulse Application Zone (AZ-Pulse)**
  - Hosts Pulse backend/frontend containers.
  - Allowed to reach the proxy over Unix socket (local) or loopback if containerised via `socat`.
- **Sensor Proxy Zone (AZ-Sensor)**
  - Dedicated VM/bare-metal host running `pulse-sensor-proxy`.
  - Maintains outbound SSH to Proxmox management interfaces only.
- **Proxmox Management Zone (AZ-Proxmox)**
  - Hypervisors / BMCs reachable on `tcp/22` (SSH) and optional IPMI UDP.
- **Logging/Monitoring Zone (AZ-Logging)**
  - Receives forwarded audit/application logs (e.g. RELP/TLS on `tcp/6514`).
  - Exposes Prometheus scrape port (default `tcp/9456`) if remote monitoring required.

## Recommended Firewall Rules

| Source Zone | Destination Zone | Protocol/Port | Purpose | Action |
|-------------|------------------|---------------|---------|--------|
| AZ-Pulse (localhost) | AZ-Sensor (Unix socket) | `unix` | RPC requests from Pulse | Allow (local only) |
| AZ-Sensor | AZ-Proxmox nodes | `tcp/22` | SSH for sensors/ipmitool wrapper | Allow (restricted to node list) |
| AZ-Sensor | AZ-Proxmox BMC | `udp/623` *(optional)* | IPMI if required for temperature data | Allow if needed |
| AZ-Proxmox | AZ-Sensor | `any` | Return SSH traffic | Allow stateful |
| AZ-Sensor | AZ-Logging | `tcp/6514` (TLS RELP) | Audit/application log forwarding | Allow |
| AZ-Logging | AZ-Sensor | `tcp/9456` *(optional)* | Prometheus scrape of proxy metrics | Allow if scraping remotely |
| Any | AZ-Sensor | `tcp/22` | Shell/SSH access | Deny (use management bastion) |
| AZ-Sensor | Internet | `any` | Outbound Internet | Deny (except package mirrors via proxy if required) |

## Implementation Steps
1. Place proxy host in dedicated subnet/VLAN with ACLs enforcing the table above.
2. Populate `/etc/hosts` or routing so proxy resolves Proxmox nodes to management IPs only (no public networks).
3. Configure iptables/nftables on proxy:
   ```bash
   # Allow SSH to Proxmox nodes
   iptables -A OUTPUT -p tcp -d <PROXMOX_SUBNET>/24 --dport 22 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT
   iptables -A INPUT -p tcp -s <PROXMOX_SUBNET>/24 --sport 22 -m conntrack --ctstate ESTABLISHED -j ACCEPT

   # Allow log forwarding
   iptables -A OUTPUT -p tcp -d <LOG_HOST> --dport 6514 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT
   iptables -A INPUT -p tcp -s <LOG_HOST> --sport 6514 -m conntrack --ctstate ESTABLISHED -j ACCEPT

   # (Optional) allow Prometheus scrape
   iptables -A INPUT -p tcp -s <SCRAPE_HOST> --dport 9456 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT
   iptables -A OUTPUT -p tcp -d <SCRAPE_HOST> --sport 9456 -m conntrack --ctstate ESTABLISHED -j ACCEPT

   # Drop everything else
   iptables -P OUTPUT DROP
   iptables -P INPUT DROP
   ```
4. Deny inbound SSH to proxy except via management bastion: block `tcp/22` or whitelist bastion IPs.
5. Ensure log-forwarding TLS certificates are rotated and stored under `/etc/pulse/log-forwarding`.

## Monitoring & Alerting
- Alert if proxy initiates connections outside permitted subnets (Netflow or host firewall counters).
- Monitor `pulse_proxy_limiter_*` metrics for unusual rate-limit hits that might signal abuse.
- Track `audit_log` forwarding queue depth and remote availability; on failure, emit alert via rsyslog action queue (set `action.resumeRetryCount=-1` already).

## Change Management
- Document node IP changes and update firewall objects (`PROXMOX_NODES`) before redeploying certificates.
- Capture segmentation in infrastructure-as-code (e.g. Terraform/security group definitions) to avoid drift.
