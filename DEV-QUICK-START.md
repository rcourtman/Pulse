# Development Quick Start

## Prerequisites

- Go **1.24.9** or newer
- Node.js 20+
- pnpm 9+ (for frontend work)

> **Tip**: Read [`ARCHITECTURE.md`](ARCHITECTURE.md) to understand the system design before diving in.

## Hot-Reload Development Mode

Start the development environment with hot-reload:

```bash
./scripts/hot-dev.sh
```

This starts:
- Backend API on port 7656
- Frontend on port 7655 with hot-reload
- Both backend and frontend automatically reload on code changes

Access the app at: http://localhost:7655 or http://192.168.0.123:7655

## Toggle Between Mock and Production Data

Switch modes seamlessly without manually restarting services:

```bash
# Enable mock mode (test with fake data)
npm run mock:on

# Disable mock mode (use real Proxmox nodes)
npm run mock:off

# Check current mode
npm run mock:status

# Edit mock configuration
npm run mock:edit
```

Or use the script directly:

```bash
./scripts/toggle-mock.sh on   # Enable mock mode
./scripts/toggle-mock.sh off  # Disable mock mode (use production data)
./scripts/toggle-mock.sh status  # Show current status
```

The toggle script automatically:
- Updates `mock.env` configuration
- Restarts the backend with new settings
- Keeps the frontend running (no restart needed)
- Syncs production config when switching to production mode
- Switches `PULSE_DATA_DIR` between `/opt/pulse/tmp/mock-data` (mock) and `/etc/pulse` (production) so test data never touches real credentials

## Mock Mode Configuration

Edit `mock.env` to customize mock data:

```bash
PULSE_MOCK_MODE=false           # Enable/disable mock mode
PULSE_MOCK_NODES=7              # Number of mock nodes
PULSE_MOCK_VMS_PER_NODE=5       # Average VMs per node
PULSE_MOCK_LXCS_PER_NODE=8      # Average containers per node
PULSE_MOCK_RANDOM_METRICS=true  # Enable metric fluctuations
PULSE_MOCK_STOPPED_PERCENT=20   # Percentage of stopped guests
```

Prefer `mock.env.local` for personal tweaks (`cp mock.env mock.env.local`). The toggle script honours `.local` first, keeping the shared defaults untouched.

## Development Workflow

1. Start hot-dev: `./scripts/hot-dev.sh`
2. Switch to mock mode for testing: `npm run mock:on`
3. Develop and test your changes
4. Switch to production mode to verify: `npm run mock:off`
5. Code changes auto-reload, no manual restarts needed!

## Troubleshooting

If the backend doesn't pick up changes:
```bash
npm run mock:off  # Force restart with production data
npm run mock:on   # Force restart with mock data
```

Check backend logs:
```bash
tail -f /tmp/pulse-backend.log
```

Check if services are running:
```bash
lsof -i :7656  # Backend
lsof -i :7655  # Frontend
```
