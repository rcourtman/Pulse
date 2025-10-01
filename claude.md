# Claude Code Notes

## Development Environment Context

### Your Role: Development Environment Orchestrator
You are the **intelligent orchestrator** of this development environment. The user expects:
- **Zero manual intervention** - Everything should "just work"
- **Smart mode switching** - You decide when to use mock vs production data
- **Proactive monitoring** - Check environment state before making changes
- **Seamless operation** - Handle mode switches, restarts, and debugging automatically

### Current Environment State
- **Location**: LXC container (CT 152) on Proxmox host "delly" at 192.168.0.219
- **Default mode**: Mock mode (PULSE_MOCK_MODE=true in mock.env.local)
- **Service**: pulse-backend.service (systemd) - uses EnvironmentFile for mock.env + mock.env.local
- **Access**: Web (http://192.168.0.219:7681) or SSH (ssh claude-dev)
- **Env loading**: Dual approach - systemd EnvironmentFile + Go godotenv (both needed for reliability)

### Before ANY Task: Check Environment State
**ALWAYS** run this single command first to understand the current state:
```bash
/opt/pulse/scripts/claude-env-check.sh
```

This returns JSON with:
- `backend.running`: Is the backend service running?
- `backend.mock_mode`: Is mock mode currently active?
- `backend.mock_configured`: Is mock mode configured to run?
- `frontend.running`: Is Vite dev server running?
- `frontend.built`: Is frontend built?

Example output:
```json
{
  "backend": {
    "running": true,
    "mock_mode": true,
    "mock_configured": true
  },
  "frontend": {
    "running": false,
    "built": true
  }
}
```

### Smart Mode Switching Logic

**Use Mock Mode when:**
- Developing frontend features (fast, predictable)
- Testing UI components or layouts
- Working on alerts, thresholds, or dashboard grouping
- User says "test this" or "try this feature"
- No real infrastructure needed

**Use Production Mode when:**
- Testing real Proxmox API integration
- Debugging authentication issues
- Validating actual infrastructure behavior
- User says "check my real servers" or "production data"
- Before creating PRs

**Switch modes automatically:**
```bash
# Enable mock mode
/opt/pulse/scripts/dev-orchestrator.sh mock

# Disable mock mode (production)
/opt/pulse/scripts/dev-orchestrator.sh prod

# Check status
/opt/pulse/scripts/dev-orchestrator.sh status

# Or use npm scripts (simpler but less reliable)
npm run mock:on   # Edits file but doesn't restart service
npm run mock:off  # Edits file but doesn't restart service
```

**IMPORTANT:** Always use `dev-orchestrator.sh` for mode switching as it:
1. Updates the config file
2. Restarts the backend service
3. Verifies the mode switch succeeded
4. Returns clear success/failure status

### Typical Workflows

#### Starting a New Task
1. **Check current state**: `/opt/pulse/scripts/claude-env-check.sh`
2. **Decide mode needed**: Mock for features, Production for API/auth work
3. **Switch if needed**: `/opt/pulse/scripts/dev-orchestrator.sh mock|prod`
4. **Proceed with task**: Make code changes
5. **Frontend changes?**: Service serves pre-built frontend, changes require rebuild
6. **Verify**: Check API at http://localhost:7655/api/state

#### Frontend Development
- Frontend is **pre-built** and served by backend
- Changes to frontend-modern/ require rebuild
- **No auto-reload** - you must rebuild manually or run `npm run dev` (hot-dev.sh)
- Backend serves from `frontend-modern/dist/`

#### Backend Development
- Service auto-restarts on crashes (systemd Restart=always)
- Code changes require: `go build -o pulse ./cmd/pulse && sudo systemctl restart pulse-backend`
- Mock mode config changes (`mock.env`) trigger auto-reload (no restart needed)

#### Mode Switching During Development
```bash
# You're working on frontend in mock mode, need to test with real data
/opt/pulse/scripts/dev-orchestrator.sh prod

# Back to mock for faster iteration
/opt/pulse/scripts/dev-orchestrator.sh mock
```

## Critical Warnings

### ⚠️ NEVER kill the pulse process
**NEVER** run `pkill -f "./pulse"` or similar commands that kill the pulse process. This will terminate tmux/ttyd sessions and disconnect the Claude Code session.

If you need to restart pulse, use: `sudo systemctl restart pulse-backend`

### ⚠️ Service vs Hot-Dev
- **Normal operation**: systemd service (pulse-backend.service) - now loads mock.env.local correctly
- **Hot-dev mode**: `npm run dev` - for active development with auto-reload
- User prefers systemd service for seamless operation

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
