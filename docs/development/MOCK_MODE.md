# Mock Mode Development Guide

Mock mode allows you to develop and test Pulse without requiring real Proxmox infrastructure. It generates realistic mock data including nodes, VMs, containers, storage, backups, and alerts.

## Quick Start

### Toggling Mock Mode

During hot-dev mode (`scripts/hot-dev.sh`), use these npm commands to toggle mock mode:

```bash
# Enable mock mode
npm run mock:on

# Disable mock mode (use real infrastructure)
npm run mock:off

# Check current status
npm run mock:status

# Edit mock configuration
npm run mock:edit
```

The backend will **automatically reload** when you toggle mock mode - no manual restarts needed!

### Configuration

Mock mode is configured via `mock.env` in the project root. This file is **tracked in the repository** with sensible defaults, so mock mode works out of the box for all developers.

**Default configuration (mock.env):**
```bash
PULSE_MOCK_MODE=false              # Disabled by default
PULSE_MOCK_NODES=7
PULSE_MOCK_VMS_PER_NODE=5
PULSE_MOCK_LXCS_PER_NODE=8
PULSE_MOCK_RANDOM_METRICS=true
PULSE_MOCK_STOPPED_PERCENT=20
```

**Local overrides (not tracked):**
Create `mock.env.local` for personal settings that won't be committed:
```bash
# mock.env.local - your personal settings
PULSE_MOCK_MODE=true              # Always start in mock mode
PULSE_MOCK_NODES=3                # Fewer nodes for faster startup
```

The `.local` file overrides values from `mock.env`, and is gitignored to keep your personal preferences private.

**Configuration options:**

- `PULSE_MOCK_MODE`: Enable/disable mock mode (`true`/`false`)
- `PULSE_MOCK_NODES`: Number of nodes to generate (default: 7)
- `PULSE_MOCK_VMS_PER_NODE`: Average VMs per node (default: 5)
- `PULSE_MOCK_LXCS_PER_NODE`: Average containers per node (default: 8)
- `PULSE_MOCK_RANDOM_METRICS`: Enable metric fluctuations (`true`/`false`)
- `PULSE_MOCK_STOPPED_PERCENT`: Percentage of guests in stopped state (default: 20)

## Hot-Dev Workflow

1. **Start hot-dev mode:**
   ```bash
   scripts/hot-dev.sh
   ```

2. **Toggle mock mode as needed:**
   ```bash
   npm run mock:on   # Backend auto-reloads with mock data
   npm run mock:off  # Backend auto-reloads with real data
   ```

3. **Edit mock configuration:**
   ```bash
   npm run mock:edit  # Opens mock.env in your editor
   # Save and exit - backend auto-reloads!
   ```

4. **Frontend changes:** Just save your files - Vite hot-reloads instantly

**No port changes. No manual restarts. Everything just works!**

## Mock Data Generation

Mock mode generates:

- **Nodes**: Mix of clustered and standalone nodes
- **Cluster**: First 5 nodes form `mock-cluster`, rest are standalone
- **VMs & Containers**: Realistic distribution with various states
- **Storage**: Local, ZFS, PBS, and shared NFS storage
- **Backups**: Both PVE and PBS backups with realistic metadata
- **Alerts**: CPU, memory, disk, and connectivity alerts
- **Metrics**: Live-updating metrics every 2 seconds

### Node Characteristics

- **Clustered nodes** (`pve1`-`pve5`): Part of `mock-cluster`
- **Standalone nodes** (`standalone1`, etc.): Independent instances
- **Offline nodes**: `pve3` is always offline to test error handling
- **Host URLs**: Each node has `Host` field set (e.g., `https://pve1.local:8006`)
- **Cluster fields**: `IsClusterMember` and `ClusterName` properly set

## API Behavior in Mock Mode

### Fast, Cached Responses

