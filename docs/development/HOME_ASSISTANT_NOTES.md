# Home Assistant Battery Automation Notes (2025-10-10)

- Work performed on delly host, LXC VMID 101 (Home Assistant). Pulse codebase unaffected.
- Root cause: malformed `automations.yaml` caused five restarts between 00:23–00:33 BST.
- Fixes applied:
  - Restored automation backups.
  - Replaced `pyscript.solis_set_charge_current` calls with `number.set_value` targeting `number.solis_rhi_time_charging_charge_current`.
  - Removed invalid `source:` attributes from `number.set_value` actions.
- Validation:
  - Triggered key automations (`automation.free_electricity_session_maximum_charging`, `automation.intelligent_dispatch_solis_battery_charging`, `automation.emergency_low_soc_protection`, EV guard hold/refresh/release).
  - Observed expected charge-current adjustments (100 A car slots/free session, 25 A emergency, 5 A guard) and matching system logs.
- Defaults restored: overnight slot `23:30–05:30`.
- Backups: `/var/lib/docker/volumes/hass_config/_data/automations.yaml.codex_source_cleanup_20251010_090205` (do not delete).
- Testing helpers: API token stored at `/tmp/ha_token.txt` inside VM; use `pct exec 101 -- bash -lc '...'` for commands.
- Final state: charge current reset to 25 A; EV guard sensor `off`.
