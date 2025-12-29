# 🐳 Docker Guide

Pulse is distributed as a lightweight, Alpine-based Docker image.

## 🚀 Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Access at `http://<your-ip>:7655`.

---

## 📦 Docker Compose

Create a `docker-compose.yml` file:

```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - TZ=Europe/London
      # Optional: Pre-configure auth (skips setup wizard)
      # - PULSE_AUTH_USER=admin
      # - PULSE_AUTH_PASS=secret123

volumes:
  pulse_data:
```

Run with: `docker compose up -d`

---

## ⚙️ Configuration

Pulse is configured via environment variables.

| Variable | Description | Default |
|----------|-------------|---------|
| `TZ` | Timezone | `UTC` |
| `PULSE_AUTH_USER` | Admin Username | *(unset)* |
| `PULSE_AUTH_PASS` | Admin Password | *(unset)* |
| `API_TOKENS` | Comma-separated API tokens | *(unset)* |
| `DISCOVERY_SUBNET` | Custom CIDR to scan | *(auto)* |
| `ALLOWED_ORIGINS` | CORS allowed domains | *(none)* |
| `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |

> **Tip**: Set `LOG_LEVEL=warn` to reduce log volume while still capturing important events.

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

To update Pulse to the latest version:

```bash
docker pull rcourtman/pulse:latest
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

## 🔄 Container Updates

Pulse can detect and apply updates to your Docker containers directly from the UI.

### How It Works

1. **Update Detection**: Pulse compares the local image digest with the latest digest from the container registry
2. **Visual Indicator**: Containers with available updates show a blue upward arrow icon
3. **One-Click Update**: Click the update button, confirm, and Pulse handles the rest

### Updating a Container

1. Navigate to the **Docker** tab
2. Look for containers with a blue update arrow (⬆️)
3. Click the update button → Click **Confirm**
4. Pulse will:
   - Pull the latest image
   - Stop the current container
   - Create a backup (renamed with `_pulse_backup_` suffix)
   - Start a new container with the same configuration
   - Clean up the backup after 5 minutes

### Safety Features

- **Automatic Backup**: The old container is renamed, not deleted, until the update succeeds
- **Rollback on Failure**: If the new container fails to start, the old one is restored
- **Configuration Preserved**: Networks, volumes, ports, environment variables are all preserved

### Requirements

- **Unified Agent v5.0.6+** running on the Docker host
- Agent must have Docker socket access (`/var/run/docker.sock`)
- Registry must be accessible for update detection (public registries work automatically)

### Private Registries

For private registries, ensure your Docker daemon has credentials configured:

```bash
docker login registry.example.com
```

The agent uses the Docker daemon's credentials for both pulling images and checking for updates.

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
