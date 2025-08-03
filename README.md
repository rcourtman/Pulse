# Pulse - Proxmox Monitoring System

Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS) with a modern web interface.

## Features

- 📊 **Real-time Monitoring** - Live updates of VMs, containers, nodes, and storage
- 💾 **Unified Backup View** - See PVE backups, PBS backups, and snapshots in one place
- 🚨 **Smart Alerts** - Configurable thresholds with webhook notifications
- 📈 **Performance Metrics** - Historical data with interactive charts
- ⚙️ **Flexible Configuration** - Configure via UI, config files, environment variables, or CLI
- 🌐 **Multi-Instance Support** - Monitor multiple PVE clusters and PBS instances

## Quick Start

### Using Docker (Recommended)

```bash
docker run -d \
  -p 3000:3000 \
  -p 7655:7655 \
  -v /path/to/config:/etc/pulse \
  -e PULSE_SERVER_BACKEND_PORT=3000 \
  -e PULSE_SERVER_FRONTEND_PORT=7655 \
  --name pulse \
  pulse:latest
```

### Manual Installation

1. **Clone the repository:**
```bash
git clone https://github.com/rcourtman/Pulse.git
cd Pulse
```

2. **Build the backend:**
```bash
go build -o bin/pulse ./cmd/pulse
```

3. **Install frontend dependencies:**
```bash
cd frontend-modern
npm install
npm run build
cd ..
```

4. **Create configuration:**
```bash
./bin/pulse config init
# Edit pulse.yml with your Proxmox credentials
```

5. **Run Pulse:**
```bash
./bin/pulse
```

Access the web interface at `http://localhost:7655`

## Configuration

Pulse supports multiple configuration methods with the following precedence (highest to lowest):

1. Command-line arguments
2. Environment variables
3. Configuration file (pulse.yml)
4. Default values

### Configuration Methods

#### 1. Web UI Configuration
Navigate to the Settings tab in the web interface to configure ports, monitoring intervals, and other settings.

#### 2. Configuration File
Generate an example configuration:
```bash
./bin/pulse config init
```

Example `pulse.yml`:
```yaml
server:
  backend:
    port: 3000
    host: "0.0.0.0"
  frontend:
    port: 7655
    host: "0.0.0.0"

monitoring:
  pollingInterval: 5000      # milliseconds
  backupPollingCycles: 10    # poll backups every N cycles
```

#### 3. Environment Variables
```bash
export PULSE_SERVER_BACKEND_PORT=8080
export PULSE_SERVER_FRONTEND_PORT=8081
export PULSE_LOG_LEVEL=debug
```

#### 4. Command-Line Arguments
```bash
./bin/pulse --backend-port=8080 --frontend-port=8081 --log-level=debug
```

### Proxmox Configuration

Create `/etc/pulse/nodes.json`:
```json
{
  "pve_instances": [
    {
      "name": "my-cluster",
      "host": "https://proxmox.example.com:8006",
      "username": "monitor@pve",
      "token_name": "monitor-token",
      "token_value": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "verify_ssl": false
    }
  ],
  "pbs_instances": [
    {
      "name": "my-backup-server",
      "host": "https://pbs.example.com:8007",
      "username": "monitor@pbs",
      "token_name": "monitor-token",
      "token_value": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "verify_ssl": false
    }
  ]
}
```

See [Configuration Documentation](docs/CONFIGURATION.md) for all options.

## API Endpoints

- `GET /api/state` - Current system state
- `GET /api/settings` - Current configuration
- `POST /api/settings/update` - Update configuration
- `GET /api/alerts/config` - Alert configuration
- `WS /ws` - WebSocket for real-time updates

## Development

### Prerequisites
- Go 1.19+
- Node.js 18+
- npm or yarn

### Building from Source

```bash
# Backend
go build -o bin/pulse ./cmd/pulse

# Frontend
cd frontend-modern
npm install
npm run dev  # Development server
npm run build  # Production build
```

### Running Tests

```bash
go test ./...
```

## Systemd Service

For production deployments, use systemd services:

```bash
# Copy service files
sudo cp systemd/*.service /etc/systemd/system/

# Enable and start services
sudo systemctl enable pulse-backend pulse-frontend
sudo systemctl start pulse-backend pulse-frontend
```

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

- Issues: [GitHub Issues](https://github.com/yourusername/pulse/issues)
- Discussions: [GitHub Discussions](https://github.com/yourusername/pulse/discussions)