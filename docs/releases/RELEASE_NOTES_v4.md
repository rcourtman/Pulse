# Release Notes (v4 archive)

This file archives the v4-era release notes that previously lived at `docs/RELEASE_NOTES.md`.

For current releases, refer to GitHub Releases:
<https://github.com/rcourtman/Pulse/releases>

---

## Pulse v4.31.0

### What's Changed (v4.31.0)

#### Temperature monitoring over HTTPS
- `pulse-sensor-proxy` now exposes an authenticated HTTPS endpoint per Proxmox host. Pulse stores each proxy’s URL + bearer token and always polls `https://node:8443/temps` before falling back to local sockets or SSH, eliminating the fragile “single proxy for every node” chain.
- Installations auto-register via the new `/api/temperature-proxy/register` endpoint, generate 4096-bit certificates, enforce CIDR allowlists, and log every HTTP request through the proxy’s audit pipeline.
- The backend temperature collector understands proxy URLs/tokens, respects strict timeouts, and publishes richer diagnostics so operators can see which node failed and why.

#### Installer, diagnostics, and UI updates
- `scripts/install-sensor-proxy.sh` gained `--http-mode` / `--http-addr`, automatic TLS generation, rollback-on-failure, allowed subnet auto-population, and a comprehensive uninstall path that purges sockets, TLS secrets, and LXC bind mounts.
- A new `Settings → Diagnostics → Temperature Proxy` table surfaces proxy health, registration status, and the errors returned by the HTTPS endpoint.
- `scripts/tests/test-sensor-proxy-http.sh` exercises the HTTP installer path end-to-end inside Docker to prevent regressions.

#### Host agent refinements
- Windows PowerShell installers/uninstallers now log verbosely, harden permissions, and clean up services more reliably.
- Linux host-agent scripts aligned with the new diagnostics UX and scoped token workflow so onboarding is less error-prone.

### Upgrade Notes (v4.31.0)

Temperature monitoring will not work for remote nodes until every Proxmox host is reinstalled with the new HTTPS workflow. Follow these steps per host:

```bash
# 1. Remove any pre-v4.31.0 proxy install
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | \
  sudo bash -s -- --uninstall --purge

# 2. Install the HTTP-enabled proxy and register it with Pulse
curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install-sensor-proxy.sh | \
  sudo bash -s -- --standalone --http-mode --pulse-server https://your-pulse-host:7655
```

Only the Pulse server (or container host) needs network access to TCP/8443 on each node. After reinstalling, open **Settings → Diagnostics → Temperature Proxy** to confirm each node reports “HTTPS proxy healthy”. If not, grab the diagnostics entry or run:

```bash
curl -vk https://node.example:8443/health \
  -H "Authorization: Bearer $(sudo cat /etc/pulse-sensor-proxy/.http-auth-token)"
```

### Installation (v4.31.0)
- **Install or upgrade with the helper script**
  ```bash
  curl -sL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
  ```
- **Binary upgrade on systemd hosts**
  ```bash
  sudo systemctl stop pulse
  curl -fsSL https://github.com/rcourtman/Pulse/releases/download/v4.31.0/pulse-v4.31.0-linux-amd64.tar.gz \
    | sudo tar -xz -C /opt/pulse --strip-components=1
  sudo systemctl start pulse
  ```
- **Docker update**
  ```bash
  docker pull rcourtman/pulse:v4.31.0
  docker stop pulse || true
  docker rm pulse || true
  docker run -d --name pulse --restart unless-stopped -p 7655:7655 rcourtman/pulse:v4.31.0
  ```
- **Helm upgrade**
  ```bash
  helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \
    --version 4.31.0 \
    --namespace pulse --create-namespace
  ```

### Downloads (v4.31.0)
- Multi-arch Linux tarballs (amd64/arm64/armv7)
- Standalone sensor proxy binaries (now include HTTP mode)
- Helm chart archive (pulse-4.31.0-helm.tgz)
- SHA256 checksums (checksums.txt)
- Docker tags: rcourtman/pulse:v4.31.0, :4.31, :4, :latest

---

## Pulse v4.26.1

### What's Changed (v4.26.1)
#### New
- Standalone host agents now ship with guided Linux, macOS, and Windows installers that stream registration status back to Pulse, generate scoped commands from **Settings → Unified Agents**, and feed host metrics into alerts alongside Proxmox and Docker.
- Alert thresholds gained host-level overrides, connectivity toggles, and snapshot size guardrails so you can tune offline behaviour per host while keeping a global policy for other resources.
- API tokens now support fine-grained scopes with a redesigned manager that previews command templates, highlights unused credentials, and makes revocation a single click.
- Proxmox replication jobs surface in a dedicated **Proxmox → Replication** view with API plumbing to track task health and bubble failures into the monitoring pipeline.
- Docker Swarm environments now receive service/task-aware reporting with configurable scope, plus a Docker settings view that highlights manager/worker roles, stack health, rollout status, and service alert thresholds.

#### Improvements
- Dashboard loads and drawer links respond faster thanks to cached guest metadata, reduced polling allocations, and inline URL editing that no longer flashes on WebSocket updates.
- Settings navigation is reorganized with dedicated platform and agent sections, richer filters, and platform icons that make onboarding and discovery workflows clearer.
- LXC guests now report dynamic interface IPs, configuration metadata, and queue metrics so alerting, discovery, and drawers stay accurate even during rapid container churn.
- Notifications consolidate into a consistent toast system, with clearer feedback during agent setup, token generation, and background job state changes.

#### Bug Fixes
- Enforced explicit node naming and respected custom Proxmox ports so cluster discovery, overrides, and disk monitoring defaults remain intact after edits.
- Hardened setup-token flows and checksum handling in the installers to prevent stale credentials and guarantee the correct binaries are fetched.
- Treated 501 responses from the Proxmox API as non-fatal during failover, restored FreeBSD disk counter parsing, and stopped guest link icons from re-triggering animations on updates.
- Preserved inline editor state across WebSocket refreshes and ensured Docker host identifiers stay collision-safe in mixed environments.

### Installation (v4.26.1)
- **Install or upgrade with the helper script**
  ```bash
  curl -sL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
  ```
- **Binary upgrade on systemd hosts**
  ```bash
  sudo systemctl stop pulse
  curl -fsSL https://github.com/rcourtman/Pulse/releases/download/v4.26.1/pulse-v4.26.1-linux-amd64.tar.gz \
    | sudo tar -xz -C /opt/pulse --strip-components=1
  sudo systemctl start pulse
  ```
- **Docker update**
  ```bash
  docker pull rcourtman/pulse:v4.26.1
  docker stop pulse || true
  docker rm pulse || true
  docker run -d --name pulse --restart unless-stopped -p 7655:7655 rcourtman/pulse:v4.26.1
  ```
- **Helm upgrade**
  ```bash
  helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \
    --version 4.26.1 \
    --namespace pulse --create-namespace
  ```

### Downloads (v4.26.1)
- Multi-arch Linux tarballs (amd64/arm64/armv7)
- Standalone sensor proxy binaries
- Helm chart archive (pulse-4.26.1-helm.tgz)
- SHA256 checksums (checksums.txt)
- Docker tags: rcourtman/pulse:v4.26.1, :4.26, :4, :latest
