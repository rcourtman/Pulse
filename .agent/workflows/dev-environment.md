---
description: How the development environment works and how to manage it
---

# Pulse Development Environment

This is a **development-only machine**. Production is never run here.

## Primary Service: `pulse-hot-dev`

The dev environment uses a **single systemd service** that runs both backend and frontend with hot-reloading:

```bash
# Status check
systemctl status pulse-hot-dev

# Restart (if needed)
// turbo
sudo systemctl restart pulse-hot-dev

# View logs
journalctl -u pulse-hot-dev -f
```

### What `pulse-hot-dev` does:
1. **Go Backend** (`./pulse`): Monitored by `inotifywait` - auto-rebuilds and restarts when `.go` files change
2. **Vite Frontend**: HMR enabled - browser updates instantly when frontend files change

### Access URLs:
- **Frontend**: http://192.168.0.123:5173/ (Vite dev server with HMR)
- **Backend API**: http://192.168.0.123:7655/ (Go server, proxied through Vite)

## DO NOT USE these services in development:
- `pulse.service` - This is for production (runs pre-built binary without hot-reload)
- Do NOT create separate `pulse-frontend.service` - the hot-dev script handles everything

## When things don't work:

1. **Frontend not loading at :5173** → Check if `pulse-hot-dev` is running
2. **Backend changes not reflected** → Check logs: `journalctl -u pulse-hot-dev -f`
3. **Need full restart** → `sudo systemctl restart pulse-hot-dev`

## Key Files:
- **Hot-dev script**: `/opt/pulse/scripts/hot-dev.sh`
- **Systemd service**: `/etc/systemd/system/pulse-hot-dev.service`
- **Makefile targets**: `make dev` or `make dev-hot`
