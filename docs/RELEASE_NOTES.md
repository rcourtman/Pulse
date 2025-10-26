# Pulse v4.26.0

## What's Changed
### New
- Standalone host agents now ship with guided Linux, macOS, and Windows installers that stream registration status back to Pulse, generate scoped commands from **Settings → Agents**, and feed host metrics into alerts alongside Proxmox and Docker.
- Alert thresholds gained host-level overrides, connectivity toggles, and snapshot size guardrails so you can tune offline behaviour per host while keeping a global policy for other resources.
- API tokens now support fine-grained scopes with a redesigned manager that previews command templates, highlights unused credentials, and makes revocation a single click.
- Proxmox replication jobs surface in a dedicated **Settings → Hosts → Replication** view with API plumbing to track task health and bubble failures into the monitoring pipeline.
- Docker Swarm environments now receive service/task-aware reporting with configurable scope, plus a Docker settings view that highlights manager/worker roles, stack health, rollout status, and service alert thresholds.

### Improvements
- Dashboard loads and drawer links respond faster thanks to cached guest metadata, reduced polling allocations, and inline URL editing that no longer flashes on WebSocket updates.
- Settings navigation is reorganized with dedicated Docker and Hosts sections, richer filters, and platform icons that make agent onboarding and discovery workflows clearer.
- LXC guests now report dynamic interface IPs, configuration metadata, and queue metrics so alerting, discovery, and drawers stay accurate even during rapid container churn.
- Notifications consolidate into a consistent toast system, with clearer feedback during agent setup, token generation, and background job state changes.

### Bug Fixes
- Enforced explicit node naming and respected custom Proxmox ports so cluster discovery, overrides, and disk monitoring defaults remain intact after edits.
- Hardened setup-token flows and checksum handling in the installers to prevent stale credentials and guarantee the correct binaries are fetched.
- Treated 501 responses from the Proxmox API as non-fatal during failover, restored FreeBSD disk counter parsing, and stopped guest link icons from re-triggering animations on updates.
- Preserved inline editor state across WebSocket refreshes and ensured Docker host identifiers stay collision-safe in mixed environments.

## Installation
- **Install or upgrade with the helper script**
  ```bash
  curl -sL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | bash
  ```
- **Binary upgrade on systemd hosts**
  ```bash
  sudo systemctl stop pulse
  curl -fsSL https://github.com/rcourtman/Pulse/releases/download/v4.26.0/pulse-v4.26.0-linux-amd64.tar.gz \
    | sudo tar -xz -C /opt/pulse --strip-components=1
  sudo systemctl start pulse
  ```
- **Docker update**
  ```bash
  docker pull rcourtman/pulse:v4.26.0
  docker stop pulse || true
  docker rm pulse || true
  docker run -d --name pulse --restart unless-stopped -p 7655:7655 rcourtman/pulse:v4.26.0
  ```
- **Helm upgrade**
  ```bash
  helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \
    --version 4.26.0 \
    --namespace pulse --create-namespace
  ```

## Downloads
- Multi-arch Linux tarballs (amd64/arm64/armv7)
- Standalone sensor proxy binaries
- Helm chart archive (pulse-4.26.0-helm.tgz)
- SHA256 checksums (checksums.txt)
- Docker tags: rcourtman/pulse:v4.26.0, :4.26, :4, :latest
