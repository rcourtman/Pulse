# 🐳 Docker Guide

Pulse is distributed as a lightweight, Alpine-based Docker image.

> **Paid Pulse Pro / Relay / legacy customers:** The public `rcourtman/pulse`
> Docker image is the community build. It can accept an activation key, but it
> does not include the private Pulse Pro runtime hooks. Use
> <https://pulserelay.pro/download.html> with your activation key, then run the
> private registry login and `PULSE_IMAGE=license.pulserelay.pro/pulse-pro:<version>`
> compose commands shown there. Those commands require the compose file image
> line to use the `PULSE_IMAGE` variable, as shown below. If your compose file
> hardcodes `image: rcourtman/pulse:...`, replace that line with the variable
> form or with the private image shown on the download page before restarting.

## 🚀 Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e PULSE_DEPLOYMENT_METHOD=docker_run \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Access at `http://<your-ip>:7655`.

---

## 📦 Docker Compose

Create a `docker-compose.yml` file:

```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - TZ=Europe/London
      - PULSE_DEPLOYMENT_METHOD=docker_compose
      # Optional: Pre-configure auth (skips setup wizard)
      # - PULSE_AUTH_USER=admin
      # - PULSE_AUTH_PASS=secret123

volumes:
  pulse_data:
```

Run with: `docker compose up -d`

The `PULSE_IMAGE` variable lets the same compose file run either the public
community image or, for eligible paid customers, the private Pulse Pro image
shown on <https://pulserelay.pro/download.html>.

---

## ⚙️ Configuration

Pulse is configured via the UI (`system.json`) with optional environment overrides.

| Variable | Description | Default |
|----------|-------------|---------|
| `TZ` | Timezone | `UTC` |
| `PULSE_AUTH_USER` | Admin Username | *(unset)* |
| `PULSE_AUTH_PASS` | Admin Password | *(unset)* |
| `DISCOVERY_SUBNET` | Custom CIDR to scan | *(auto)* |
| `ALLOWED_ORIGINS` | CORS allowed origin (`*` or a single origin). Empty = same-origin only. | *(unset)* |
| `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |
| `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS` | Hide Docker update buttons (read-only mode) | `false` |
| `PULSE_METRICS_DB_PATH` | Optional path for only `metrics.db`, useful with tmpfs | `/data/metrics.db` |
| `PULSE_METRICS_ROLLUP_INTERVAL` | Metrics aggregation cadence; minimum 5 minutes | `15m` |

> **Tip**: Set `LOG_LEVEL=warn` to reduce log volume while still capturing important events.
> **Note**: API tokens are managed in the UI and stored in `api_tokens.json`.
> **Note**: Plain text values in `PULSE_AUTH_PASS` are auto-hashed on startup.

For SSD-sensitive installs, keep `/data` persistent and put only metrics
history on tmpfs:

```yaml
services:
  pulse:
    environment:
      PULSE_METRICS_DB_PATH: /metrics-tmpfs/metrics.db
    tmpfs:
      - /metrics-tmpfs:size=512m,uid=1000,gid=1000,mode=0700
```

Metrics history stored this way is lost on container restart.

<details>
<summary><strong>Advanced: Resource Limits & Healthcheck</strong></summary>

```yaml
services:
  pulse:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:7655/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```
</details>

---

## 🔄 Updates

To update Pulse to a specific release tag:

```bash
docker pull rcourtman/pulse:vX.Y.Z
docker stop pulse
docker rm pulse
# Re-run your docker run command
```

If using Compose:
```bash
docker compose pull
docker compose up -d
```

---

## 🔄 Docker / Podman Updates

Pulse can detect and apply updates to your Docker / Podman containers directly from the UI.

### How It Works

1. **Update Detection**: Pulse compares the local image digest with the latest digest from the container registry
2. **Visual Indicator**: Containers with available updates show a blue upward arrow icon
3. **One-Click Update**: Click the update button, confirm, and Pulse handles the rest
4. **Batch Updates**: Use the **"Update All"** button in the filter bar to queue updates for multiple containers

### Updating a Container

1. Navigate to the **Workloads** page (or filter by Docker sources on **Infrastructure**)
2. Look for containers with a blue update arrow (⬆️)
3. Click the update button → Click **Confirm**
4. Pulse will:
   - Pull the latest image
   - Stop the current container
   - Create a backup (renamed with `_pulse_backup_` suffix)
   - Start a new container with the same configuration
   - Clean up the backup after 15 minutes (if the update succeeds)

### Batch Updates

When multiple containers have updates available, an **"Update All"** button appears in the filter bar.
1. Click **"Update All"**
2. Click again within 3 seconds to confirm
3. Pulse queues update commands for each container (they run on the next agent report cycle)
4. A toast summary reports how many updates were queued or failed

### Safety Features

- **Automatic Backup**: The old container is renamed, not deleted, until the update succeeds
- **Rollback on Failure**: If the new container fails to start, the old one is restored
- **Configuration Preserved**: Networks, volumes, ports, environment variables are all preserved

### Requirements

- **Unified agent** running on the Docker host with Docker monitoring enabled
- Agent must have Docker socket access (`/var/run/docker.sock`)
- Registry must be accessible for update detection (public registries work automatically)

### Private Registries

For private registries, ensure your Docker daemon has credentials configured:

```bash
docker login registry.example.com
```

The agent uses the Docker daemon's credentials for both pulling images and checking for updates.

Paid Pulse Pro Docker installs use the private Pulse Pro registry rather than
the public `rcourtman/pulse` image. Open <https://pulserelay.pro/download.html>,
paste your activation key, run the Docker login command shown there, then run
the shown `PULSE_IMAGE=license.pulserelay.pro/pulse-pro:<version> docker compose pull`
and `docker compose up -d` commands from the host that already runs Pulse. If
your compose file has a hardcoded `image: rcourtman/pulse:...` line, change it
to `image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}` or directly to the private
image shown on the download page before running those commands.

### Disabling Update Features

Pulse provides granular control over update features via environment variables on the **Pulse server**:

| Variable | Description |
|----------|-------------|
| `PULSE_DISABLE_DOCKER_UPDATE_ACTIONS` | Hides update buttons from the UI while still detecting updates. Use this for "read-only" monitoring. |

**Example - Read-Only Mode** (detect updates but prevent actions):
```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    environment:
      - PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true
```

To disable registry checks entirely, set `PULSE_DISABLE_DOCKER_UPDATE_CHECKS=true` on the **agent**.

You can also toggle "Hide Docker Update Buttons" from the UI in **Settings → System → General** under **Docker / Podman updates**.

---

## 🛠️ Troubleshooting

- **Forgot Password?**
  ```bash
  docker exec pulse rm /data/.env
  docker restart pulse
  # Access UI again. Pulse will require a bootstrap token for setup.
  # Get it with:
  docker exec pulse /app/pulse bootstrap-token
  ```

- **Logs**
  ```bash
  docker logs -f pulse
  ```

- **Shell Access**
  ```bash
  docker exec -it pulse /bin/sh
  ```
