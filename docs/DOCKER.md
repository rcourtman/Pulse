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
| `LOG_LEVEL` | Log verbosity | `info` |

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

## 🛠️ Troubleshooting

- **Forgot Password?**
  ```bash
  docker exec pulse rm /data/.env
  docker restart pulse
  # Access UI to run setup wizard again
  ```

- **Logs**
  ```bash
  docker logs -f pulse
  ```

- **Shell Access**
  ```bash
  docker exec -it pulse /bin/sh
  ```
