# Claude Development Notes

## 🚨 CRITICAL: Git Commit Policy
**NEVER COMMIT WITHOUT EXPLICIT USER REQUEST** - Only when user explicitly asks

## Git Workflow - Ultra-Simple Release
**Principle**: Linear auto-merge flow
1. Work on `develop` (push freely, no releases)
2. PR to `main` → RC + auto-merge  
3. Manual stable release trigger

**Commands**:
```bash
# Check commits: git log --oneline main..develop
# Create RC: gh pr create --base main --head develop --title "Release: X"
# Stable: gh workflow run stable-release.yml --ref main
```

**Branch Strategy**: Stay on `develop` always. Only switch for PR conflicts/critical main fixes. `pull.rebase = true` set.

**RC PR Format**: Group commits by ✨Features/🐛Fixes/🔧Improvements with attribution `(abc1234, addressing #123)`

**Release Types**: All created as drafts, manual publish required. RC=prerelease, Stable=production.

## Development Behavior
- Do exactly what's asked, no more
- Edit existing files over creating new
- No proactive docs creation  
- Use TodoWrite for complex tasks
- Test fixes but DON'T commit unless asked

## Service Management
**systemd service** with hot reloading enabled. File changes auto-restart.
- Status: `systemctl status pulse`
- Logs: `journalctl -u pulse -f`
- Never use `npm start` - service handles it

## Commit Consolidation
When multiple related commits, consolidate before push:
```bash
git reset --soft HEAD~N  # Uncommit N commits (keeps changes)
git commit -m "consolidated message addressing #123"
```

## Technical Notes

### Proxmox API I/O Limitation
`/cluster/resources?type=vm` I/O counters update ~10s (not real-time). **Solution**: Hybrid approach - bulk for CPU/memory/disk, individual calls for I/O rates.

### API Architecture
**REST**: `/api/alerts|config|health|version|charts|thresholds` - NO `/api/guests|data` endpoints exist
**WebSocket**: `rawData` (guest metrics), `alert*` (notifications), `update*` (system events)
**Guest data**: WebSocket `rawData` only, not REST - prevents polling, ensures real-time updates

### Update System
- Channel persistence (stable/RC)
- Auto-refresh 8s after restart
- Files: `settings.js`, `main.js`, `api.js`, `install-pulse.sh`

### Commit/Issue Preferences
- Reference issues: `addressing #123` (not `fixes/closes`)
- Short hash in comments: `abc1234` (GitHub auto-links)
- Group related changes, avoid commit spam
- Run tests before committing

## Workflow Summary
1. 🔨 Develop on `develop` (no releases)
2. 🧪 PR → RC + auto-merge  
3. 🚀 Manual stable trigger