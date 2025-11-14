# Migration & Rollout Scaffolding

This document tracks temporary code paths, feature flags, and kill switches that
let us roll out risky changes safely. Keep it current so on-call engineers know
how to disable new behavior without spelunking through the codebase.

## How to Use This Document

1. **Add an entry** whenever you introduce code that needs a manual cleanup
   window (feature flags, compatibility layers, installers with rollback paths).
2. **State the kill switch** (environment variable, UI toggle, config file, or
   systemd override) and who owns the rollout.
3. **Define removal criteria**. Once criteria are met, delete the scaffolding
   and update this file.

### Entry Template

```
## <Feature / Migration Name>
- Introduced: vX.Y.Z (date/commit, if known)
- Owner: <team/person>
- Kill switch: how to disable + default state
- Monitoring: metrics/logs to watch
- Cleanup criteria: what must be true before removing scaffolding
- Notes: cross-links to docs or runbooks
```

## Active Scaffolding

### Adaptive Polling Scheduler
- **Introduced**: v4.24.0 (documented in `docs/monitoring/ADAPTIVE_POLLING.md`)
- **Owner**: Monitoring subsystem (rcourtman)
- **Kill switch**: Toggle **Settings → System → Monitoring → Adaptive
  Polling**, or set `ADAPTIVE_POLLING_ENABLED=false` before starting Pulse
  (Docker/Kubernetes). Helm users can override `adaptivePollingEnabled: false`.
- **Monitoring**: `pulse_monitor_poll_queue_depth`,
  `pulse_monitor_poll_staleness_seconds`, and
  `/api/monitoring/scheduler/health` for breaker/dead-letter status.
- **Cleanup criteria**: Remove flag and legacy scheduler code after three stable
  releases with `queue.depth < 50` during peak hours and no dead-letter growth.
- **Notes**: Operational guidance lives in
  `docs/operations/ADAPTIVE_POLLING_ROLLOUT.md`.

### Automatic Updates Service
- **Introduced**: commit `f46ff1792` (2025-10-11) added `scripts/pulse-auto-update.sh`
- **Owner**: Installer/Update subsystem (rcourtman)
- **Kill switch**: Disable **Settings → System → Updates → Automatic Updates** or
  set `AUTO_UPDATE_ENABLED=false` in `/var/lib/pulse/system.json`. Installers
  accept `--no-auto-update` to force the flag off.
- **Monitoring**: `journalctl -u pulse-update.service`, UI update history
  (Settings → System → Updates), and webhook notifications tagged with
  `event_id=system.update`.
- **Cleanup criteria**: Remove legacy opt-out plumbing once the opt-in rate is
  ≥90 % across supported installers and we have at least two release cycles with
  zero rollback events triggered by auto-update.
- **Notes**: Keep release-specific caveats in `docs/RELEASE_NOTES.md`. The
  update scheduler runs only on systemd installs; container builds ignore the
  flag.