In mock mode, `/api/state` returns **cached data instantly** - no locks, no delays, no timeouts. The mock data is stored in memory and updated every 2 seconds with realistic metric fluctuations.

### WebSocket Updates

The WebSocket connection receives updates every 2 seconds with changing metrics, just like production.

### Dashboard Grouping

Mock nodes include all required fields for proper dashboard grouping:
- `isClusterMember`: Boolean indicating cluster membership
- `clusterName`: Name of the cluster (e.g., "mock-cluster")
- `host`: Full node URL (e.g., "https://pve1.local:8006")

## Demo Server Usage

On the demo server, mock mode works the same way:

### Systemd Service

If using systemd (`pulse-dev` service):

```bash
# Toggle mock mode (restarts service)
sudo /opt/pulse/scripts/toggle-mock.sh on
sudo /opt/pulse/scripts/toggle-mock.sh off

# Check status
/opt/pulse/scripts/toggle-mock.sh status
```

### Manual Mode

If running the backend manually:

```bash
# Edit mock.env
nano /opt/pulse/mock.env

# The file watcher will detect changes and auto-reload the backend
# within 5 seconds (or immediately if fsnotify is working)
```

## File Watcher Details

The backend watches `mock.env` using:

1. **Primary**: `fsnotify` (instant notification on file changes)
2. **Fallback**: Polling every 5 seconds (if fsnotify fails)

When `mock.env` changes:
- Environment variables are updated
- Monitor is reloaded with new configuration
- New mock data is generated
- WebSocket clients receive updated state

**No manual process restarts required!**

## Troubleshooting

### Mock mode not updating

1. Check that mock.env exists: `ls -la /opt/pulse/mock.env`
2. Check file watcher logs: Look for "Detected mock.env file change" in logs
3. Verify environment variables: `env | grep PULSE_MOCK`
4. Try touching the file: `touch /opt/pulse/mock.env`

### Backend not reloading

1. Ensure hot-dev mode is running (not systemd service)
2. Check for errors in backend logs
3. Verify file watcher started successfully
4. Fall back to manual restart if needed

### Missing cluster grouping

1. Verify mock data includes `isClusterMember` and `clusterName`
2. Check API response: `curl http://localhost:7656/api/state | jq '.nodes[0]'`
3. Ensure frontend is receiving WebSocket updates

## Implementation Details

### Backend Components

- **Config Watcher** (`internal/config/watcher.go`): Watches both `.env` and `mock.env`
- **Mock Integration** (`internal/mock/integration.go`): Manages mock state and updates
- **Mock Generator** (`internal/mock/generator.go`): Generates realistic mock data
- **Monitor** (`internal/monitoring/monitor.go`): Returns cached mock data when enabled

### Auto-Reload Flow

1. User runs `npm run mock:on` (or edits `mock.env`)
2. `toggle-mock.sh` updates `mock.env` and touches the file
3. Config watcher detects file change (via fsnotify or polling)
4. Watcher triggers reload callback
5. ReloadableMonitor reloads with fresh config
6. New monitor instance starts with updated mock mode
7. WebSocket broadcasts new state to connected clients

### Performance

- Mock data generation: < 100ms for 7 nodes with 90+ guests
- State snapshot: Instant (returns cached data)
- Memory usage: ~50MB additional for mock data
- Update interval: 2 seconds for metric fluctuations

## Best Practices

1. **Use mock mode for frontend development** - Fast, predictable data
2. **Test with real data before PRs** - Ensure real infrastructure works
3. **Adjust mock config to test edge cases** - High load, many nodes, etc.
4. **Use mock.env.local for personal settings** - Your preferences won't be committed
5. **Keep mock.env defaults reasonable** - Other developers will use them
6. **Document any mock data assumptions** - Help other developers

## See Also

- [CONFIGURATION.md](../CONFIGURATION.md) - Production configuration
- [TROUBLESHOOTING.md](../TROUBLESHOOTING.md) - Common issues
- [API.md](../API.md) - API documentation
