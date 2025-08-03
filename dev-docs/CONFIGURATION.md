# Pulse Configuration Guide

Pulse supports a flexible configuration system with multiple sources and clear precedence rules.

## Configuration Sources (in order of precedence)

1. **Command-line arguments** (highest priority)
2. **Environment variables**
3. **Configuration file**
4. **Default values** (lowest priority)

## Quick Start

### Using Environment Variables

```bash
# Set custom ports
export PULSE_SERVER_BACKEND_PORT=8080
export PULSE_SERVER_FRONTEND_PORT=8081

# Set log level
export PULSE_LOG_LEVEL=debug

# Run Pulse
./bin/pulse
```

### Using Configuration File

1. Generate an example configuration file:
```bash
./bin/pulse config init
```

2. Edit `pulse.yml` to customize settings

3. Run Pulse:
```bash
./bin/pulse
```

### Using Command-Line Arguments

```bash
# Override specific settings
./bin/pulse --backend-port=9000 --frontend-port=9001 --log-level=debug

# Use a custom config file
./bin/pulse --config=/custom/path/pulse.yml
```

## Configuration File Formats

Pulse supports both YAML and JSON configuration files:

- `/etc/pulse/pulse.yml` (system-wide, YAML)
- `/etc/pulse/pulse.json` (system-wide, JSON)
- `./pulse.yml` (local directory, YAML)
- `./pulse.json` (local directory, JSON)

### Example YAML Configuration

```yaml
server:
  backend:
    port: 3000
    host: "0.0.0.0"
  frontend:
    port: 7655
    host: "0.0.0.0"

monitoring:
  pollingInterval: 5000
  backupPollingCycles: 10

logging:
  level: "info"
  file: "/var/log/pulse/pulse.log"
```

## All Configuration Options

### Server Settings

| Setting | ENV Variable | CLI Flag | Default | Description |
|---------|-------------|----------|---------|-------------|
| `server.backend.port` | `PULSE_SERVER_BACKEND_PORT` | `--backend-port` | 3000 | Backend API server port |
| `server.backend.host` | `PULSE_SERVER_BACKEND_HOST` | `--backend-host` | 0.0.0.0 | Backend bind address |
| `server.frontend.port` | `PULSE_SERVER_FRONTEND_PORT` | `--frontend-port` | 7655 | Frontend web UI port |
| `server.frontend.host` | `PULSE_SERVER_FRONTEND_HOST` | `--frontend-host` | 0.0.0.0 | Frontend bind address |

### Monitoring Settings

| Setting | ENV Variable | Default | Description |
|---------|-------------|---------|-------------|
| `monitoring.pollingInterval` | `PULSE_MONITORING_POLLING_INTERVAL` | 5000 | Polling interval in milliseconds |
| `monitoring.concurrentPolling` | `PULSE_MONITORING_CONCURRENT_POLLING` | true | Enable concurrent polling |
| `monitoring.backupPollingCycles` | `PULSE_MONITORING_BACKUP_POLLING_CYCLES` | 10 | Poll backups every N cycles |
| `monitoring.metricsRetentionDays` | `PULSE_MONITORING_METRICS_RETENTION_DAYS` | 7 | Days to retain metrics |

### Logging Settings

| Setting | ENV Variable | CLI Flag | Default | Description |
|---------|-------------|----------|---------|-------------|
| `logging.level` | `PULSE_LOG_LEVEL` | `--log-level` | info | Log level (debug, info, warn, error) |
| `logging.file` | `PULSE_LOG_FILE` | `--log-file` | /opt/pulse/pulse.log | Log file path |
| `logging.maxSize` | `PULSE_LOG_MAX_SIZE` | - | 100 | Max log file size in MB |
| `logging.maxBackups` | `PULSE_LOG_MAX_BACKUPS` | - | 5 | Number of log files to keep |
| `logging.maxAge` | `PULSE_LOG_MAX_AGE` | - | 30 | Max age in days for log files |
| `logging.compress` | `PULSE_LOG_COMPRESS` | - | true | Compress rotated logs |

### Security Settings

| Setting | ENV Variable | Default | Description |
|---------|-------------|---------|-------------|
| `security.apiToken` | `PULSE_API_TOKEN` | "" | API authentication token |
| `security.allowedOrigins` | `PULSE_ALLOWED_ORIGINS` | ["*"] | CORS allowed origins (comma-separated) |
| `security.iframeEmbedding` | `PULSE_IFRAME_EMBEDDING` | SAMEORIGIN | X-Frame-Options header |
| `security.enableAuthentication` | `PULSE_ENABLE_AUTHENTICATION` | false | Enable authentication |

## Docker Configuration

When running in Docker, you can use environment variables or mount a config file:

```yaml
version: '3.8'
services:
  pulse:
    image: pulse:latest
    environment:
      - PULSE_SERVER_BACKEND_PORT=8080
      - PULSE_SERVER_FRONTEND_PORT=8081
      - PULSE_LOG_LEVEL=debug
    volumes:
      - ./pulse.yml:/etc/pulse/pulse.yml
    ports:
      - "8080:8080"
      - "8081:8081"
```

## Configuration Commands

### Generate Example Configuration

```bash
# Generate YAML config (default)
./bin/pulse config init

# Generate JSON config
./bin/pulse config init --format=json --output=pulse.json

# Force overwrite existing file
./bin/pulse config init --force
```

### Validate Configuration

```bash
# Validate default config file
./bin/pulse config validate

# Validate specific file
./bin/pulse config validate /path/to/pulse.yml

# Show effective configuration
./bin/pulse config validate --verbose
```

## Port Configuration Notes

1. **Privileged Ports**: Ports below 1024 require root privileges
2. **Port Conflicts**: Pulse will check if ports are available before binding
3. **Firewall**: Remember to update firewall rules when changing ports

## Best Practices

1. **Production**: Use a configuration file in `/etc/pulse/`
2. **Development**: Use environment variables or CLI arguments
3. **Docker**: Use environment variables for flexibility
4. **Security**: Always set `apiToken` in production environments
5. **Logging**: Use appropriate log levels (info for production, debug for development)

## Troubleshooting

### Configuration Not Loading

1. Check file permissions: `ls -la /etc/pulse/pulse.yml`
2. Validate syntax: `./bin/pulse config validate`
3. Check logs for errors: `tail -f /opt/pulse/pulse.log`

### Port Already in Use

1. Check what's using the port: `sudo lsof -i :PORT`
2. Either stop the conflicting service or choose a different port
3. Update firewall rules if needed

### Environment Variables Not Working

1. Ensure correct prefix: `PULSE_`
2. Check spelling and case sensitivity
3. Export variables: `export PULSE_SERVER_BACKEND_PORT=8080`

## Migration from Old Configuration

If upgrading from an older version:

1. Backend port was in main config, now in `server.backend.port`
2. Polling interval now in milliseconds (was seconds)
3. Node configuration remains in separate files (nodes.json)