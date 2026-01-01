# Pulse Dev Container Setup

This dev container provides a complete, reproducible development environment for Pulse with hot-reload, debugging, and testing capabilities.

## What's Included

### Development Tools
- **Go 1.24** - Backend development
- **Node.js 20** - Frontend development  
- **gopls v0.17.0** - Go language server
- **Delve** - Go debugger
- **inotify-tools** - File watching for hot reload
- **lsof** - Port management
- **Gemini CLI** - AI-assisted coding

### Features
- ✅ Hot reload for both frontend (Vite) and backend (Go)
- ✅ VS Code debugging with breakpoints
- ✅ Test Explorer integration
- ✅ Pre-commit hooks (formatting, linting)
- ✅ Persistent build caches (fast rebuilds)
- ✅ Mock data mode for safe development

## Quick Start

### First Time Setup

1. **Open in VS Code**:
   ```bash
   # On your Mac, connect to dev-containers VM via Remote-SSH
   # File → Open Folder → /root/pulse
   ```

2. **Reopen in Container**:
   - VS Code will prompt: "Reopen in Container"
   - Or: Cmd+Shift+P → "Dev Containers: Reopen in Container"

3. **Wait for build** (first time takes ~5 minutes):
   - Downloads base image
   - Installs Node.js
   - Installs Go tools
   - Runs npm install

### Daily Development

**Start the dev server:**
```bash
pd  # Alias for ./scripts/hot-dev.sh
```

Or use VS Code task: `Cmd+Shift+P` → "Tasks: Run Task" → "Start Pulse Dev Server"

**Access the app:**
- Frontend: http://localhost:7655
- Backend API: http://localhost:7656
- Metrics: http://localhost:9091

## Development Workflows

### Hot Reload

**Frontend (instant):**
- Edit any file in `frontend-modern/src/`
- Save → Browser updates automatically

**Backend (3-5 seconds):**
- Edit any `.go` file
- Save → Terminal shows:
  ```
  🔄 Change detected: yourfile.go
  Rebuilding backend...
  ✓ Build successful, restarting backend...
  ```

### Debugging

**Debug backend:**
1. Set breakpoints in Go files (click left of line number)
2. Press `F5` or click "Run and Debug" → "Debug Pulse Backend"
3. App starts in debug mode
4. Execution pauses at breakpoints

**Debug tests:**
1. Open a test file
2. Press `F5` → "Debug Current Go Test"
3. Or use Test Explorer sidebar (beaker icon)

### Testing

**Run all tests:**
```bash
ptest  # Alias for go test ./...
```

**Run specific test:**
- Use Test Explorer (beaker icon in sidebar)
- Click play button next to test name
- See results inline with code coverage

**Test with coverage:**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Mock vs Real Data

**Mock mode (default):**
```bash
mock-on   # Enable mock data
mock-edit # Configure number of nodes/VMs
pd        # Restart dev server
```

Mock mode creates fake Proxmox nodes/VMs/containers for safe testing without touching real infrastructure.

**Real infrastructure mode:**
```bash
mock-off  # Connect to real Proxmox/PBS
pd        # Restart dev server
```

⚠️ Use carefully - connects to production minipc, debian-go, pulse-relay

### Git Workflow

**Pre-commit hooks automatically run:**
- Go code formatting (`gofmt`)
- Go linting (`golangci-lint`)
- Frontend linting (ESLint)

**Make a commit:**
```bash
git add .
git commit -m "Your message"  # Hooks run automatically
git push
```

If hooks fail, fix the issues and commit again.

## Useful Aliases

| Alias | Command | Description |
|-------|---------|-------------|
| `pd` | `./scripts/hot-dev.sh` | Start dev server |
| `ptest` | `go test ./...` | Run all tests |
| `plint` | `golangci-lint run` | Run linter |
| `pfmt` | `gofmt -w -s .` | Format Go code |
| `plog` | `tail -f /tmp/pulse-dev.log` | View logs |
| `mock-on` | Toggle mock mode | Enable mock data |
| `mock-off` | Toggle mock mode | Use real infrastructure |
| `ll` | `ls -lah` | List files |
| `gs` | `git status` | Git status |

## Persistence & Rebuilds

### What Persists
- ✅ Your code (on VM disk at `/root/pulse`)
- ✅ Git commits and branches
- ✅ Go build cache (Docker volume)
- ✅ npm cache (Docker volume)
- ✅ VS Code extensions

### What Gets Reset on Rebuild
- ❌ Running processes (dev server stops)
- ❌ Terminal history
- ❌ Uncommitted environment changes

### When to Rebuild

**Rebuild needed when:**
- You change `Dockerfile` or `devcontainer.json`
- You want to update base image
- Container is corrupted

**How to rebuild:**
`Cmd+Shift+P` → "Dev Containers: Rebuild Container"

**Rebuilds are fast** (~30 seconds) thanks to:
- Persistent Go build cache
- Persistent npm cache  
- Docker layer caching

## Troubleshooting

### Port already in use
```bash
# Kill processes on ports 7655, 7656
lsof -ti:7655,7656 | xargs kill -9
pd  # Restart
```

### Out of disk space
```bash
# On dev-containers VM (via SSH)
docker system prune -af --volumes
```

### gopls not working
```bash
# Reinstall Go tools
go install golang.org/x/tools/gopls@v0.17.0
```

### Hot reload not working
Check terminal for file watcher errors. Restart dev server with `Ctrl+C` then `pd`.

### Container won't start
1. Check Docker is running: `docker ps`
2. Check VM has resources: `df -h` and `free -h`
3. Rebuild: `Cmd+Shift+P` → "Rebuild Container"

## Environment Variables

Set in `devcontainer.json` → `containerEnv`:

| Variable | Value | Purpose |
|----------|-------|---------|
| `PULSE_DEV_API_HOST` | `localhost` | Backend API host |
| `FRONTEND_DEV_HOST` | `0.0.0.0` | Frontend bind address |
| `LAN_IP` | `localhost` | LAN IP for URLs |

Custom overrides: Create `.env.devcontainer` (gitignored)

## Resources

- **VM Specs**: 8GB RAM, 30GB disk, 2 CPU cores
- **Base Image**: `golang:1.24` (Ubuntu-based)
- **Caches**: ~2-3GB for Go modules and build artifacts

## Tips & Tricks

1. **Split terminals**: Click + in terminal to run dev server in one, commands in another
2. **Quick test**: Click ▶️ next to test function name in editor
3. **Go to definition**: Cmd+click on function/type
4. **Find references**: Right-click → "Find All References"
5. **Refactor**: Right-click → "Rename Symbol" (renames everywhere)
6. **Format on save**: Already enabled for Go and frontend files
7. **Problems panel**: See all errors/warnings in one place
8. **Git sidebar**: View changes, stage, commit without terminal

## Architecture

```
MacBook (VS Code)
    ↓ Remote-SSH
dev-containers VM (Proxmox)
    ↓ Docker
Dev Container
    ├── Go 1.24 + tools
    ├── Node 20 + npm
    ├── Your code (/workspaces/pulse)
    ├── Hot reload watchers
    └── Running dev server
```

Code lives on VM disk, container mounts it. VS Code connects remotely and forwards ports to your Mac.

## Next Steps

- Read `CONTRIBUTING.md` for contribution guidelines
- Check `ARCHITECTURE.md` to understand the codebase
- Run `ptest` to ensure all tests pass
- Start coding! Changes hot-reload automatically 🚀
