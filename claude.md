# Claude Code Notes

## Critical Warnings

### ⚠️ NEVER kill the pulse process
**NEVER** run `pkill -f "./pulse"` or similar commands that kill the pulse process. This will terminate tmux/ttyd sessions and disconnect the Claude Code session.

If you need to restart pulse or enable mock mode, coordinate with the user first to avoid disconnection.

## Mock Mode System

### How Mock Mode Works
Mock mode generates realistic test data (nodes, VMs, containers, storage, backups) without requiring real Proxmox infrastructure.

**Key Implementation Details:**
- **Cached responses**: `/api/state` returns cached data from memory (instant, no API calls)
- **Auto-reload**: Backend watches `mock.env` and reloads when changed (no manual restarts)
- **File watcher**: `internal/config/watcher.go` monitors both `.env` and `mock.env`
- **State management**: `internal/mock/integration.go` manages mock state and updates
- **Data generation**: `internal/mock/generator.go` creates realistic mock data

### Toggling Mock Mode
```bash
# Enable mock mode
npm run mock:on

# Disable mock mode
npm run mock:off

# Check status
npm run mock:status

# Edit configuration
npm run mock:edit
```

**What happens when toggling:**
1. `toggle-mock.sh` updates `PULSE_MOCK_MODE` in `mock.env`
2. File watcher detects change (via fsnotify or 5s polling)
3. `ConfigWatcher` triggers `reloadableMonitor.Reload()`
4. Backend stops old monitor, loads fresh config, starts new monitor
5. New mock data generated (or real data if disabled)
6. WebSocket broadcasts new state to clients
7. Total time: 2-5 seconds, no manual restarts

### Mock Mode Architecture
```
Monitor.GetState()
  ↓
  Checks mock.IsMockEnabled()
  ↓
  If true: Returns mock.GetMockState() (cached, instant)
  If false: Returns m.state.GetSnapshot() (real data)
```

**Mock data includes all required fields:**
- `Host`: Node URL (e.g., `https://pve1.local:8006`)
- `IsClusterMember`: Boolean for cluster membership
- `ClusterName`: Cluster name for grouped nodes
- `Instance`: Cluster or standalone identifier

### Configuration (mock.env)
The `mock.env` file is tracked in the repository with sensible defaults (mock mode disabled by default). Developers can create `mock.env.local` for personal overrides (gitignored).

```bash
PULSE_MOCK_MODE=false             # Enable/disable (false by default in repo)
PULSE_MOCK_NODES=7                # Number of nodes
PULSE_MOCK_VMS_PER_NODE=5         # Average VMs per node
PULSE_MOCK_LXCS_PER_NODE=8        # Average containers per node
PULSE_MOCK_RANDOM_METRICS=true    # Enable metric fluctuations
PULSE_MOCK_STOPPED_PERCENT=20     # % of stopped guests
```

**Personal overrides:** Create `mock.env.local` with your settings - it overrides `mock.env` and is gitignored.

### When to Use Mock Mode
- Frontend development (fast, predictable data)
- Testing dashboard grouping/clustering
- Demo environments
- CI/CD (faster than real infrastructure)

### When NOT to Use Mock Mode
- Testing real Proxmox API integration
- Validating authentication flows
- Performance testing against actual infrastructure
- Before creating PRs (test with real data first)
