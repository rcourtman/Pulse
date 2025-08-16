# Docker Deployment Guide

## Quick Start

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

Access at: `http://your-server:7655`

## Docker Compose

### Basic Configuration
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

### With Authentication

⚠️ **CRITICAL**: Docker Compose requires escaping `$` characters in bcrypt hashes!

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
      # IMPORTANT: Use $$ instead of $ in docker-compose.yml!
      PULSE_AUTH_USER: 'admin'
      PULSE_AUTH_PASS: '$$2a$$12$$YourHashHere...'  # <-- Note the $$
      API_TOKEN: 'your-48-char-hex-token'
    restart: unless-stopped

volumes:
  pulse_data:
```

### Alternative: Using .env File (Recommended)

Create `.env` file (no escaping needed):
```env
PULSE_AUTH_USER=admin
PULSE_AUTH_PASS=$2a$12$YourHashHere...
API_TOKEN=your-48-char-hex-token
```

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

## Environment Variables

### Security Configuration
| Variable | Description | Example |
|----------|-------------|---------|
| `PULSE_AUTH_USER` | Username for web UI | `admin` |
| `PULSE_AUTH_PASS` | Bcrypt password hash (60 chars) | `$2a$12$...` |
| `API_TOKEN` | API token (plain text) | 48 hex characters |
| `ALLOW_UNPROTECTED_EXPORT` | Allow export without auth | `false` |

### Network Configuration
| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` or `FRONTEND_PORT` | Web UI port | `7655` |
| `ALLOWED_ORIGINS` | CORS allowed origins | none (same-origin) |

### System Configuration
| Variable | Description | Default |
|----------|-------------|---------|
| `POLLING_INTERVAL` | Seconds between node checks | `3` |
| `CONNECTION_TIMEOUT` | Connection timeout in seconds | `10` |
| `LOG_LEVEL` | Logging level | `info` |
| `UPDATE_CHANNEL` | Update channel (stable/rc) | `stable` |

## Volume Management

### Data Persistence
All configuration and data is stored in `/data`:
- `.env` - Authentication credentials (if using Quick Setup)
- `*.enc` - Encrypted node credentials
- `*.json` - Configuration files
- `metrics/` - Historical metrics data

### Backup
```bash
# Backup volume
docker run --rm \
  -v pulse_data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/pulse-backup.tar.gz -C /data .

# Restore volume
docker run --rm \
  -v pulse_data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/pulse-backup.tar.gz -C /data
```

## Security Setup

### Method 1: Quick Security Setup (Recommended)
1. Start container WITHOUT auth environment variables
2. Access http://your-server:7655
3. Follow the mandatory Quick Security Setup wizard
4. Credentials are saved to `/data/.env`
5. Security is active immediately (no restart needed)

### Method 2: Manual Configuration
1. Generate password hash:
   ```bash
   # Using online bcrypt generator or another tool
   # Hash must be exactly 60 characters
   ```

2. Generate API token:
   ```bash
   # Generate random token (24 bytes = 48 hex chars)
   openssl rand -hex 24
   ```

3. Add to docker-compose.yml (remember to escape $ as $$)

### Method 3: Using Existing .env
If you have a `.env` file from another installation:
```bash
docker cp .env pulse:/data/.env
docker restart pulse
```

## Common Issues

### Cannot Login
1. **Check hash length**: Must be exactly 60 characters
2. **Check escaping**: In docker-compose.yml, use `$$` instead of `$`
3. **Check quotes**: Hash must be in single quotes
4. **Check logs**: `docker logs pulse | grep -i auth`

### No .env File
**This is normal** when using environment variables. The .env file is only created when:
- Using Quick Security Setup
- Changing password through UI
- Manually creating it

### Container Won't Start
```bash
# Check logs
docker logs pulse

# Common issues:
# - Port already in use
# - Volume permission issues
# - Invalid environment variables
```

### Password Change Fails
For v4.3.7 and earlier, password changes fail with sudo error. Update to v4.3.8+:
```bash
docker pull rcourtman/pulse:latest
docker stop pulse
docker rm pulse
# Re-run docker run command
```

## Networking

### Network Discovery Configuration

Docker containers use their internal network by default (e.g., 172.17.0.0/24), which prevents proper discovery of Proxmox nodes on your LAN. To fix this:

1. After first start, edit the configuration:
   ```bash
   docker exec pulse sh -c 'cat > /data/system.json << EOF
   {
     "pollingInterval": 10,
     "discoverySubnet": "192.168.1.0/24"  # Your LAN subnet
   }
   EOF'
   ```

2. Restart the container:
   ```bash
   docker restart pulse
   ```

Discovery will now scan your specified subnet every 5 minutes.

### Using Host Network
Alternatively, use host network mode for automatic LAN detection:
```bash
docker run -d \
  --name pulse \
  --network host \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### Behind a Reverse Proxy
See [Reverse Proxy Guide](REVERSE_PROXY.md) for nginx, Traefik, Caddy configurations.

## Updates

### Manual Update
```bash
# Pull latest image
docker pull rcourtman/pulse:latest

# Stop and remove old container
docker stop pulse
docker rm pulse

# Start new container with same settings
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### Docker Compose Update
```bash
docker-compose pull
docker-compose up -d
```

### Automatic Updates
Use Watchtower or similar tools:
```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  containrrr/watchtower \
  --schedule "0 0 3 * * *" \
  pulse
```

## Tips and Best Practices

1. **Always use named volumes** for data persistence
2. **Escape $ characters** in docker-compose.yml
3. **Use .env files** for cleaner configuration
4. **Set resource limits** for production:
   ```yaml
   deploy:
     resources:
       limits:
         cpus: '2'
         memory: 512M
   ```
5. **Enable auto-restart** with `--restart unless-stopped`
6. **Regular backups** of the data volume
7. **Monitor logs** with `docker logs -f pulse`

## Debugging

### Enable Debug Logging
```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  -e LOG_LEVEL=debug \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### Access Container Shell
```bash
docker exec -it pulse sh
```

### View Environment
```bash
docker exec pulse env | grep -E "PULSE|API"
```

### Check Version
```bash
curl http://localhost:7655/api/version
```