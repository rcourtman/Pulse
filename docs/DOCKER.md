# Docker Deployment Guide

> **Proxmox VE Users:** Consider using the [official installer](https://github.com/rcourtman/Pulse#install) instead, which automatically creates an optimized LXC container.

## Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

1. Access at `http://your-server:7655`
2. **Complete the mandatory security setup** on first access
3. Save your credentials - they won't be shown again!

## First-Time Setup

When you first access Pulse, you'll see the security setup wizard:

1. **Create Admin Account**
   - Choose a username (default: admin)
   - Set a password or use the generated one
   - An API token is automatically generated

2. **Save Your Credentials**
   - Download or copy them immediately
   - They won't be shown again after setup

3. **Access Dashboard**
   - Click "Continue to Login"
   - Use your new credentials to sign in

## Docker Compose

### Basic Setup (Recommended for First-Time Users)

```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    restart: unless-stopped

volumes:
  pulse_data:
```

Then:
1. Run: `docker compose up -d`
2. Access: `http://your-server:7655`
3. Complete the security setup wizard
4. (Optional) Copy `.env.example` to `.env` if you want to pre-configure credentials later

### Pre-Configured Authentication (Advanced)

If you want to skip the setup wizard, you can pre-configure authentication:

```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      PULSE_AUTH_USER: 'admin'
      # Plain text values are auto-hashed on startup. To use a bcrypt hash,
      # escape $ as $$ (e.g. $$2a$$12$$...) so docker compose does not treat it
      # as variable expansion.
      PULSE_AUTH_PASS: 'super-secret-password'
      API_TOKEN: 'your-48-char-hex-token'  # Generate: openssl rand -hex 24
      PULSE_PUBLIC_URL: 'https://pulse.example.com'  # Used for webhooks/links
      # TZ: 'UTC'
    restart: unless-stopped

volumes:
  pulse_data:
```

⚠️ **Important**: If you paste a bcrypt hash instead of a plain-text password, remember that Compose treats `$` as variable expansion. Escape each `$` as `$$`. Example: `$2a$12$...` becomes `$$2a$$12$$...`.

### Using External .env File (Cleaner Approach)

Create `.env` file (no escaping needed here). You can copy `.env.example` from the repository as a starting point:
```env
PULSE_AUTH_USER=admin
PULSE_AUTH_PASS=super-secret-password          # Plain text (auto-hashed) or bcrypt hash
API_TOKEN=your-48-char-hex-token               # Generate with: openssl rand -hex 24
PULSE_PUBLIC_URL=https://pulse.example.com     # Recommended for webhooks
TZ=Asia/Kolkata                                # Optional: matches host timezone
```

**Note**: Plain text credentials are automatically hashed for security. You can provide either plain text (simpler) or pre-hashed values (advanced).

Docker-compose.yml:
```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    container_name: pulse
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    env_file: .env
    restart: unless-stopped

volumes:
  pulse_data:
```

### Updating Your Stack

```bash
docker compose pull        # Fetch the latest Pulse image
docker compose up -d       # Recreate container with zero-downtime update
```

If you change anything in `.env`, run `docker compose up -d` again so the container picks up the new values.

## Generating Credentials (Optional)

**Note**: Since v4.5.0, plain text credentials are automatically hashed. Pre-hashing is optional for advanced users.

### Simple Approach (Recommended)
```bash
# Just use plain text - Pulse auto-hashes for you
docker run -d \
  -e PULSE_AUTH_USER=admin \
  -e PULSE_AUTH_PASS=mypassword \
  -e API_TOKEN=mytoken123 \
  rcourtman/pulse:latest
```

### Advanced: Pre-Hashing (Optional)
```bash
# Generate bcrypt hash for password
docker run --rm -it rcourtman/pulse:latest pulse hash-password

# Generate random API token
openssl rand -hex 32
```

## Data Persistence

All configuration and data is stored in `/data`:
- `.env` - Authentication credentials (if using setup wizard)
- `*.enc` - Encrypted node credentials
- `*.json` - Configuration files
- `.encryption.key` - Auto-generated encryption key

### Backup
```bash
docker run --rm -v pulse_data:/data -v $(pwd):/backup alpine tar czf /backup/pulse-backup.tar.gz -C /data .
```

### Restore
```bash
docker run --rm -v pulse_data:/data -v $(pwd):/backup alpine tar xzf /backup/pulse-backup.tar.gz -C /data
```

## Docker Workspace Highlights

Once the agent is reporting, open the **Docker** tab in Pulse to explore:

- **Host grid with issues column** – surfaces restart loops, health-check failures, and highlights hosts that have missed their heartbeat.
- **Inline search** – filter by host name, stack label, or container name; results update instantly in the grid and side drawer.
- **Container drawers** – show CPU/memory charts, restart counters, last exit codes, mounted ports, and environment labels at a glance.
- **Time-since heartbeat** – every host entry shows the last heartbeat timestamp so you can spot telemetry gaps quickly.

If a host remains offline, review [Troubleshooting → Docker Agent Shows Hosts Offline](TROUBLESHOOTING.md#docker-agent-shows-hosts-offline).

## Network Discovery

**New in v4.5.0+**: Pulse automatically scans common home/office networks when running in Docker!

### How It Works
1. Detects Docker environment automatically
2. Scans multiple common subnets in parallel:
   - 192.168.1.0/24 (most routers)
   - 192.168.0.0/24 (very common)
   - 10.0.0.0/24 (some setups)
   - 192.168.88.0/24 (MikroTik)
   - 172.16.0.0/24 (enterprise)

**Result**: Finds all Proxmox nodes without any configuration!

### Custom Networks (Rarely Needed)
Only for non-standard subnets:
```yaml
environment:
  DISCOVERY_SUBNET: "192.168.50.0/24"  # Only if using unusual subnet
```

## Common Issues

### Can't Access After Upgrade
If upgrading from pre-v4.5.0:
1. You'll see the security setup wizard
2. Complete the setup - your nodes are preserved
3. Use your new credentials to login

### Lost Credentials
If you've lost your credentials:
```bash
# Stop container
docker stop pulse

# Remove auth configuration
docker exec pulse rm /data/.env

# Restart and go through setup again
docker restart pulse
```

### Setup Wizard Not Showing
This happens if you have auth environment variables set:
1. Remove environment variables from docker-compose.yml
2. Recreate the container
3. Access the UI to see the setup wizard

### Password Hash Issues
Common problems:
- **Hash truncated**: Must be exactly 60 characters
- **Not escaped in docker-compose**: Use `$$` instead of `$`
- **Wrong format**: Must start with `$2a$`, `$2b$`, or `$2y$`

## Security Best Practices

1. **Always use HTTPS in production** - Use a reverse proxy (nginx, Traefik, Caddy)
2. **Strong passwords** - Use the generated password or 16+ characters
3. **Protect API tokens** - Treat them like passwords
4. **Regular backups** - Backup the `/data` volume regularly
5. **Network isolation** - Don't expose port 7655 directly to the internet

## Environment Variables Reference

> **⚠️ Important**: Environment variables always override UI/system.json settings. If you set a value via env var (e.g., `DISCOVERY_SUBNET`), changes made in the UI for that setting will NOT take effect until you remove the env var. This follows standard container practices where env vars have highest precedence.

### Authentication
| Variable | Description | Example / Default |
|----------|-------------|-------------------|
| `PULSE_AUTH_USER` | Admin username | `admin` |
| `PULSE_AUTH_PASS` | Admin password (plain text auto-hashed or bcrypt hash) | `super-secret-password` or `$2a$12$...` |
| `API_TOKEN` | API access token (plain text or SHA3-256 hash) | `openssl rand -hex 24` |
| `DISABLE_AUTH` | Disable authentication entirely | `false` |
| `PULSE_AUDIT_LOG` | Enable security audit logging | `false` |

### Network
| Variable | Description | Default |
|----------|-------------|---------|
| `FRONTEND_PORT` | Port exposed for the UI inside the container | `7655` |
| `BACKEND_PORT` | API port (same as UI for the all-in-one container) | `7655` |
| `BACKEND_HOST` | Bind address for the backend | `0.0.0.0` |
| `PULSE_PUBLIC_URL` | External URL used in notifications/webhooks | *(unset)* |
| `ALLOWED_ORIGINS` | Additional CORS origins (comma separated) | Same-origin only |
| `DISCOVERY_SUBNET` | Override automatic network discovery CIDR | Auto-scans common networks |
| `CONNECTION_TIMEOUT` | Proxmox/PBS API timeout (seconds) | `10` |
| `PORT` | Legacy alias for `FRONTEND_PORT` | `7655` |

### System
| Variable | Description | Default |
|----------|-------------|---------|
| `TZ` | Timezone inside the container | `UTC` |
| `LOG_LEVEL` | Logging verbosity | `info` |
| `METRICS_RETENTION_DAYS` | Days of metrics history to keep | `7` |

## Advanced Configuration

### Custom Network
```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    networks:
      - monitoring
    # ... rest of config

networks:
  monitoring:
    driver: bridge
```

### Resource Limits
```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
    # ... rest of config
```

### Health Check
```yaml
services:
  pulse:
    image: rcourtman/pulse:latest
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:7655/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    # ... rest of config
```
